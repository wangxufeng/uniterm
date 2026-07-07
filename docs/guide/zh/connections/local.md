# 本地连接

无需网络，直接在本机打开终端或连接串口设备。

## Local Shell

### 连接参数

| 参数 | 说明 |
|------|------|
| Shell 类型 | 选择要打开的本地 Shell |
| 启动脚本 | 终端启动后自动执行的命令或脚本（可选） |

支持的 Shell 类型（根据操作系统不同）：

**Windows：**
- PowerShell
- CMD
- Git Bash
- WSL（已安装的 Linux 发行版）

**macOS / Linux：**
- bash
- zsh
- 其他已安装的 Shell

## Serial（串口）

连接串口设备，如路由器控制台、嵌入式开发板、工业设备等。

### 连接参数

| 参数 | 说明 |
|------|------|
| 端口 | 串口号（Windows 下为 COMx，Linux 下为 /dev/ttyUSBx） |
| 波特率 | 传输速率，常用 9600、115200 |
| 数据位 | 每帧数据位，常用 8 |
| 停止位 | 停止位，常用 1 |
| 校验位 | 错误检测，可选 None、Even、Odd |
| 本地回显 | 是否在本地显示输入内容，按需开启 |


::: tip 相关内容
- [远程终端](/zh/connections/remote-terminal) —— SSH/Telnet/Mosh 远程连接
:::
