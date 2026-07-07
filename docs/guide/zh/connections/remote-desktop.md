# 远程桌面

uniTerm 集成了三种远程桌面协议，提供流畅的图形化远程控制体验。

## RDP

RDP（Remote Desktop Protocol）是 Windows 系统内置的远程桌面协议。

> 仅支持 Windows 版本客户端，macOS 和 Linux 版本该功能隐藏。使用系统内置的远程桌面控件。

![RDP](/imgs/rdp_light.png)

### 连接参数

| 参数 | 说明 |
|------|------|
| 主机 | Windows 主机 IP 或域名 |
| 端口 | 默认 3389 |
| 用户名 | Windows 登录用户名 |
| 密码 | Windows 登录密码 |
| 分辨率 | 可选固定分辨率（800×600 到 2560×1440），默认 1280×720 |
| 智能缩放 | 开启后远程桌面自动缩放适配窗口大小 |
| SSH 隧道 | 选择已有的 SSH 连接作为跳板机 |

## VNC

VNC（Virtual Network Computing）广泛用于 Linux 系统远程控制，通过 RFB 协议传输。

### 连接参数

| 参数 | 说明 |
|------|------|
| 主机 | VNC 服务器 IP 或域名 |
| 端口 | 默认 5900。小于 100 则视为 libvirt 显示器编号（自动加 5900） |
| 密码 | VNC 认证密码 |
| SSH 隧道 | 选择已有的 SSH 连接作为跳板机 |

## SPICE

SPICE（Simple Protocol for Independent Computing Environments）专为 KVM/QEMU 虚拟机优化，提供高性能的虚拟桌面体验。

### 连接参数

| 参数 | 说明 |
|------|------|
| 主机 | SPICE 服务器 IP 或域名 |
| 端口 | 默认 5900。小于 100 则视为 libvirt 显示器编号（自动加 5900） |
| 密码 | SPICE 认证密码 |

> SPICE 不支持 SSH 隧道。


::: tip 相关内容
- [远程终端](/zh/connections/remote-terminal) —— SSH 隧道可用于保护 RDP/VNC 连接
:::
