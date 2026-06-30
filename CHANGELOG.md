# Changelog

## v1.2.1

- **new** Added Redis connection support with visual key browser and value editor.
- **new** SFTP bookmark button for quick directory navigation.
- **new** Notification dot on inactive tabs when terminal receives new output.
- **improve** Added local terminal type in connection form with post-login script support.
- **improve** Added serial port connection type with custom baud rate input.
- **improve** AI `send_terminal_key` tool now supports `send_enter` parameter (default true) for auto-Enter after input.
- **improve** Enhanced AI conversation markdown rendering with broader syntax support and improved dark theme readability.
- **improve** Improved connection retry flow with auto-focus and pre-filled credential dialog.
- **bugfix** Fixed macOS title bar button misalignment. (@surenwuyuwuqiu)
- **bugfix** Fixed SFTP fallback to SFTP-level copy when remote cp command fails.

- **bugfix** Fixed garbled CJK input and broken editing keys (Backspace, etc.) in zsh on macOS local terminal. (@surenwuyuwuqiu)

Thanks to @surenwuyuwuqiu for contributions to this release.

## v1.2.0

- **new** Added Oracle database support. (@yuwei5380)
- **new** Added SQL Server database support.
- **new** Moved database CRUD SQL logic to backend for unified management.
- **new** Database page now includes query/object tabs with view-aware actions.
- **new** Added Expect/Send auto-login support for jumphost and similar scenarios. (@yuwei5380)
- **new** Added file picker button for SSH key path input. (@yuwei5380)
- **new** SSH connections now support terminal character encoding configuration.
- **new** Added collapsible advanced section to connection form.
- **new** Added app config section to sidebar personalization panel.
- **new** Added 4px padding around terminal content.
- **improve** Dark loading mask for monitor refresh to avoid white flash.
- **bugfix** Fixed SSH key authentication failure. (@yuwei5380)
- **bugfix** Fixed OOM when processing large files in sz/rz. (@yuwei5380)
- **bugfix** Fixed macOS header double-click maximize not working. (@surenwuyuwuqiu)
- **bugfix** Fixed command history not recording with zsh/oh-my-zsh Unicode prompt glyphs.
- **bugfix** Fixed SFTP breadcrumb freezing on overflow boundary.
- **bugfix** Fixed database grid not refreshing immediately after inline edit/delete.
- **bugfix** Fixed missing key file picker logic in connection form.

Thanks to @yuwei5380 and @surenwuyuwuqiu for their contributions to this release.

## v1.1.2

- **new** Added 22 terminal themes with a sidebar personalization panel for one-click theme switching.
- **new** SFTP built-in text editor with encoding & line ending configuration.
- **new** SFTP new file/folder creation and copy/paste/cut.
- **new** Prompt for credentials when connecting without saved username or password.
- **improve** SFTP chmod dialog now supports octal permission input.
- **improve** Connection-type icons in sidebar session list.
- **improve** Sidebar visibility now persisted to local state file across restarts.
- **bugfix** Fixed history panel not updating in real time.
- **bugfix** Fixed missing overwrite confirmation when uploading files with same name in SFTP.
- **bugfix** Fixed paste event not dispatched to all panels in broadcast mode due to SFTP overlay interference.
- **bugfix** Fixed `__AI_KEY_` marker residue in AI output.
- **bugfix** Fixed AI command heredoc syntax compatibility.
- **bugfix** Fixed SFTP drive dropdown closing on mousedown before click event fires.

## v1.1.1

- **new** Prompt for SSH password directly in the terminal when no password is saved.
- **new** Detect system monospace fonts with live preview in the settings font selector.
- **new** AI model config supports API protocol switching, connection test, and custom User-Agent.
- **new** Customizable keyboard shortcuts for common actions.
- **bugfix** Fixed Linux multi-screen maximize using wrong screen dimensions.
- **bugfix** Fixed empty AI API message role causing request errors.
- **bugfix** Fixed brief UI freeze during maximize/restore on Windows 11.
- **bugfix** Fixed WSL local terminal console window flashing on startup.

## v1.1.0

- **new** Added serial port terminal connection. Supports scanning available serial ports and connecting with configurable baud rate, data bits, stop bits, and parity.
- **new** Added WSL support to local terminal. The `New Local Terminal` sidebar menu now scans and lists installed WSL distributions (e.g. `WSL - Ubuntu`), which can be opened with one click.
- **new** Added Windows portable zip artifact to the build workflow.

## v1.0.1

- **new** Quick Commands management. Sidebar panel with drag-drop groups, search filtering, keyboard navigation (arrow keys + Enter), edit dialog, and full 9-language i18n support.
- **new** History Panel. New sidebar tab displaying all terminal command history with search and copy support.
- **new** Quick command suggestions. Smart completion popup now includes matching quick command suggestions in real time.
- **new** "Upload File (rz -be)" right-click menu option in SSH panels to trigger Zmodem upload.
- **improve** New-connection button moved to sidebar top; quick command toolbar menu unified with sidebar styling.
- **improve** Terminal command history is now always recorded regardless of the smart completion setting.
- **bugfix** Fixed right-click paste in broadcast mode only applying to the current panel instead of all panels in the workspace.
- **bugfix** Fixed double input after SSH reconnect via generation counter guard.
- **bugfix** Fixed history panel tooltip, button layout, and text brightness issues.
- **bugfix** Fixed session data replay on terminal reuse during panel merge/split.
- **bugfix** Fixed text highlight clearing terminal background colors.
- **bugfix** Fixed escape-sequence guard failing to skip TUI lines, causing highlight interference in vim/k9s.
- **bugfix** Fixed SSH keepalive switching to global request to prevent auto-disconnect on some servers.

## v2026.06.17

- **bugfix** Fixed URL highlight causing all subsequent text to be underlined in the terminal.
- **bugfix** Fixed terminal canvas blocking window edge resize by adding 3px padding to the tab area.
- **improve** Tightened dialog padding and form item spacing; form labels now auto-expand row height when text wraps.
- **improve** All dialogs are now draggable by default.
- **improve** Unified select dropdown font size to 12px.
- **improve** Update notification link now opens in the system browser instead of the built-in WebView2 window.
- **improve** New connection button now shows "Save & Connect" consistently, same as the edit dialog.

## v2026.06.16-alpha

- **bugfix** Fixed local terminal tab close causing entire app to crash. Root cause: multiple goroutines concurrently calling `ConPTY.Close()` → double `ClosePseudoConsole` on Windows triggers OS-level access violation unrecoverable by Go's `recover()`. Fix: wrap entire `Disconnect()` body in `sync.Once`.
- **bugfix** Fixed terminal output loss when switching tabs. Data arriving during KeepAlive deactivation was buffered in sessionStore but never replayed on reactivation. Track written chunk count and replay missed chunks in `onActivated`.
- **bugfix** Fixed Enter key not triggering reconnect after SSH disconnect occurred while tab was in background. `session:status` event was dropped by `isActive` guard, so `retryOnEnter` never got set. Sync from `sessionStore.getStatus()` on reactivation.
- **bugfix** Fixed suggestion popup stuck at top-left after SSH reconnect. `terminalInput` held stale reference to disposed terminal, cursor tracking returned `{0,0}`. Recreate `terminalInput` when session ID changes.
- **bugfix** Fixed terminal content being cleared after reconnect. Release + acquire created a new xterm.js instance. Use `transferTerminal()` to move the existing terminal entry to the new session ID, preserving scrollback.
- **bugfix** Fixed pressing Ctrl+G / Shift+G / PageDown in vim leaving rendering residue. `\x1b[2J` replacement with scrollClear was also applied in alternate screen buffer, corrupting vim's screen state. Now only applies to main buffer.
- **bugfix** Fixed sidebar resize handle blocking the connection list scrollbar. Moved activation area outside the sidebar edge via negative `right` offset.
- **improve** Text highlighting overhaul:
  - Highlight only resets foreground color (`\x1b[39m`) instead of all SGR (`\x1b[0m`), preserving vim's reverse video selection
  - Lines with display attributes (reverse video, bold, etc.) skip highlighting to avoid color mixup
  - File path regex now matches directories and files without extensions, anchored by `(^|\s)` to avoid false positives inside words
  - Added datetime formats: `HH:MM` without seconds, ISO 8601 `Z` suffix, syslog `Mon DD HH:MM:SS`, weekday+year `Wed Jan 21 HH:MM:SS YYYY`
  - Color palette extracted into named constants; number color 145→152, brace color 147→223 for better contrast on vim reverse video
  - Local terminal sessions no longer apply text highlighting
  - Suggestion popup no longer triggers on arrow key navigation when closed
- **refactor** Merged `WorkspaceTabItem` into `TabItem`, eliminating ~300 lines of duplicate code. All tab types (terminal, workspace, SFTP, RDP, VNC, etc.) handled by a single component.
- **improve** Tab close buttons replaced plain `×` text with Lucide `X` SVG icon, adjusted sizing and spacing to prevent text shift when switching tabs. AI lock button always visible. SFTP and Monitor context menu items hidden for local terminal panels.
- **bugfix** Fixed AI lock state being cleared when panel detached from workspace tab.

## v2026.06.14-alpha

- **new** SSH tunnel (local port forwarding). Any connection can use an existing SSH connection as a jump host. Auto-assigns local port, tunnels TCP through SSH. VNC ports automatically adjusted for libvirt display numbers.
- **new** FTP/FTPS file transfer. New File Transfer category with FTP and FTPS (explicit TLS), passive/active mode, configurable character encoding. Reuses the SFTP two-pane file manager UI; Go backend uses shared fileTransferSession interface.
- **new** SFTP max concurrent transfers (per SSH connection, default 5). Semaphore-based concurrency control prevents bandwidth saturation and server MaxSessions limits.
- **new** Connection form now has four categories: Terminal / File Transfer / Remote Desktop / Database. SSH labeled as SSH (SFTP), appears under both Terminal and File Transfer.
- **improve** All notifications now have a close button and auto-dismiss after 5 seconds. Unified via `services/message.ts` wrapper.
- **improve** KeepAlive cache extended to all tab components (Settings/SFTP/RDP/VNC/SPICE). Switching tabs no longer rebuilds components.
- **improve** Fonts switched to system native font stack, removing Google Fonts CDN dependency. UI uses system interface fonts, monospace uses system-provided fixed-width fonts. CJK fallback covers Windows/macOS/Linux.
- **bugfix** Fixed KeepAlive-cached SFTP instances picking up global drag events, causing files to upload to the wrong connection. Document event listeners now managed via onActivated/onDeactivated.
- **bugfix** Fixed stale edit data leaking into the quick-new-connection form.
- **bugfix** Fixed duplicate task IDs from identical nanosecond timestamps in concurrent transfers causing jumbled progress bars. Switched to atomic counter.
- **bugfix** Fixed port input min=1 preventing value 0.
- **bugfix** Fixed 4px body padding preventing the titlebar from being flush with the window edge.
- **bugfix** Fixed 4px gap between local terminal submenu and its trigger causing submenu to close on mouse enter.
- **bugfix** Fixed AI confirmation level dropdown button missing ChevronDown icon import.
- **bugfix** Fixed default port not updating when switching between remote desktop types (e.g. RDP 3389 → VNC still showing 3389).

## v2026.06.13-alpha.1

- **new** Update checker. Manual check + auto-check for GitHub Releases. Settings About page shows current version, notification on new release with view details link.
- **fix** macOS rounded corners now use native Wails TitleBarHiddenInset, removing CSS border-radius workaround that caused a visible square frame.
- **fix** macOS traffic lights now use system native controls instead of custom simulated buttons.

## v2026.06.13-alpha

- **new** AI terminal toolchain. 5 new tools: start_command (fire-and-forget), capture_terminal (read screen), collect_output (passive wait), send_terminal_key (interactive input), interrupt_command (cancel). execute_command gains configurable timeout and output truncation.
- **new** AI SSE streaming. Go backend proxies Anthropic SSE events, frontend renders tokens in real time via ai:token.
- **new** AI context management. Layered system prompt (static cached + dynamic injected), token-aware context window management for improved prompt cache hit rate.
- **new** AI IN boxes show tool type names with i18n and parsed parameters per tool type. Headers display tool name with timeout `[xxs]`, body shows command/params instead of raw JSON.
- **new** AI sidebar search. Highlight matches, navigate matches (Enter / Shift+Enter), match count, auto-scroll to active match.
- **bugfix** Fixed text search menu opening search bar in all terminal windows simultaneously. Event now targets the current panel.
- **improve** Rewritten AI system prompt with timeout guidelines, decision tree, interactive prompt handling, and clear-screen prohibition.

## v2026.06.12-alpha

- **new** Zmodem file transfer (rz/sz). Upload (including drag-and-drop onto terminal) and download files in SSH terminals via `rz -be` and `sz`, with real-time progress bars.
- **new** SSH panel header "..." dropdown menu and tab right-click menu now include Duplicate Session, Connect SFTP, Server Monitor, and Text Search.
- **improve** Refactored terminal instance management so xterm instances are reused across workspace panel and standalone tab drag-and-drop merge/detach, eliminating garbled text during transitions.
- **bugfix** Fixed double-click text selection not copying to clipboard. Replaced mousedown/mouseup tracking with xterm's native onSelectionChange event.

## v2026.06.10-alpha

- **new** AI model list sync. One-click fetch available models from the server in the model edit dialog, with autocomplete suggestions in the model input.
- **new** Sidebar search now supports filtering by connection type (Terminal / Remote Desktop / Database), combined with text search.
- **new** Multilingual support. Supports 9 languages (zh-CN, zh-TW, en, ja, ko, de, es, fr, ru) with real-time switching in settings.
- **improve** Simplified AI command markers (echo-only), removed self-check from system prompt, expanded run confirmation panel by default.
- **improve** Fixed AI confirm-write button to use primary color style.
- **improve** Window title simplified, showing only "uniTerm" without version number.
- **bugfix** Fixed terminal losing focus after paste, causing invisible cursor.
- **bugfix** Fixed CSI response sequences being echoed as garbage text by bash on tab switch.

## v2026.06.08-alpha

- **new** SPICE remote desktop protocol support.
- **new** Panel duplicate, rename, drag image preview, and title synchronization.
- **new** Drag active terminal tab to adjacent tab with workspace merge.
- **improve** SSH keepalive changed from global request to session channel request (`keepalive@openssh.com`), matching OpenSSH `ServerAliveInterval` behavior; interval adjusted from 30s to 60s, max failures from 2 to 3.
- **improve** On Windows, prefer Git Bash over WSL and fix WSL bash argument passing.
- **improve** Suggestion popup position fixed in multi-panel workspace; SFTP scroll behavior improved.
- **bugfix** Fixed terminal size not updating after SSH reconnect. New sessions default to 80×24 PTY; now forces a `SessionResize` with the current terminal dimensions when reconnected, so apps like vim/k9s display at the correct size.
- **bugfix** Fixed `clear` command destroying scrollback history. Replaces ED2 (clear screen) with newline scrolling + home, pushing viewport content into scrollback before clearing.
- **bugfix** Fixed text highlighting disappearing after tab switch. Restored history now applies `highlight()` based on the current `highlightEnabled` setting.
- **bugfix** Fixed copy-on-select overwriting clipboard when switching panels or returning from another app. Copy now only triggers when the mouse selection actually started inside the same terminal.
- **bugfix** Fixed dropping panel/tab onto empty tab bar area not working in certain layouts.

## v2026.06.02-alpha

- **new** Telnet and Mosh connection protocol support. Telnet provides IAC negotiation (binary mode, terminal type, window size); Mosh uses UDP-based SSP protocol for low-latency mobile connections.
- **new** Terminal text highlighting. Automatically highlights timestamps, IP addresses, URLs, file paths, keywords (ERROR/WARN/INFO), quoted strings, numbers, and punctuation in terminal output. Toggle in settings; lines containing ESC are skipped to avoid TUI interference.
- **new** xterm.js Unicode11 addon for correct emoji and wide character rendering (e.g. k9s dog icon).
- **improve** Merged tab bar into titlebar as a single row, saving ~40px vertical space. All buttons icon-only, new connection + local terminal merged into `+` dropdown, window controls styled consistently.
- **improve** New/Edit connection dialog restructured into two-level category selection (Terminal / Remote Desktop / Database) with radio-button toggle for protocol sub-type.
- **improve** Unified control sizing across the entire UI: 28px height, 12px font, consistent border-radius, background, and border colors for all controls (el-input, el-button, el-select, el-radio-button, el-switch, el-checkbox, etc.).
- **improve** Smart completion UX fixes: popup flips above/below intelligently to avoid covering input; mouse hover only activates after movement to prevent accidental selection; password (hidden) input is not saved to history and does not trigger suggestions.
- **improve** Terminal tabs restyled as buttons with accent border + background for active state, AI lock + active effects combined.
- **improve** AI sidebar defaults to new session on restart; empty sessions are not saved; max 15 sessions retained.
- **bugfix** Fixed terminal history capture logic. Scans visible buffer area (bottom to top) instead of relying on cursorY, which is unreliable after buffer scrolling.
- **bugfix** Fixed race condition where suggestion popup remained open after Enter (debounce timer cancellation + empty token check).
- **bugfix** Fixed WebView2 conflict causing input failure when opening multiple processes on Windows 11. UserDataFolder now uses a per-process PID-isolated path.
- **bugfix** Fixed garbled text on tab switch. xterm.js OSC color queries sent via onData were echoed back by the server as scrambled text; added OSC filtering to resolve.

## v2026.05.29-alpha

- **new** Terminal smart completion. Real-time popup with history command and AI rewrite suggestions while typing in SSH terminals. Settings page adds a command history management section with search, select-all, and batch delete.
- **new** Server monitor. Real-time monitoring for connected servers. Supports performance metrics (CPU/memory/disk/network), process list with details, listening ports, disk usage and mount info, network interfaces with bond/bridge detection.
- **new** SSH post-login script execution. Configure a script to run automatically after SSH connection; supports idle detection to avoid executing during manual interaction.
- **new** SSH keepalive to prevent idle disconnect. Sends periodic keepalive packets and shows a reconnect prompt when the connection drops.
- **improve** Sidebar splitter visibility and terminal scrollbar contrast improved for easier interaction.
- **bugfix** Fixed MySQL multi-database race condition in database query capabilities.
- **bugfix** Unified connection type label rendering between grouped and ungrouped views in the sidebar.

## v2026.05.27-alpha

- **new** Database connection and query. Supports MySQL, PostgreSQL, and rqlite. Provides SQL query execution, table schema browsing, CRUD on data rows, and tree navigation of databases/tables.
- **new** Terminal search bar. Press Ctrl+F to open the search bar; highlights matches and counts results using @xterm/addon-search.
- **new** Connection sidebar and AI sidebar visibility states are now persisted to localStorage, restoring expand/collapse state after restart.
- **improve** Scrollbar width increased from 5px to 8px for easier grabbing.
- **improve** AI sidebar maximize button now shows a shrink icon when expanded for clearer indication.
- **improve** Added transparent padding around window edges so the terminal can still be resized by dragging even when it fills the edge.
- **improve** Workspace tabs now display the LayoutDashboard icon.
- **bugfix** Fixed garbled text appearing when switching tabs. Session buffer truncation could fall in the middle of escape sequences (DA2, OSC color queries, etc.), leaving fragments without the \x1b prefix that xterm.js rendered as garbage. Fix: scan for \x1b before the first \n to determine a safe restart boundary.

## v2026.05.25-alpha

- **bugfix** Editing and saving a connection caused passwords for other connections to be lost. Fixed an issue in the Go backend Save method where underlying array sharing inadvertently cleared passwords/APIKeys.

## v2026.05.24-alpha

- **new** Cloud config sync. Build a private cloud sync repository based on GitHub, GitLab, or Gitee private repos. All configurations (connections, AI model keys, app settings) are encrypted with AES-256-GCM before being saved remotely. Supports auto-sync, conflict resolution, master password change, and repo binding management.

## v2026.05.23-alpha

- **new** VNC remote desktop. Connect to VNC servers (TigerVNC, TightVNC, QEMU, etc.) via noVNC with a built-in WebSocket↔TCP proxy bridge. DOM remains alive across tab switches for zero-latency screen recovery. Supports auto-resize toggle and bidirectional clipboard sharing (Ctrl+Shift+V to paste local clipboard).
- **new** Local terminal support. Open a local shell directly (Windows PowerShell/CMD, macOS/Linux bash/zsh) without an SSH connection.
- **new** AI Shell awareness. AI can detect the current terminal's shell type to generate more accurate commands.
- **improve** Connection list now shows ports. Sidebar entries changed from `user@host` to `user@host:port`, displaying `host:port` when the username is empty.
- **improve** Username field is hidden when creating a new VNC connection (VNC authentication only requires a password).
- **improve** Icon unification.
- **bugfix** RDP windows now correctly hide/show when menus, dialogs, or window dragging occur, avoiding obstruction.
- **bugfix** Settings page state persistence issue fixed.
- **bugfix** Password input visibility toggle fixed.
- **bugfix** Windows 11 security dialog suppression fixed.

## v2026.05.22-alpha

- **new** RDP remote desktop. Connect to Windows Remote Desktop via the Microsoft RDP ActiveX control. Native security dialogs are fully suppressed; the RDP window is seamlessly embedded into uniTerm tabs and smoothly follows window dragging and resizing.
- **new** AI sidebar maximize button. Expands the AI assistant panel to fill the entire main area; click again to restore original width.
- **new** AI message copy functionality. Added copy buttons next to tool-call IN/OUT expand boxes with a checkmark feedback on click. Added a "Copy as Markdown" button in the top-right corner of messages that expands on hover.
- **new** Broadcast input. Added a broadcast button to the workspace panel header to send keyboard input to all terminals in the workspace simultaneously.
- **new** About section in Settings page showing the app version.
- **new** Added "Write-operation confirmation" level to AI command execution confirmation. On top of the existing three levels (Off / Dangerous only / All), added a "Dangerous + Write" option that also requires confirmation for write operations like rm and mv, filling the granularity gap between Dangerous and All.
- **new** Added "New Connection" to the connection group context menu; the group is automatically preselected when creating.
- **improve** Unified sidebar selection state. Merged `selectedId` and `multiSelectedIds` into a single `selectedIds` set for consistent highlight logic; fixed the issue where previous selection was not cleared on right-click.
- **improve** Right-clicking a connection now automatically deselects others and selects only the current one if it wasn't already in the selection.
- **improve** Tab bar improvements: supports horizontal scrolling with mouse wheel, and dropdown selections auto-scroll into view.
- **improve** System prompt message role changed from `assistant` to `tool` to avoid polluting the LLM conversation context.
- **improve** AI session storage migrated from localStorage to Go backend file storage.
- **improve** README redesign: added Chinese version, landing page, categorized feature showcase, and screenshot carousel.
- **bugfix** Fixed garbled text when switching tabs (strip incomplete escape sequences).
- **bugfix** Fixed panel active state not syncing terminal focus in workspaces.
- **bugfix** Fixed edit button incorrectly grayed out when a single connection is selected.
- **bugfix** Fixed "New Connection..." virtual item not auto-selected when search yields no matches.
- **bugfix** Fixed tab title displaying with host suffix (should show connection name only).

## v2026.05.17-alpha

- **new** Connection grouping. Supports creating, renaming, and deleting connection groups. Connections can be collapsed by group; drag-and-drop to change group assignment; ungrouped connections are automatically placed in a "(No Group)" virtual group.
- **new** Batch connection selection and actions. Supports Ctrl+click multi-select and Shift+click range select. Context menu supports batch connect, batch SFTP connect, batch copy, and batch delete. Enter key also supports batch open.
- **new** A "New Connection..." virtual item appears at the bottom of the list while typing in the search bar. Double-click or press Enter to prefill the host field and open the new connection dialog.
- **new** Confirmation prompt before deleting connections, showing the number of items to be deleted.
- **change** Light theme redesigned with a Windows-style neutral gray palette, using blue accent color instead of the previous warm yellow.
- **improve** AI session names, workspace names, AI lock button tooltips, and more now support Chinese/English internationalization.
- **bugfix** Fixed AI assistant settings button navigating to General Settings instead of AI Settings.
- **bugfix** Fixed right-click menu appearing on the ".." parent directory row in the SFTP file list.
- **bugfix** Fixed selected count being one less than actual when multi-selecting connections.

## v2026.05.16-alpha

- **new** SFTP file manager with dual-pane browsing of local and remote files. Supports upload, download, rename, delete, and permission changes. Transfer tasks are tracked independently per tab.

## v2026.05.15-alpha

- **new** Workspace panel system. Merge multiple terminal tabs into a workspace, displayed side-by-side or stacked within the same window. Drag panel headers to freely adjust panel position and size; drag tabs to panel edges to auto-create new splits.
- **new** Custom window title bar adapted for Windows and macOS platform window controls.
- **new** Terminal http/https links are auto-detected and underlined. Hover to see tooltip; Ctrl+click to open in default browser.
- **new** Windows installer now detects running processes and prompts to close the running application before installation.
- **improve** Dark theme contrast improved; both Default and Deep Blue color schemes have clearer background layers and more readable text.
- **bugfix** Selected text auto-copy to clipboard now works even if the mouse is released outside the terminal.
- **bugfix** Fixed errors caused by missing tool responses in AI multi-turn conversations.
- **bugfix** Fixed blank areas in child panels and unmovable split bars when panel splitting occurs.
- **bugfix** Fixed abnormal terminal content display when window or panel size changes.
- **bugfix** Fixed Ctrl+scroll wheel causing unexpected full-window zoom.

## v2026.05.13-alpha

- **new** Free panel splitting. Split windows left/right or top/bottom to open multiple terminals simultaneously. Drag borders to resize panels; tabs can also be dragged across panels.
- **new** AI window lock button. The "AI" button on each terminal tab locks AI to that terminal. After locking, AI command execution targets only that terminal; switching to other tabs won't send commands to the wrong place.
- **improve** Increased AI max consecutive interactions per conversation from 10 to 20. A "Continue" button appears when the limit is reached to continue the conversation; added history message length control to prevent sluggishness from overly long context.
- **new** Connection list supports up/down arrow key navigation; press Enter to connect directly without double-clicking.
- **change** Windows releases now provide installer (.exe) only, no additional zip archives. macOS releases now provide DMG only, no additional zip archives.
- **improve** Terminal default scrollback lines reduced from 5000 to 2500 to reduce memory usage.
- **bugfix** Fixed terminal content not resizing when the window or panel shrinks.
