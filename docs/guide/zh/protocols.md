# 支持协议

uniTerm 支持 20+ 种连接协议，覆盖远程终端、远程桌面、文件传输、数据库和服务器监控。

## 远程终端

| 协议 | 默认端口 | 说明 |
|------|----------|------|
| SSH | 22 | 加密远程 Shell，支持密码/密钥认证、隧道转发 |
| Telnet | 23 | 明文远程终端，适用于嵌入式设备和遗留系统 |
| Mosh | 22 (SSH) | 基于 UDP 的移动 Shell，适合高延迟网络 |

## 本地连接

| 类型 | 说明 |
|------|------|
| Local Shell | PowerShell、CMD、Git Bash、bash、zsh 等 |
| WSL | Windows Subsystem for Linux，打开已安装的 Linux 发行版 |
| Serial | 串口连接，可配置波特率、数据位、停止位、校验位 |

## 远程桌面

| 协议 | 默认端口 | 说明 |
|------|----------|------|
| RDP | 3389 | Windows 远程桌面 |
| VNC | 5900 | Linux 远程控制 |
| SPICE | 5900 | KVM/QEMU 虚拟机桌面 |

## 文件传输

| 协议 | 默认端口 | 说明 |
|------|----------|------|
| SFTP | 22 (SSH) | 基于 SSH 的安全文件传输 |
| FTP / FTPS | 21 | 传统文件传输及加密版本 |
| SMB | 445 | Windows 文件共享 |
| WebDAV | 80 / 443 | 基于 HTTP 的文件管理 |
| S3 | 自定义 | 兼容 S3 API 的对象存储 |

## 数据库

| 数据库 | 默认端口 | 说明 |
|--------|----------|------|
| MySQL | 3306 | 兼容 MySQL 协议：MySQL、MariaDB、TiDB 等 |
| PostgreSQL | 5432 | 兼容 PostgreSQL 协议：PostgreSQL、CockroachDB 等 |
| Oracle | 1521 | 通过纯 Go 驱动连接 Oracle Database |
| SQL Server | 1433 | 通过纯 Go 驱动连接 SQL Server |
| rqlite | 4001 | 基于 SQLite、Raft 共识的轻量分布式数据库 |
| Redis | 6379 | 内存键值数据库，可视化键值浏览与编辑 |
| MongoDB | 27017 | 文档数据库，树形浏览、查询与行内编辑 |
