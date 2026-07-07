# Supported Protocols

uniTerm supports 20+ connection protocols, covering remote terminals, remote desktops, file transfers, databases, and server monitoring.

## Remote Terminal

| Protocol | Default Port | Description |
|------|----------|------|
| SSH | 22 | Encrypted remote shell, supports password/key authentication and tunnel forwarding |
| Telnet | 23 | Plaintext remote terminal, suitable for embedded devices and legacy systems |
| Mosh | 22 (SSH) | UDP-based mobile shell, ideal for high-latency networks |

## Local Connections

| Type | Description |
|------|------|
| Local Shell | PowerShell, CMD, Git Bash, bash, zsh, and more |
| WSL | Windows Subsystem for Linux, opens installed Linux distributions |
| Serial | Serial port connection, configurable baud rate, data bits, stop bits, parity |

## Remote Desktop

| Protocol | Default Port | Description |
|------|----------|------|
| RDP | 3389 | Windows Remote Desktop |
| VNC | 5900 | Linux remote control |
| SPICE | 5900 | KVM/QEMU virtual machine desktop |

## File Transfer

| Protocol | Default Port | Description |
|------|----------|------|
| SFTP | 22 (SSH) | SSH-based secure file transfer |
| FTP / FTPS | 21 | Traditional file transfer and its encrypted version |
| SMB | 445 | Windows file sharing |
| WebDAV | 80 / 443 | HTTP-based file management |
| S3 | Custom | S3 API-compatible object storage |

## Databases

| Database | Default Port |
|--------|----------|
| MySQL | 3306 |
| PostgreSQL | 5432 |
| Oracle | 1521 |
| SQL Server | 1433 |
| rqlite | 4001 |
| Redis | 6379 |
