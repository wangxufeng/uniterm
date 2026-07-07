# Installation and First Connection

This guide will help you download, install uniTerm, and establish your first connection.


## Download and Installation

### Windows

Download the `uniterm-amd64-installer.exe` installer from [GitHub Releases](https://github.com/ys-ll/uniterm/releases/latest) and double-click to run.

Alternatively, download the `uniterm.exe` portable version, extract it, and run directly.

### macOS

Download `uniterm.darwin-universal.dmg`, open it, and drag uniTerm into the Applications folder.

### Linux

Download `uniterm.linux-amd64.tar.gz`, extract it, and run:

```bash
tar xzf uniterm.linux-amd64.tar.gz
./uniterm
```


## Creating Your First Connection

1. Open uniTerm, click the **+** button in the left sidebar, or click the "New Connection" card.

   ![New Connection](/imgs/new_connection_light.png)

2. In the New Connection dialog, select the protocol type (e.g., **SSH**) and fill in the connection details:
   - **Name**: Give the connection an easily recognizable name
   - **Host**: Server IP or domain name
   - **Port**: The protocol's default port will be filled in automatically
   - **Username**: Login username
   - **Password / Key**: Choose an authentication method

3. Click **OK**, and the connection will appear in the left-hand list.

4. Double-click the connection to open the terminal/session.


## Interface Overview

uniTerm's main interface is divided into the following areas:

- **Left Sidebar** — Connection list, where all your connections are managed by groups
- **Central Terminal Area** — Terminal tabs, supporting drag-and-drop splits to form workspaces
- **Right Panel** — File browser / AI Assistant


## Next Steps

- See [Start Page](/en/start-page) to learn how to use the start page
- See [Remote Terminal](/en/connections/remote-terminal) for detailed usage of SSH/Telnet/Mosh
- See [AI Assistant](/en/features/ai-assistant) to configure the AI Agent
- See [Personalization](/en/features/personalization) to adjust themes, keyboard shortcuts, and language
