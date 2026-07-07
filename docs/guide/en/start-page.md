# Start Page

The start page is the default page displayed after uniTerm launches, providing quick access entries. It also opens automatically when all tabs are closed.

![Start Page](/imgs/start_tab_light.png)

## Search and Quick Connect

The search box at the top allows searching saved connections by name, host, and protocol type. It also serves as a quick connect entry — after entering a host address, a "Quick Connect" card appears at the end of the search results. Double-click it to create a new connection to that address. It supports parsing protocol prefix formats like `ssh user@host:22`.

## Connection Cards

Connections are displayed in a card grid. Each card shows the protocol icon, connection name, and summary information (e.g., `SSH user@host:22`).

**Mouse Operations:**

- **Click** — Select a card. `Ctrl + Click` for multi-select, `Shift + Click` for range selection
- **Double-Click** — Open the connection
- **Right-Click** — Context menu with options to connect, edit, switch protocol (SSH can switch to SFTP/Monitoring), change group, duplicate connection, delete
- **Batch Operations** — After multi-selecting, right-click for batch delete or batch group change

**Keyboard Operations:**

- `Tab` — Switch focus between the search box and the card grid
- `↑` `↓` `←` `→` — Move focus between cards
- `Enter` — Open the currently focused connection (opens all selected connections when multi-selected)
- `Esc` / `Backspace` — Return to the home page from the group view

## Quick Actions

Three action buttons below the search box:

- **New Connection** — Opens the New Connection dialog
- **Local Terminal** — Dropdown to select an installed shell (PowerShell / CMD / Git Bash / WSL / bash / zsh, etc.)
- **Serial Connection** — Quickly open a serial port connection

## Recent Connections

Displays recently opened connection records (up to 20 entries), sorted by last used time. Click or double-click to quickly reconnect.

## Connection Groups

Displays all connection groups as cards, showing the group name and connection count. Ungrouped connections are categorized under "Ungrouped".

- **Enter Group** — Click or double-click a group card to enter the group detail view, showing only connections within that group
- **Breadcrumb Navigation** — The group view shows `Home / Group Name` at the top; click Home to return
- **Create Group** — Click the `+` button next to the groups to create a new one
- **Right-Click** — Rename or delete a group. When deleting, you can choose to keep or remove the connections inside
