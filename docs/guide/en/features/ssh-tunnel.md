# SSH Tunnel

uniTerm has a built-in SSH Tunnel Manager that supports three forwarding modes. You can create, manage, and monitor multiple tunnel connections.

![SSH Tunnel](/imgs/ssh_tunnel_light.png)

## Tunnel Manager

SSH tunnels are managed in a dedicated sidebar tab. Click the tunnel icon in the left sidebar to open the tunnel panel and view all created tunnels along with their running status.

- **Status Indicator** — Green means the tunnel is running; gray means stopped
- **Right-Click Menu** — Right-click a tunnel item to start, stop, edit, or delete the tunnel
- **Drag-and-Drop Groups** — Drag tunnel items into groups for organized management

## Three Forwarding Modes

### Local Forwarding (-L)

Map a remote server's port to your local machine, accessing the remote service through a local port.

**Typical Scenario**: A remote database (e.g., MySQL :3306) is not publicly accessible. With local forwarding mapping it to `localhost:3307`, local tools can connect directly.

```
Local Port :3307  →  SSH Tunnel  →  Remote Database :3306
```

### Remote Forwarding (-R)

Expose a local port to a remote server, allowing remote hosts to access your local service through their local port.

**Typical Scenario**: A locally developed service needs to be accessed by a remote server or external users, such as exposing a local web app through a public server.

```
Remote Port :8080  →  SSH Tunnel  →  Local Service :3000
```

### Dynamic Forwarding (-D, SOCKS5)

Set up a local SOCKS5 proxy, allowing browsers or other applications to access all services within the remote network through the proxy.

**Typical Scenario**: Access all internal web systems, APIs, etc. within a corporate network through a jump host, without configuring individual port forwards.

```
Browser  →  SOCKS5 :1080  →  SSH Tunnel  →  All Services in Remote Network
```

## Creating a Tunnel

1. Click **New Tunnel** in the tunnel panel
2. Select an **SSH Connection** as the jump host (requires an SSH connection to be created in the connection list first)
3. Choose a **Forwarding Mode**: Local, Remote, or Dynamic
4. Fill in local address/port and target address/port based on the mode (Dynamic mode only requires a local port)
5. Optional: Enable **Auto-Start** to automatically connect the tunnel on application launch

## Auto-Start

When auto-start is enabled, the tunnel will automatically connect when uniTerm starts. This is useful for tunnels that need to remain connected at all times, such as a SOCKS5 proxy for a development environment.

## Proxy Chain

Multiple tunnels can be cascaded together. One tunnel's target can point to another tunnel's entry, enabling multi-layer network penetration.

---

::: tip Related
- [Remote Terminal](/en/connections/remote-terminal) — SSH connection configuration
- [Workspace](/en/features/workspace) — Sidebar tab management
- [Server Monitor](/en/connections/server-monitor) — Real-time server monitoring via SSH
:::
