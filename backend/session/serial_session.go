package session

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.bug.st/serial"
)

// SerialConfig holds serial port connection parameters.
type SerialConfig struct {
	PortName string
	BaudRate int
	DataBits int
	StopBits serial.StopBits
	Parity   serial.Parity
}

type SerialSession struct {
	baseSession
	port     serial.Port
	config   SerialConfig
	quit     chan struct{}
	quitOnce sync.Once

	logMu      sync.Mutex
	logEnabled bool
	logFile    *os.File
	logPath    string
}

func NewSerialSession(id string) *SerialSession {
	return &SerialSession{
		baseSession: baseSession{
			id:          id,
			sessionType: "serial",
			status:      StatusDisconnected,
		},
		quit: make(chan struct{}),
	}
}

func (s *SerialSession) Connect(config ConnectionConfig) error {
	// Serial sessions ignore ConnectionConfig fields; they receive
	// their real config via SetSerialConfig before Connect is called.
	if s.config.PortName == "" || s.config.BaudRate == 0 {
		s.setStatus(StatusError)
		return fmt.Errorf("serial config not set: call SetSerialConfig before Connect")
	}
	s.setStatus(StatusConnecting)
	s.title = fmt.Sprintf("%s@%d", s.config.PortName, s.config.BaudRate)

	mode := &serial.Mode{
		BaudRate: s.config.BaudRate,
		DataBits: s.config.DataBits,
		StopBits: s.config.StopBits,
		Parity:   s.config.Parity,
	}

	port, err := serial.Open(s.config.PortName, mode)
	if err != nil {
		s.setStatus(StatusError)
		return fmt.Errorf("serial open %s: %w", s.config.PortName, err)
	}
	// s.port is assigned once before readLoop starts. Write() is safe to call
	// on a closed port (returns an error), matching SSH/Telnet convention of
	// not nil-ing closed handles.
	s.port = port
	s.setStatus(StatusConnected)

	go s.readLoop()
	return nil
}

// normalizeNewlines converts lone \r to \r\n so that carriage returns
// from serial devices produce proper line breaks in the terminal.
// \r\n sequences are kept as-is.
// Special cases: when \r is followed by another \r (double Enter)
// or when \r is at end of data (trailing Enter), don't add extra \n
// to avoid extra blank lines on empty command echo.
func normalizeNewlines(data []byte) []byte {
	out := make([]byte, 0, len(data)+16)
	for i := 0; i < len(data); i++ {
		b := data[i]
		if b == '\r' {
			// Check if followed by \n (keep as-is)
			if i+1 < len(data) && data[i+1] == '\n' {
				out = append(out, b)
			} else if i+1 < len(data) && data[i+1] == '\r' {
				// Double \r (double Enter): just pass through, don't add extra newline
				// This avoids the extra blank line when user presses Enter on empty prompt
				out = append(out, b)
			} else if i+1 >= len(data) {
				// Trailing \r at end of data: this is likely an empty Enter
				// Don't convert to avoid extra blank line from echo
				out = append(out, b)
			} else {
				// Lone \r not at end, convert to \r\n
				out = append(out, '\r', '\n')
			}
		} else {
			out = append(out, b)
		}
	}
	return out
}

func (s *SerialSession) SetSerialConfig(cfg SerialConfig) {
	s.config = cfg
}

func (s *SerialSession) StartLogAtPath(path string) string {
	if path == "" {
		return ""
	}
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return ""
		}
	}

	s.logMu.Lock()
	defer s.logMu.Unlock()

	if s.logFile != nil {
		_ = s.logFile.Close()
		s.logFile = nil
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return ""
	}

	s.logEnabled = true
	s.logFile = f
	s.logPath = path
	_, _ = fmt.Fprintf(f, "\n=== Serial log started at %s ===\n", time.Now().Format(time.RFC3339))
	_ = f.Sync()
	return s.logPath
}

func (s *SerialSession) StopLog() {
	s.logMu.Lock()
	defer s.logMu.Unlock()

	if s.logFile != nil {
		_, _ = fmt.Fprintf(s.logFile, "\n=== Serial log stopped at %s ===\n", time.Now().Format(time.RFC3339))
		_ = s.logFile.Sync()
		_ = s.logFile.Close()
		s.logFile = nil
	}
	s.logEnabled = false
	s.logPath = ""
}

func (s *SerialSession) IsLogEnabled() bool {
	s.logMu.Lock()
	defer s.logMu.Unlock()
	return s.logEnabled
}

func (s *SerialSession) logChunk(data []byte) {
	if len(data) == 0 {
		return
	}

	s.logMu.Lock()
	defer s.logMu.Unlock()
	if !s.logEnabled || s.logFile == nil {
		return
	}
	if _, err := s.logFile.Write(data); err == nil {
		_ = s.logFile.Sync()
	}
}

func (s *SerialSession) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := s.port.Read(buf)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			if s.IsZmodemMode() {
				s.emitBinary(data)
			} else if looksLikeZmodemHeader(data) {
				s.SetZmodemMode(true)
				s.emitBinary(data)
			} else {
				normalized := normalizeNewlines(data)
				s.logChunk(normalized)
				s.emitData(normalized)
			}
		}
		if err != nil {
			if err != io.EOF {
				s.emitData([]byte(fmt.Sprintf("\r\n\x1b[31m[Serial read error: %v]\x1b[0m\r\n", err)))
			} else {
				s.emitData([]byte("\r\n\x1b[31mSerial device disconnected. Press Enter to reconnect.\x1b[0m\r\n"))
			}
			s.Disconnect()
			return
		}
	}
}

func (s *SerialSession) Write(data []byte) error {
	if s.port == nil {
		return fmt.Errorf("serial port not connected")
	}
	_, err := s.port.Write(data)
	return err
}

func (s *SerialSession) Disconnect() error {
	s.quitOnce.Do(func() {
		close(s.quit)
		if s.port != nil {
			s.port.Close()
		}
		s.StopLog()
		s.setStatus(StatusDisconnected)
	})
	return nil
}

func (s *SerialSession) Resize(cols, rows int) error {
	// Serial sessions don't support terminal resize in the SSH sense.
	// Store pending size for consistency but it's a no-op.
	s.SetPendingSize(cols, rows)
	return nil
}

func (s *SerialSession) IsConnected() bool {
	return s.Status() == StatusConnected
}

// ListSerialPorts returns the names of available serial ports.
func ListSerialPorts() ([]string, error) {
	ports, err := serial.GetPortsList()
	if err != nil {
		return nil, err
	}
	names := make([]string, len(ports))
	for i, p := range ports {
		names[i] = p
	}
	return names, nil
}
