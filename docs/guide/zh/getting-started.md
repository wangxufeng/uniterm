# 安装与首次连接

本指南将帮助您下载安装 uniTerm 并建立第一个连接。


## 下载与安装

### Windows

从 [GitHub Releases](https://github.com/ys-ll/uniterm/releases/latest) 下载 `uniterm-amd64-installer.exe` 安装包，双击运行即可。

或者下载 `uniterm.exe` 便携版，解压后直接运行。

### macOS

下载 `uniterm.darwin-universal.dmg`，打开后将 uniTerm 拖入 Applications 文件夹。

### Linux

下载 `uniterm.linux-amd64.tar.gz`，解压后运行：

```bash
tar xzf uniterm.linux-amd64.tar.gz
./uniterm
```


## 创建第一个连接

1. 打开 uniTerm，点击左侧边栏的 **+** 按钮，或点击"新建连接"卡片。

   ![新建连接](/imgs/new_connection_light.webp)

2. 在弹出的新建连接对话框中，选择协议类型（例如 **SSH**），填写连接信息：
   - **名称**：给连接起一个易识别的名字
   - **主机**：服务器 IP 或域名
   - **端口**：协议默认端口会自动填充
   - **用户名**：登录用户名
   - **密码 / 密钥**：选择认证方式

3. 点击 **确定**，连接将出现在左侧列表中。

4. 双击连接，即可打开终端/会话。


## 界面概览

uniTerm 的主界面主要分为以下区域：

- **左侧边栏** — 连接列表，按分组管理您的所有连接
- **中央终端区** — 终端 Tab 页，支持拖拽分栏组成工作区
- **右侧面板** — 文件浏览器 / AI 助理


## 下一步

- 查看 [开始页](/zh/start-page) 了解开始页的使用方法
- 查看 [远程终端](/zh/connections/remote-terminal) 了解 SSH/Telnet/Mosh 的详细用法
- 查看 [AI 助理](/zh/features/ai-assistant) 配置 AI Agent
- 查看 [个性化](/zh/features/personalization) 调整主题、快捷键和语言
