# FTP 文件传输功能设计

## 概述

新增 FTP/FTPS 连接类型，复用 SFTPTabContent 的两栏文件管理器 UI，提供远端目录浏览、上传下载、拖拽等文件传输能力。

## 需求摘要

- FTP 连接类型，支持普通 FTP 和 FTPS（TLS 加密）
- TLS 加密策略：自动（默认）/ 必须 / 不加密
- 被动模式（默认）/ 主动模式可选
- 字符编码可配置（UTF-8 / GBK 等）
- 复用现有 SFTPTabContent 两栏文件管理器 UI
- 支持 SSH 隧道

## 架构

### 新增：FTPSession

`backend/session/ftp_session.go` — 实现 `Session` 接口，使用 `github.com/jlaffaye/ftp` 库。

```
FTPSession
├── Connect(config)
│   1. 根据 ftpEncryption 策略建立连接
│   2. 登录（user/password）
│   3. 设置被动/主动模式
│
├── ListRemote(dir) → FileListResult
├── Get(remotePath, localPath) → taskID
├── Put(localPath, remotePath) → taskID
├── Remove(path, recursive)
├── Rename(oldPath, newPath)
├── Mkdir(path)
├── Chmod(path, mode)
│
└── Disconnect()
```

### 后端 API 统一

`app.go` 中新增 FTP 对应的文件操作方法（FtpListRemote、FtpPut 等），模式与 Sftp* 方法一致。前端根据 `panel.config.type` 切换调用。

### 连接配置新增字段

```go
FtpEncryption string `json:"ftpEncryption,omitempty"` // "none"(默认) | "auto" | "required"
FtpPassive    bool   `json:"ftpPassive"`              // 默认 true
FtpEncoding   string `json:"ftpEncoding,omitempty"`   // "utf-8"(默认) | "gbk" | "shift-jis" | "latin-1"
```

## 前端 UI

### ConnectionForm.vue

新增"文件传输"大类（SFTP 不在此列，因为 SFTP 依托 SSH 连接，已有右键菜单入口）。新建 FTP 连接时：
- 大类：终端 / 远程桌面 / 数据库 / **文件传输**（新增）
- 类型：**FTP**
- 端口默认 21
- TLS 加密：下拉选择（不加密 / 如果可用则加密 / 必须加密），默认不加密
- 传输模式：被动/主动切换
- 字符编码：下拉选择（UTF-8 / GBK / Shift-JIS / Latin-1），默认 UTF-8

DNS 解析：不做，交给后端直连。

### SFTPTabContent 适配

通过 `panel.config.type` 区分 `sftp` 和 `ftp`：

- 文件列表、上传、下载、删除、重命名等操作，根据 type 调用对应的 `Sftp*` 或 `Ftp*` 函数
- 传输进度事件共用 `session:data` 通道，格式一致

### Sidebar / App.vue

- 右键菜单加"连接 FTP"选项
- `onConnectFtp` 创建 FTP 类型 panel 和 tab

## 适用范围

- 普通 FTP（端口 21）
- FTPS（显式 TLS，端口 21）
- 被动模式（PASV）/ 主动模式（PORT）
- 支持 SSH 隧道

不支持：
- SFTP（已有独立实现）
- 隐式 FTPS（端口 990，较少用，后续可加）

## 涉及文件

| 文件 | 操作 |
|------|------|
| `backend/session/ftp_session.go` | **新增** |
| `backend/session/session.go` | `ConnectionConfig` 加 FTP 字段 |
| `backend/session/manager.go` | `Create` 加 `ftp` 分支 |
| `app.go` | 新增 Ftp* API 方法；`CreateSession` 支持 ftp |
| `frontend/src/types/session.ts` | `ConnectionConfig` 加 FTP 字段 |
| `frontend/src/components/ConnectionForm.vue` | 加 FTP 类型和配置项 |
| `frontend/src/App.vue` | 加 `onConnectFtp` |
| `frontend/src/components/Sidebar.vue` | 加 FTP 连接菜单 |
| `frontend/src/components/SFTPTabContent.vue` | FTP/SFTP 双协议适配 |
| `frontend/src/i18n/locales/*.json` | FTP 相关翻译 |

## 错误处理

| 场景 | 处理 |
|------|------|
| TLS 协商失败（required 模式） | 连接失败，返回错误 |
| TLS 协商失败（auto 模式） | 降级为明文连接 |
| 认证失败 | 连接失败，返回错误 |
| 字符编码不匹配 | 文件名显示乱码，用户可修改编码配置重连 |
| 主动模式被防火墙拦截 | 提示切换到被动模式 |

## 测试要点

- FTP 明文连接：列表、上传、下载、删除、重命名
- FTPS 显式 TLS：同上
- TLS 自动降级：服务器不支持 TLS 时正常连接
- 被动/主动模式切换
- 字符编码（GBK 服务器文件名正常显示）
- SSH 隧道：通过跳板机连接 FTP 服务器
- 大文件传输进度
- 传输取消/暂停/恢复
