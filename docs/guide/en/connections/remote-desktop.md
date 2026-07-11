# Remote Desktop

uniTerm integrates three remote desktop protocols, providing a smooth graphical remote control experience.

## RDP

RDP (Remote Desktop Protocol) is the built-in remote desktop protocol of Windows.

> Only supported on Windows clients. This feature is hidden on macOS and Linux versions. Uses the system's built-in Remote Desktop control.

![RDP](/imgs/rdp_light.webp)

### Connection Parameters

| Parameter | Description |
|------|------|
| Host | Windows host IP or domain name |
| Port | Default 3389 |
| Username | Windows login username |
| Password | Windows login password |
| Resolution | Optional fixed resolution (800x600 to 2560x1440), default 1280x720 |
| Smart Scaling | When enabled, the remote desktop automatically scales to fit the window size |
| SSH Tunnel | Select an existing SSH connection as a jump host |

## VNC

VNC (Virtual Network Computing) is widely used for remote control of Linux systems, transmitted via the RFB protocol.

### Connection Parameters

| Parameter | Description |
|------|------|
| Host | VNC server IP or domain name |
| Port | Default 5900. Values less than 100 are treated as libvirt display numbers (5900 is automatically added) |
| Password | VNC authentication password |
| SSH Tunnel | Select an existing SSH connection as a jump host |

## SPICE

SPICE (Simple Protocol for Independent Computing Environments) is optimized for KVM/QEMU virtual machines, providing a high-performance virtual desktop experience.

### Connection Parameters

| Parameter | Description |
|------|------|
| Host | SPICE server IP or domain name |
| Port | Default 5900. Values less than 100 are treated as libvirt display numbers (5900 is automatically added) |
| Password | SPICE authentication password |

> SPICE does not support SSH tunnels.


::: tip Related
- [Remote Terminal](/en/connections/remote-terminal) -- SSH tunnels can be used to secure RDP/VNC connections
:::
