# Local

Open a terminal directly on the local machine or connect to serial devices without a network.

## Local Shell

### Connection Parameters

| Parameter | Description |
|------|------|
| Shell Type | Select which local shell to open |
| Startup Script | Commands or scripts automatically executed after terminal starts (optional) |

Supported shell types (varies by operating system):

**Windows:**
- PowerShell
- CMD
- Git Bash
- WSL (installed Linux distributions)

**macOS / Linux:**
- bash
- zsh
- Other installed shells

## Serial

Connect to serial devices such as router consoles, embedded development boards, industrial equipment, etc.

### Connection Parameters

| Parameter | Description |
|------|------|
| Port | Serial port name (COMx on Windows, /dev/ttyUSBx on Linux) |
| Baud Rate | Transmission rate, commonly 9600 or 115200 |
| Data Bits | Data bits per frame, commonly 8 |
| Stop Bits | Stop bits, commonly 1 |
| Parity | Error detection, options: None, Even, Odd |
| Local Echo | Whether to display input locally; enable as needed |


::: tip Related
- [Remote Terminal](/en/connections/remote-terminal) -- SSH/Telnet/Mosh remote connections
:::
