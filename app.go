package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	stdsync "sync"
	"time"
	goruntime "runtime"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/ys-ll/uniterm/backend/log"
	"github.com/ys-ll/uniterm/backend/database"
	"github.com/ys-ll/uniterm/backend/session"
	"github.com/ys-ll/uniterm/backend/store"
	"github.com/ys-ll/uniterm/backend/sync"
	"github.com/ys-ll/uniterm/backend/update"
)

type App struct {
	ctx                  context.Context
	sessionManager       *session.SessionManager
	connectionStore      *store.ConnectionStore
	aiSessionStore       *store.AISessionStore
	settingsStore        *store.SettingsStore
	terminalHistoryStore *store.TerminalHistoryStore
	syncService          *sync.SyncService
	tunnelService        *session.TunnelService
	mainHwnd            uintptr
	originalWndProc     uintptr
	wndProcCb           uintptr // keep alive to prevent GC
	inSizeMove          bool
	webviewDataPath     string
	chatCancel          context.CancelFunc // active stream cancellation
	chatCancelMu        stdsync.Mutex       // guards chatCancel
}

func NewApp(webviewDataPath string) *App {
	return &App{webviewDataPath: webviewDataPath}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Init logger first so subsequent log.Writef calls actually write
	if err := log.Init(); err != nil {
		fmt.Printf("WARN: log.Init failed: %v\n", err)
	}

	a.sessionManager = session.NewSessionManager()
	a.tunnelService = session.NewTunnelService()

	// Discover main window HWND for RDP child window embedding
	a.mainHwnd = a.findMainWindow()
	a.subclassMainWindow()

	cs, err := store.NewConnectionStore()
	if err != nil {
		log.Writef("Failed to init connection store: %v", err)
		return
	}
	a.connectionStore = cs

	ass, err := store.NewAISessionStore()
	if err != nil {
		log.Writef("Failed to init AI session store: %v", err)
		return
	}
	a.aiSessionStore = ass

	ss, err := store.NewSettingsStore()
	if err != nil {
		log.Writef("Failed to init settings store: %v", err)
		return
	}
	a.settingsStore = ss

	// Init terminal history store (same config dir as other stores)
	configDir, _ := os.UserConfigDir()
	appDir := filepath.Join(configDir, "uniTerm")
	a.terminalHistoryStore = store.NewTerminalHistoryStore(appDir)

	syncSvc, err := sync.NewSyncService()
	if err != nil {
		log.Writef("Failed to create sync service: %v", err)
	} else {
		a.syncService = syncSvc
		// Wire keychain into stores for password/API key migration
		if a.connectionStore != nil {
			a.connectionStore.SetPasswordStore(syncSvc.PasswordStore())
		}
		if a.settingsStore != nil {
			a.settingsStore.SetPasswordStore(syncSvc.PasswordStore())
		}
		// Auto-sync on startup if enabled
		if syncSvc.IsAutoSyncEnabled() {
			go func() {
				result, err := syncSvc.Sync()
				if err != nil {
					log.Writef("Auto-sync on startup failed: %v", err)
				} else if result.Direction == sync.SyncConflict {
					runtime.EventsEmit(a.ctx, "sync:conflict", map[string]interface{}{
						"localTime":  result.Conflict.LocalTime.Format(time.RFC3339),
						"remoteTime": result.Conflict.RemoteTime.Format(time.RFC3339),
					})
				}
			}()
		}
	}
}

func (a *App) shutdown(ctx context.Context) {
	a.unsubclassMainWindow()
	if a.tunnelService != nil {
		a.tunnelService.Shutdown()
	}
	if a.sessionManager != nil {
		a.sessionManager.CloseAll()
	}
	os.RemoveAll(a.webviewDataPath)
}

// ConnectionStore methods

func (a *App) SaveConnections(data session.ConnectionStoreData) error {
	if a.connectionStore == nil {
		return fmt.Errorf("connection store not initialized")
	}
	err := a.connectionStore.Save(data)
	if err == nil {
		runtime.EventsEmit(a.ctx, "store:connections:changed", data)
		a.triggerAutoSync()
	}
	return err
}

func (a *App) LoadConnections() (session.ConnectionStoreData, error) {
	if a.connectionStore == nil {
		return session.ConnectionStoreData{}, fmt.Errorf("connection store not initialized")
	}
	return a.connectionStore.Load()
}

// AI Config Store methods

func (a *App) SaveAIConfig(config store.AIConfig) error {
	if a.settingsStore == nil {
		return fmt.Errorf("settings store not initialized")
	}
	settings, err := a.settingsStore.Load()
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}
	// Update the active model's fields
	for i := range settings.AI.Models {
		if settings.AI.Models[i].ID == settings.AI.ActiveModelID {
			settings.AI.Models[i].APIKey = config.APIKey
			settings.AI.Models[i].BaseURL = config.BaseURL
			settings.AI.Models[i].Model = config.Model
			break
		}
	}
	if err := a.settingsStore.Save(settings); err != nil {
		return err
	}
	a.triggerAutoSync()
	return nil
}

// reloadStoresAfterSync reloads connections and settings from disk and emits
// events so the frontend refreshes after a sync pull.
func (a *App) reloadStoresAfterSync() {
	if a.connectionStore != nil {
		if data, err := a.connectionStore.Load(); err == nil {
			runtime.EventsEmit(a.ctx, "store:connections:changed", data)
		}
	}
	if a.settingsStore != nil {
		if settings, err := a.settingsStore.Load(); err == nil {
			runtime.EventsEmit(a.ctx, "store:settings:changed", settings)
		}
	}
}

func (a *App) triggerAutoSync() {
	if a.syncService == nil || !a.syncService.IsAutoSyncEnabled() {
		return
	}
	go func() {
		result, err := a.syncService.Sync()
		if err != nil {
			log.Writef("Auto-sync failed: %v", err)
		} else if result.Direction == sync.SyncConflict {
			runtime.EventsEmit(a.ctx, "sync:conflict", map[string]interface{}{
				"localTime":  result.Conflict.LocalTime.Format(time.RFC3339),
				"remoteTime": result.Conflict.RemoteTime.Format(time.RFC3339),
			})
		}
		if err == nil && result.Direction == sync.SyncPull {
			a.reloadStoresAfterSync()
		}
		runtime.EventsEmit(a.ctx, "sync:completed")
	}()
}
func (a *App) SyncGetConfig() (sync.SyncConfig, error) {
	if a.syncService == nil {
		return sync.SyncConfig{}, fmt.Errorf("sync service not initialized")
	}
	return a.syncService.GetConfig()
}

// SyncSaveConfig saves the sync configuration.
func (a *App) SyncSaveConfig(config sync.SyncConfig, token string) error {
	if a.syncService == nil {
		return fmt.Errorf("sync service not initialized")
	}
	return a.syncService.SaveConfig(config, token)
}

// SyncNow runs an immediate sync.
func (a *App) SyncNow() (*sync.SyncResult, error) {
	if a.syncService == nil {
		return nil, fmt.Errorf("sync service not initialized")
	}
	result, err := a.syncService.Sync()
	if err != nil {
		return nil, err
	}
	if result.Direction == sync.SyncConflict {
		runtime.EventsEmit(a.ctx, "sync:conflict", map[string]interface{}{
			"localTime":  result.Conflict.LocalTime.Format(time.RFC3339),
			"remoteTime": result.Conflict.RemoteTime.Format(time.RFC3339),
		})
	}
	if result.Direction == sync.SyncPull {
		a.reloadStoresAfterSync()
	}
	runtime.EventsEmit(a.ctx, "sync:completed")
	return result, nil
}

// SyncResolveConflict resolves a sync conflict.
func (a *App) SyncResolveConflict(useLocal bool) (*sync.SyncResult, error) {
	if a.syncService == nil {
		return nil, fmt.Errorf("sync service not initialized")
	}
	result, err := a.syncService.ResolveConflict(useLocal)
	if err != nil {
		return nil, err
	}
	if result.Direction == sync.SyncPull {
		if data, err := a.connectionStore.Load(); err == nil {
			runtime.EventsEmit(a.ctx, "store:connections:changed", data)
		}
		if settings, err := a.settingsStore.Load(); err == nil {
			runtime.EventsEmit(a.ctx, "store:settings:changed", settings)
		}
	}
	return result, nil
}

// SyncTestConnection tests the repository connection.
func (a *App) SyncTestConnection() error {
	if a.syncService == nil {
		return fmt.Errorf("sync service not initialized")
	}
	return a.syncService.TestConnection()
}

// SyncConfigureRepo sets up a new or existing sync repository.
func (a *App) SyncConfigureRepo(repoURL, username, token, masterPassword string) (*sync.SyncResult, error) {
	if a.syncService == nil {
		return nil, fmt.Errorf("sync service not initialized")
	}
	result, err := a.syncService.ConfigureRepo(repoURL, username, token, masterPassword)
	if err == nil {
		a.reloadStoresAfterSync()
		runtime.EventsEmit(a.ctx, "sync:completed")
	}
	return result, err
}

// SyncChangePassword re-encrypts synced files with a new master password.
func (a *App) SyncChangePassword(oldPassword, newPassword string) error {
	if a.syncService == nil {
		return fmt.Errorf("sync service not initialized")
	}
	return a.syncService.ChangePassword(oldPassword, newPassword)
}

// SyncVerifyPassword verifies the given password can decrypt the repo config.
func (a *App) SyncVerifyPassword(password, username, token string) error {
	if a.syncService == nil {
		return fmt.Errorf("sync service not initialized")
	}
	return a.syncService.VerifySyncPassword(password, username, token)
}

// SyncDeleteRepo removes the sync repository configuration.
func (a *App) SyncDeleteRepo() error {
	if a.syncService == nil {
		return fmt.Errorf("sync service not initialized")
	}
	return a.syncService.DeleteRepo()
}

func (a *App) LoadAIConfig() (store.AIConfig, error) {
	if a.settingsStore == nil {
		return store.AIConfig{}, fmt.Errorf("settings store not initialized")
	}
	settings, err := a.settingsStore.Load()
	if err != nil {
		return store.AIConfig{}, err
	}
	// Return the active model's config
	for _, m := range settings.AI.Models {
		if m.ID == settings.AI.ActiveModelID {
			return store.AIConfig{
				APIKey:  m.APIKey,
				BaseURL: m.BaseURL,
				Model:   m.Model,
			}, nil
		}
	}
	return store.AIConfig{}, nil
}

// AI Session Store methods

func (a *App) SaveAISessions(data store.AISessionData) error {
	if a.aiSessionStore == nil {
		return fmt.Errorf("AI session store not initialized")
	}
	return a.aiSessionStore.Save(data)
}

func (a *App) LoadAISessions() (store.AISessionData, error) {
	if a.aiSessionStore == nil {
		return store.AISessionData{}, fmt.Errorf("AI session store not initialized")
	}
	return a.aiSessionStore.Load()
}

// SettingsStore methods

func (a *App) SaveSettings(settings store.AppSettings) error {
	if a.settingsStore == nil {
		return fmt.Errorf("settings store not initialized")
	}
	err := a.settingsStore.Save(settings)
	if err == nil {
		a.triggerAutoSync()
	}
	return err
}

func (a *App) LoadSettings() (store.AppSettings, error) {
	if a.settingsStore == nil {
		return store.AppSettings{}, fmt.Errorf("settings store not initialized")
	}
	return a.settingsStore.Load()
}

func (a *App) OpenFileDialog() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select File",
	})
}

func (a *App) OpenMultipleFilesDialog() ([]string, error) {
	files, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Files",
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func (a *App) OpenDirectoryDialog() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Directory",
	})
}

func (a *App) SaveFileDialog(defaultName string) (string, error) {
	return runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save File",
		DefaultFilename: defaultName,
	})
}

func (a *App) GetDesktopPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, "Desktop"), nil
}

func (a *App) GetPlatform() string {
	return goruntime.GOOS
}

func (a *App) OnConnectionsChanged(callback func(session.ConnectionStoreData)) {
	runtime.EventsOn(a.ctx, "store:connections:changed", func(optionalData ...interface{}) {
		if len(optionalData) > 0 {
			if data, ok := optionalData[0].(session.ConnectionStoreData); ok {
				callback(data)
			}
		}
	})
}

// SessionManager methods

func (a *App) CreateSession(sessionType string, config session.ConnectionConfig) (*session.SessionInfo, error) {
	if a.sessionManager == nil {
		return nil, fmt.Errorf("session manager not initialized")
	}
	log.Writef("[CreateSession] type=%s, dbType=%s, host=%s, port=%d, user=%s, dbName=%s, name=%s",
		sessionType, config.DBType, config.Host, config.Port, config.User, config.DBName, config.Name)
	s, err := a.sessionManager.Create(sessionType, config)
	if err != nil {
		log.Writef("[CreateSession] manager.Create failed: %v", err)
		return nil, err
	}
	log.Writef("[CreateSession] session created, id=%s", s.ID())

	// ── SSH Tunnel ──────────────────────────────────────────────
	if config.TunnelSSHConnID != "" && a.tunnelService != nil {
		if a.connectionStore == nil {
			_ = a.sessionManager.Close(s.ID())
			return nil, fmt.Errorf("connection store not initialized")
		}
		data, err := a.connectionStore.Load()
		if err != nil {
			_ = a.sessionManager.Close(s.ID())
			return nil, fmt.Errorf("load connections for tunnel: %w", err)
		}
		var tunnelSSHConfig *session.ConnectionConfig
		for _, c := range data.Connections {
			if c.ID == config.TunnelSSHConnID {
				tunnelSSHConfig = &c
				break
			}
		}
		if tunnelSSHConfig == nil {
			_ = a.sessionManager.Close(s.ID())
			return nil, fmt.Errorf("tunnel SSH connection not found: %s", config.TunnelSSHConnID)
		}

		// Resolve actual target port. VNC/SPICE use libvirt display
			// numbers (port < 100 means display :N → port 5900+N).
			targetPort := config.Port
			if sessionType == "vnc" || sessionType == "spice" {
				if targetPort <= 0 {
					targetPort = 5900
				} else if targetPort < 100 {
					targetPort += 5900
				}
			}
			localPort, err := a.tunnelService.Start(s.ID(), *tunnelSSHConfig, config.Host, targetPort)
		if err != nil {
			_ = a.sessionManager.Close(s.ID())
			return nil, fmt.Errorf("tunnel start: %w", err)
		}
		log.Writef("[CreateSession] tunnel established for session=%s via ssh=%s, localPort=%d",
			s.ID(), config.TunnelSSHConnID, localPort)
		config.Host = "127.0.0.1"
		config.Port = localPort
	}
	// ── End SSH Tunnel ──────────────────────────────────────────

	// SFTP concurrency limit
	if sessionType == "sftp" {
		if sftp, ok := s.(*session.SFTPSession); ok {
			n := config.SftpMaxConcurrency
			if n <= 0 {
				n = 5
			}
			sftp.SetMaxConcurrency(n)
		}
	}

	// Set parent HWND for RDP sessions
	if rdp, ok := s.(*session.RDPSession); ok {
		rdp.SetParentHwnd(a.mainHwnd)
	}

	s.SetOnDataCallback(func(data []byte) {
		runtime.EventsEmit(a.ctx, "session:data", map[string]interface{}{
			"id":   s.ID(),
			"data": string(data),
		})
	})

	s.SetOnBinaryCallback(func(data []byte) {
		runtime.EventsEmit(a.ctx, "session:binary", map[string]interface{}{
			"id":   s.ID(),
			"data": base64.StdEncoding.EncodeToString(data),
		})
	})

	s.SetOnStatusChangeCallback(func(status session.SessionStatus) {
		payload := map[string]interface{}{
			"id":     s.ID(),
			"status": status,
		}
		// For RDP sessions, include client area screen coordinates so the
		// frontend can position the overlay window without fragile browser APIs.
		if status == session.StatusConnected {
			if rdp, ok := s.(*session.RDPSession); ok {
				cx, cy, cw, ch := rdp.ClientAreaScreenRect()
				payload["clientX"] = cx
				payload["clientY"] = cy
				payload["clientW"] = cw
				payload["clientH"] = ch
			}
			// Attach proxyAddr for VNC and SPICE sessions
			if vnc, ok := s.(*session.VNCSession); ok {
				payload["proxyAddr"] = vnc.ProxyAddr()
			}
			if spice, ok := s.(*session.SPICESession); ok {
				payload["proxyAddr"] = spice.ProxyAddr()
			}
		}
		runtime.EventsEmit(a.ctx, "session:status", payload)
	})

	// Database sessions connect synchronously so errors are returned to the frontend.
	if sessionType == "database" {
		log.Writef("[CreateSession] connecting database session synchronously...")
		if err := s.Connect(config); err != nil {
			log.Writef("[CreateSession] database connect failed: %v", err)
			_ = a.sessionManager.Close(s.ID())
			return nil, fmt.Errorf("database connect failed: %w", err)
		}
		log.Writef("[CreateSession] database session connected successfully, id=%s", s.ID())
	} else {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Writef("session %s connect panic: %v\n%s", s.ID(), r, string(debug.Stack()))
				}
			}()
			if err := s.Connect(config); err != nil {
				if a.ctx != nil {
					runtime.EventsEmit(a.ctx, "session:data", map[string]interface{}{
						"id":   s.ID(),
						"data": fmt.Sprintf("\r\n\x1b[31m[Connection failed: %v]\x1b[0m\r\nPress Enter to retry...\r\n", err),
					})
				}
				log.Writef("session %s connect error: %v", s.ID(), err)
				// Remove failed session from manager to avoid leaking stale entries
				if a.sessionManager != nil {
					_ = a.sessionManager.Close(s.ID())
				}
			}
		}()
	}

	info := &session.SessionInfo{
		ID:     s.ID(),
		Type:   s.Type(),
		Title:  s.Title(),
		Status: s.Status(),
	}
	return info, nil
}

func (a *App) CloseSession(sessionID string) error {
	if a.sessionManager == nil {
		return fmt.Errorf("session manager not initialized")
	}
	if a.tunnelService != nil {
		a.tunnelService.Stop(sessionID)
	}
	return a.sessionManager.Close(sessionID)
}

func (a *App) ListSessions() []session.SessionInfo {
	if a.sessionManager == nil {
		return []session.SessionInfo{}
	}
	return a.sessionManager.List()
}

func (a *App) SessionWrite(sessionID string, data string) error {
	if a.sessionManager == nil {
		return fmt.Errorf("session manager not initialized")
	}
	s, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	return s.Write([]byte(data))
}

func (a *App) SessionResize(sessionID string, cols, rows int) error {
	if a.sessionManager == nil {
		return fmt.Errorf("session manager not initialized")
	}
	s, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	return s.Resize(cols, rows)
}

func (a *App) SessionStartZmodem(sessionID string) error {
	if a.sessionManager == nil {
		return fmt.Errorf("session manager not initialized")
	}
	s, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	s.SetZmodemMode(true)
	return nil
}

func (a *App) SessionEndZmodem(sessionID string) error {
	if a.sessionManager == nil {
		return fmt.Errorf("session manager not initialized")
	}
	s, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	s.SetZmodemMode(false)
	return nil
}

func (a *App) SessionWriteBinary(sessionID string, base64Data string) error {
	if a.sessionManager == nil {
		return fmt.Errorf("session manager not initialized")
	}
	s, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return fmt.Errorf("decode base64: %w", err)
	}
	return s.Write(data)
}

func (a *App) ReadFileBase64(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

func (a *App) WriteFileBase64(path string, base64Data string) error {
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return fmt.Errorf("decode base64: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func (a *App) RDPSetPosition(sessionID string, x, y, w, h int) error {
	if a.sessionManager == nil {
		return fmt.Errorf("session manager not initialized")
	}
	s, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	rdp, ok := s.(*session.RDPSession)
	if !ok {
		return fmt.Errorf("session is not RDP")
	}
	rdp.SetPosition(x, y, w, h)
	return nil
}

func (a *App) RDPShow(sessionID string) error {
	if a.sessionManager == nil {
		return fmt.Errorf("session manager not initialized")
	}
	s, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	rdp, ok := s.(*session.RDPSession)
	if !ok {
		return fmt.Errorf("session is not RDP")
	}
	rdp.Show()
	return nil
}

func (a *App) RDPSetFocus(sessionID string, focused bool) error {
	if a.sessionManager == nil {
		return fmt.Errorf("session manager not initialized")
	}
	s, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	rdp, ok := s.(*session.RDPSession)
	if !ok {
		return fmt.Errorf("session is not RDP")
	}
	rdp.SetFocus(focused)
	return nil
}

func (a *App) RDPHide(sessionID string) error {
	if a.sessionManager == nil {
		return fmt.Errorf("session manager not initialized")
	}
	s, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	rdp, ok := s.(*session.RDPSession)
	if !ok {
		return fmt.Errorf("session is not RDP")
	}
	rdp.Hide()
	return nil
}

// MonitorSession methods

func (a *App) getMonitorSession(sessionID string) (*session.MonitorSession, error) {
	if a.sessionManager == nil {
		return nil, fmt.Errorf("session manager not initialized")
	}
	s, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	ms, ok := s.(*session.MonitorSession)
	if !ok {
		return nil, fmt.Errorf("session is not a monitor session: %s", sessionID)
	}
	return ms, nil
}

func (a *App) SetMonitorActiveTab(sessionID string, tab string) error {
	ms, err := a.getMonitorSession(sessionID)
	if err != nil {
		return err
	}
	ms.SetActiveTab(tab)
	return nil
}

func (a *App) SetMonitorPaused(sessionID string, paused bool) error {
	ms, err := a.getMonitorSession(sessionID)
	if err != nil {
		return err
	}
	ms.SetPaused(paused)
	return nil
}

func (a *App) GetProcessDetail(sessionID string, pid int) (map[string]interface{}, error) {
	ms, err := a.getMonitorSession(sessionID)
	if err != nil {
		return nil, err
	}
	return ms.GetProcessDetail(pid)
}

func (a *App) KillProcess(sessionID string, pid int, signal string) error {
	ms, err := a.getMonitorSession(sessionID)
	if err != nil {
		return err
	}
	return ms.KillProcess(pid, signal)
}

func (a *App) GetPorts(sessionID string) ([]session.PortInfo, error) {
	ms, err := a.getMonitorSession(sessionID)
	if err != nil {
		return nil, err
	}
	return ms.GetPorts()
}

func (a *App) GetDisks(sessionID string) ([]session.DiskInfo, error) {
	ms, err := a.getMonitorSession(sessionID)
	if err != nil {
		return nil, err
	}
	return ms.GetDisks()
}

func (a *App) GetNetworkCards(sessionID string) ([]session.NetCardInfo, error) {
	ms, err := a.getMonitorSession(sessionID)
	if err != nil {
		return nil, err
	}
	return ms.GetNetworkCards()
}

type AppInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func (a *App) GetAppInfo() AppInfo {
	return AppInfo{
		Name:    "uniTerm",
		Version: Version,
	}
}

func (a *App) CheckForUpdate() (*update.UpdateInfo, error) {
	return update.Check(Version)
}

func (a *App) SaveTerminalHistory(entries []store.HistoryEntry) error {
	if a.terminalHistoryStore == nil {
		return fmt.Errorf("terminal history store not initialized")
	}
	return a.terminalHistoryStore.Save(entries)
}

func (a *App) LoadTerminalHistory() ([]store.HistoryEntry, error) {
	if a.terminalHistoryStore == nil {
		return []store.HistoryEntry{}, fmt.Errorf("terminal history store not initialized")
	}
	return a.terminalHistoryStore.Load()
}

func (a *App) DeleteTerminalHistoryEntry(ids []string) error {
	if a.terminalHistoryStore == nil {
		return fmt.Errorf("terminal history store not initialized")
	}
	return a.terminalHistoryStore.DeleteByIDs(ids)
}

// ChatCompletion streams the Anthropic API response via SSE, emitting Wails
// events for each token while collecting the full message. It returns the
// complete message JSON when the stream ends (backward-compatible).
func (a *App) ChatCompletion(apiKey, baseURL, model string, requestJSON string, protocol string) (string, error) {
	// Inject stream: true into the request body
	var reqBody map[string]interface{}
	if err := json.Unmarshal([]byte(requestJSON), &reqBody); err != nil {
		return "", fmt.Errorf("invalid request JSON: %w", err)
	}
	reqBody["stream"] = true

	modifiedJSON, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal modified request: %w", err)
	}

	url := strings.TrimRight(baseURL, "/") + "/messages"

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Store cancel for frontend stop
	a.chatCancelMu.Lock()
	a.chatCancel = cancel
	a.chatCancelMu.Unlock()
	defer func() {
		a.chatCancelMu.Lock()
		a.chatCancel = nil
		a.chatCancelMu.Unlock()
	}()

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(modifiedJSON))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-beta", "prompt-caching-2024-07-31")
	req.Header.Set("User-Agent", "uniTerm")

	client := &http.Client{Timeout: 0} // no timeout; context handles it
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return "", fmt.Errorf("HTTP %d: %s", res.StatusCode, string(body))
	}

	// Accumulated state from SSE events
	var contentBlocks []map[string]interface{}
	var currentBlock map[string]interface{}
	var messageRole string
	var usage map[string]interface{}
	currentBlockIndex := -1

	scanner := bufio.NewScanner(res.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB max line

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		dataStr := line[6:]

		var event map[string]interface{}
		if err := json.Unmarshal([]byte(dataStr), &event); err != nil {
			continue
		}

		eventType, _ := event["type"].(string)

		switch eventType {
		case "message_start":
			if msg, ok := event["message"].(map[string]interface{}); ok {
				messageRole, _ = msg["role"].(string)
			}

		case "content_block_start":
			currentBlockIndex++
			if block, ok := event["content_block"].(map[string]interface{}); ok {
				currentBlock = block
				runtime.EventsEmit(a.ctx, "ai:block_start", map[string]interface{}{
					"index":         currentBlockIndex,
					"content_block": block,
				})
			}

		case "content_block_delta":
			delta, _ := event["delta"].(map[string]interface{})
			deltaType, _ := delta["type"].(string)

			if deltaType == "text_delta" {
				text, _ := delta["text"].(string)
				// Accumulate text into the current block for the final response
				if currentBlock != nil {
					if currentBlock["text"] == nil {
						currentBlock["text"] = ""
					}
					currentBlock["text"] = currentBlock["text"].(string) + text
				}
				runtime.EventsEmit(a.ctx, "ai:token", map[string]interface{}{
					"text":  text,
					"index": currentBlockIndex,
				})
			}
			if deltaType == "input_json_delta" && currentBlock != nil {
				partial, _ := delta["partial_json"].(string)
				// content_block_start sets input to {} for tool_use; reset to ""
				if currentBlock["input"] == nil || fmt.Sprintf("%T", currentBlock["input"]) != "string" {
					currentBlock["input"] = ""
				}
				if s, ok := currentBlock["input"].(string); ok {
					currentBlock["input"] = s + partial
				}
			}

		case "content_block_stop":
			if currentBlock != nil {
				if blockType, _ := currentBlock["type"].(string); blockType == "tool_use" {
					if inputStr, ok := currentBlock["input"].(string); ok && inputStr != "" {
						var inputObj map[string]interface{}
						if err := json.Unmarshal([]byte(inputStr), &inputObj); err == nil {
							currentBlock["input"] = inputObj
						}
					}
				}
				contentBlocks = append(contentBlocks, currentBlock)
				currentBlock = nil
			}

		case "message_delta":
			if u, ok := event["usage"].(map[string]interface{}); ok {
				usage = u
			}
			if delta, ok := event["delta"].(map[string]interface{}); ok {
				if stopReason, ok := delta["stop_reason"].(string); ok {
					runtime.EventsEmit(a.ctx, "ai:done", map[string]interface{}{
						"message": map[string]interface{}{
							"role":    messageRole,
							"content": contentBlocks,
						},
						"usage":       usage,
						"stop_reason": stopReason,
					})
				}
			}

		case "message_stop":
			fullMessage := map[string]interface{}{
				"role":    messageRole,
				"content": contentBlocks,
			}
			resultJSON, err := json.Marshal(fullMessage)
			if err != nil {
				return "", fmt.Errorf("marshal full message: %w", err)
			}
			return string(resultJSON), nil

		case "error":
			errData, _ := event["error"].(map[string]interface{})
			errMsg, _ := errData["message"].(string)
			return "", fmt.Errorf("stream error: %s", errMsg)
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	// Stream ended without message_stop — return what we have
	if len(contentBlocks) > 0 {
		fullMessage := map[string]interface{}{
			"role":    messageRole,
			"content": contentBlocks,
		}
		resultJSON, _ := json.Marshal(fullMessage)
		return string(resultJSON), nil
	}

	return "", fmt.Errorf("stream ended without message_stop")
}

// CancelChatStream cancels the currently active ChatCompletion stream.
func (a *App) CancelChatStream() {
	a.chatCancelMu.Lock()
	defer a.chatCancelMu.Unlock()
	if a.chatCancel != nil {
		a.chatCancel()
	}
}

// ModelInfo represents a model entry from the /v1/models response.
type ModelInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

// FetchModels fetches the available model list from an OpenAI-compatible /v1/models endpoint.
func (a *App) FetchModels(apiKey, baseURL string) ([]ModelInfo, error) {
	url := strings.TrimRight(baseURL, "/") + "/models"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("User-Agent", "uniTerm")

	client := &http.Client{Timeout: 10 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", res.StatusCode, string(body))
	}

	var result struct {
		Data []ModelInfo `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse models response: %w", err)
	}
	return result.Data, nil
}

// SFTP direct API — called from frontend without terminal layer

// fileTransferSession is the common interface for SFTP and FTP sessions.
type fileTransferSession interface {
	ListRemote(dir string) (session.FileListResult, error)
	ListLocal(dir string) (session.FileListResult, error)
	ChangeRemoteDir(dir string) (session.FileListResult, error)
	ChangeLocalDir(dir string) (session.FileListResult, error)
	ListLocalDrives() ([]session.FileItem, error)
	MakeDir(dir string) error
	Remove(path string, recursive bool) error
	Rename(oldPath, newPath string) error
	Chmod(path string, mode os.FileMode) error
	LocalRemove(path string, recursive bool) error
	LocalRename(oldPath, newPath string) error
	LocalMkdir(dir string) error
	Get(remotePath, localPath string, recursive bool) (string, error)
	Put(localPath, remotePath string, recursive bool) (string, error)
	PutContent(remotePath string, content []byte) error
	CancelTransfer(taskID string) error
	PauseTransfer(taskID string) error
	ResumeTransfer(taskID string) error
}

func (a *App) getSftp(sid string) (fileTransferSession, error) {
	if a.sessionManager == nil {
		return nil, fmt.Errorf("session manager not initialized")
	}
	s, ok := a.sessionManager.Get(sid)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sid)
	}
	if fs, ok := s.(fileTransferSession); ok {
		return fs, nil
	}
	return nil, fmt.Errorf("not a file transfer session: %s", sid)
}

func (a *App) SftpListRemote(sessionID, dir string) (session.FileListResult, error) {
	fs, err := a.getSftp(sessionID)
	if err != nil {
		return session.FileListResult{}, err
	}
	return fs.ListRemote(dir)
}

func (a *App) SftpListLocal(sessionID, dir string) (session.FileListResult, error) {
	fs, err := a.getSftp(sessionID)
	if err != nil {
		return session.FileListResult{}, err
	}
	return fs.ListLocal(dir)
}

func (a *App) SftpChangeRemoteDir(sessionID, dir string) (session.FileListResult, error) {
	fs, err := a.getSftp(sessionID)
	if err != nil {
		return session.FileListResult{}, err
	}
	return fs.ChangeRemoteDir(dir)
}

func (a *App) SftpChangeLocalDir(sessionID, dir string) (session.FileListResult, error) {
	fs, err := a.getSftp(sessionID)
	if err != nil {
		return session.FileListResult{}, err
	}
	return fs.ChangeLocalDir(dir)
}

func (a *App) SftpListLocalDrives(sessionID string) ([]session.FileItem, error) {
	fs, err := a.getSftp(sessionID)
	if err != nil {
		return nil, err
	}
	return fs.ListLocalDrives()
}

func (a *App) SftpMakeDir(sessionID, dir string) error {
	fs, err := a.getSftp(sessionID)
	if err != nil {
		return err
	}
	return fs.MakeDir(dir)
}

func (a *App) SftpRemove(sessionID, path string, recursive bool) error {
	fs, err := a.getSftp(sessionID)
	if err != nil {
		return err
	}
	return fs.Remove(path, recursive)
}

func (a *App) SftpRename(sessionID, oldPath, newPath string) error {
	fs, err := a.getSftp(sessionID)
	if err != nil {
		return err
	}
	return fs.Rename(oldPath, newPath)
}

func (a *App) SftpChmod(sessionID, path, mode string) error {
	fs, err := a.getSftp(sessionID)
	if err != nil {
		return err
	}
	modeUint, err := strconv.ParseUint(mode, 8, 32)
	if err != nil {
		return fmt.Errorf("invalid mode: %s", mode)
	}
	return fs.Chmod(path, os.FileMode(modeUint))
}

func (a *App) SftpLocalRemove(sessionID, path string, recursive bool) error {
	fs, err := a.getSftp(sessionID)
	if err != nil {
		return err
	}
	return fs.LocalRemove(path, recursive)
}

func (a *App) SftpLocalRename(sessionID, oldPath, newPath string) error {
	fs, err := a.getSftp(sessionID)
	if err != nil {
		return err
	}
	return fs.LocalRename(oldPath, newPath)
}

func (a *App) SftpLocalMkdir(sessionID, dir string) error {
	fs, err := a.getSftp(sessionID)
	if err != nil {
		return err
	}
	return fs.LocalMkdir(dir)
}

func (a *App) SftpGet(sessionID, remotePath, localPath string, recursive bool) (string, error) {
	fs, err := a.getSftp(sessionID)
	if err != nil {
		return "", err
	}
	return fs.Get(remotePath, localPath, recursive)
}

func (a *App) SftpCancelTransfer(sessionID, taskID string) error {
	fs, err := a.getSftp(sessionID)
	if err != nil {
		return err
	}
	return fs.CancelTransfer(taskID)
}

func (a *App) SftpPauseTransfer(sessionID, taskID string) error {
	fs, err := a.getSftp(sessionID)
	if err != nil {
		return err
	}
	return fs.PauseTransfer(taskID)
}

func (a *App) SftpResumeTransfer(sessionID, taskID string) error {
	fs, err := a.getSftp(sessionID)
	if err != nil {
		return err
	}
	return fs.ResumeTransfer(taskID)
}

func (a *App) SftpPut(sessionID, localPath, remotePath string, recursive bool) (string, error) {
	fs, err := a.getSftp(sessionID)
	if err != nil {
		return "", err
	}
	return fs.Put(localPath, remotePath, recursive)
}

func (a *App) SftpPutContent(sessionID, remotePath, contentBase64 string) error {
	fs, err := a.getSftp(sessionID)
	if err != nil {
		return err
	}
	content, err := base64.StdEncoding.DecodeString(contentBase64)
	if err != nil {
		return err
	}
	return fs.PutContent(remotePath, content)
}

// WriteTempFile writes base64-encoded content to a temp file and returns its path.
func (a *App) WriteTempFile(fileName, contentBase64 string) (string, error) {
	content, err := base64.StdEncoding.DecodeString(contentBase64)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(os.TempDir(), "uniterm")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	dst := filepath.Join(dir, fileName)
	if err := os.WriteFile(dst, content, 0644); err != nil {
		return "", err
	}
	return dst, nil
}

// RemoveTempFile removes a file created by WriteTempFile.
func (a *App) RemoveTempFile(path string) error {
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" && !strings.HasPrefix(path, homeDir) {
		// Safety: only allow removing files in temp dir
		tmpDir := filepath.Join(os.TempDir(), "uniterm")
		if !strings.HasPrefix(path, tmpDir) {
			return fmt.Errorf("path not in temp directory")
		}
	}
	return os.Remove(path)
}

// FrontendLog writes a frontend log message to the application log file.
// This is the canonical interface for the frontend to persist debug/audit
// messages alongside backend logs.
func (a *App) FrontendLog(tag string, message string) {
	_ = log.Init()
	log.Writef("[%s] %s", tag, message)
}

// GetAvailableShells returns a list of shell executables found on the system.
func (a *App) GetAvailableShells() []string {
	var shells []string
	var seen = make(map[string]bool)

	add := func(path string) {
		if path == "" {
			return
		}
		abs, err := exec.LookPath(path)
		if err != nil {
			return
		}
		// Deduplicate by normalized path (lower case, forward slashes).
		key := strings.ToLower(strings.ReplaceAll(abs, `\`, `/`))
		if seen[key] {
			return
		}
		seen[key] = true
		shells = append(shells, abs)
	}

	// Check if a shell with the given base name is already in the list.
	hasShell := func(name string) bool {
		for _, sh := range shells {
			if strings.EqualFold(filepath.Base(sh), name) {
				return true
			}
		}
		return false
	}

	switch goruntime.GOOS {
	case "windows":
		add("pwsh.exe")
		add("powershell.exe")
		add("cmd.exe")
		// Prefer explicit Git for Windows paths over WSL bash to avoid
		// WSL relay errors when no Linux distribution is installed.
		// On Windows, LookPath("bash.exe") finds C:\Windows\System32\bash.exe
		// (the WSL launcher) before Git Bash, which fails if WSL isn't set up.
		for _, p := range []string{
			`C:\Program Files\Git\bin\bash.exe`,
			`C:\Program Files (x86)\Git\bin\bash.exe`,
			`C:\ProgramData\chocolatey\bin\bash.exe`,
		} {
			add(p)
		}
		if !hasShell("bash.exe") {
			add("bash.exe")
		}
	default:
		add(os.Getenv("SHELL"))
		add("bash")
		add("zsh")
		add("fish")
		add("sh")
	}

	return shells
}

// GetDefaultShell returns the system's default shell path for local terminals.
func (a *App) GetDefaultShell() string {
	switch goruntime.GOOS {
	case "windows":
		if _, err := exec.LookPath("pwsh.exe"); err == nil {
			return "pwsh.exe"
		}
		if _, err := exec.LookPath("powershell.exe"); err == nil {
			return "powershell.exe"
		}
		// Prefer explicit Git for Windows paths over WSL bash to avoid
		// WSL relay errors when no Linux distribution is installed.
		for _, p := range []string{
			`C:\Program Files\Git\bin\bash.exe`,
			`C:\Program Files (x86)\Git\bin\bash.exe`,
		} {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
		if _, err := exec.LookPath("bash.exe"); err == nil {
			return "bash.exe"
		}
		return "cmd.exe"
	default:
		if shell := os.Getenv("SHELL"); shell != "" {
			return shell
		}
		if _, err := exec.LookPath("bash"); err == nil {
			return "bash"
		}
		return "sh"
	}
}

// ── Database methods ──

func (a *App) dbSession(sessionID string) (*session.DatabaseSession, error) {
	s, ok := a.sessionManager.Get(sessionID)
	if !ok {
		log.Writef("[dbSession] session not found: %s", sessionID)
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	ds, ok := s.(*session.DatabaseSession)
	if !ok {
		log.Writef("[dbSession] session is not a database session: %s (type=%s)", sessionID, s.Type())
		return nil, fmt.Errorf("session is not a database session: %s", sessionID)
	}
	return ds, nil
}

func (a *App) dbProvider(sessionID string) (*session.DatabaseSession, database.Provider, error) {
	ds, err := a.dbSession(sessionID)
	if err != nil {
		return nil, nil, err
	}
	p, err := database.NewProvider(ds.DBType())
	if err != nil {
		return nil, nil, err
	}
	return ds, p, nil
}

func (a *App) GetDatabases(sessionID string) ([]string, error) {
	log.Writef("[GetDatabases] sessionID=%s", sessionID)
	ds, p, err := a.dbProvider(sessionID)
	if err != nil {
		return nil, err
	}
	log.Writef("[GetDatabases] dbType=%s, fetching databases...", ds.DBType())
	dbs, err := p.GetDatabases(ds.DB())
	if err != nil {
		log.Writef("[GetDatabases] failed: %v", err)
	} else {
		log.Writef("[GetDatabases] got %d databases: %v", len(dbs), dbs)
	}
	return dbs, err
}

func (a *App) GetTables(sessionID string, dbName string) ([]database.TableInfo, error) {
	log.Writef("[GetTables] sessionID=%s, dbName=%s", sessionID, dbName)
	ds, p, err := a.dbProvider(sessionID)
	if err != nil {
		return nil, err
	}
	tables, err := p.GetTables(ds.DB(), dbName)
	if err != nil {
		log.Writef("[GetTables] failed: %v", err)
		return nil, err
	}
	sort.Slice(tables, func(i, j int) bool {
		return tables[i].Name < tables[j].Name
	})
	names := make([]string, len(tables))
	for i, t := range tables {
		names[i] = t.Name
	}
	log.Writef("[GetTables] got %d tables: %v", len(tables), names)
	return tables, nil
}

func (a *App) GetTableSchema(sessionID string, dbName string, tableName string) (*database.SchemaResult, error) {
	ds, p, err := a.dbProvider(sessionID)
	if err != nil {
		return nil, err
	}
	return p.GetTableSchema(ds.DB(), dbName, tableName)
}

func (a *App) CreateDatabase(sessionID string, dbName string) error {
	ds, p, err := a.dbProvider(sessionID)
	if err != nil {
		return err
	}
	return p.CreateDatabase(ds.DB(), dbName)
}

func (a *App) DropDatabase(sessionID string, dbName string) error {
	ds, p, err := a.dbProvider(sessionID)
	if err != nil {
		return err
	}
	return p.DropDatabase(ds.DB(), dbName)
}

func (a *App) CreateTable(sessionID string, dbName string, tableName string) error {
	ds, p, err := a.dbProvider(sessionID)
	if err != nil {
		return err
	}
	return p.CreateTable(ds.DB(), dbName, tableName)
}

func (a *App) DropTable(sessionID string, dbName string, tableName string) error {
	ds, p, err := a.dbProvider(sessionID)
	if err != nil {
		return err
	}
	return p.DropTable(ds.DB(), dbName, tableName)
}

func (a *App) TruncateTable(sessionID string, dbName string, tableName string) error {
	ds, p, err := a.dbProvider(sessionID)
	if err != nil {
		return err
	}
	return p.TruncateTable(ds.DB(), dbName, tableName)
}

func (a *App) ExecuteQuery(sessionID string, dbName string, sql string) (*database.QueryResult, error) {
	ds, p, err := a.dbProvider(sessionID)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	qr, qErr := database.ExecuteQuery(p, ds.DB(), dbName, sql)
	elapsed := time.Since(start).Milliseconds()

	entry := database.HistoryEntry{SQL: sql, Duration: elapsed}
	if qErr != nil {
		entry.Error = qErr.Error()
	} else {
		entry.RowCount = len(qr.Rows)
	}
	_ = database.SaveHistory(ds.ID(), entry)

	return qr, qErr
}

func (a *App) ExecuteStatement(sessionID string, dbName string, sql string) (*database.ExecResult, error) {
	ds, p, err := a.dbProvider(sessionID)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	er, sErr := database.ExecuteStatement(p, ds.DB(), dbName, sql)
	elapsed := time.Since(start).Milliseconds()

	entry := database.HistoryEntry{SQL: sql, Duration: elapsed}
	if sErr != nil {
		entry.Error = sErr.Error()
	} else {
		entry.RowCount = int(er.Affected)
	}
	_ = database.SaveHistory(ds.ID(), entry)

	return er, sErr
}

func (a *App) AddColumn(sessionID string, dbName string, tableName string, col database.ColumnDef) error {
	ds, p, err := a.dbProvider(sessionID)
	if err != nil {
		return err
	}
	return p.AddColumn(ds.DB(), dbName, tableName, col)
}

func (a *App) ModifyColumn(sessionID string, dbName string, tableName string, col database.ColumnDef) error {
	ds, p, err := a.dbProvider(sessionID)
	if err != nil {
		return err
	}
	return p.ModifyColumn(ds.DB(), dbName, tableName, col)
}

func (a *App) DropColumn(sessionID string, dbName string, tableName string, colName string) error {
	ds, p, err := a.dbProvider(sessionID)
	if err != nil {
		return err
	}
	return p.DropColumn(ds.DB(), dbName, tableName, colName)
}

func (a *App) AddIndex(sessionID string, dbName string, tableName string, idx database.IndexDef) error {
	ds, p, err := a.dbProvider(sessionID)
	if err != nil {
		return err
	}
	return p.AddIndex(ds.DB(), dbName, tableName, idx)
}

func (a *App) DropIndexOp(sessionID string, dbName string, tableName string, idxName string, isPrimary bool, autoIncCols []string) error {
	ds, p, err := a.dbProvider(sessionID)
	if err != nil {
		return err
	}
	return p.DropIndex(ds.DB(), dbName, tableName, idxName, isPrimary, autoIncCols)
}

func (a *App) GetDBCapabilities(sessionID string) (database.DBCapabilities, error) {
	_, p, err := a.dbProvider(sessionID)
	if err != nil {
		return nil, err
	}
	return database.MergeCapabilities(p.GetCapabilities()), nil
}

func (a *App) GetQueryHistory(sessionID string) ([]database.HistoryEntry, error) {
	return database.LoadHistory(sessionID)
}

func (a *App) ClearQueryHistory(sessionID string) error {
	return database.ClearHistory(sessionID)
}
