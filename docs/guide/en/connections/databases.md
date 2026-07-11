# Databases

uniTerm has a built-in database client that supports mainstream relational databases, Redis, and MongoDB.

## Relational Databases

![Database](/imgs/database_light.webp)

### Supported Databases

| Database | Default Port |
|--------|----------|
| MySQL | 3306 |
| PostgreSQL | 5432 |
| Oracle | 1521 |
| SQL Server | 1433 |
| rqlite | 4001 |

### Connection Parameters

| Parameter | Description |
|------|------|
| Host | Database server address |
| Port | Default port auto-filled for each database type |
| Username | Database login user |
| Password | Database login password |
| Database Name | Default database to use after connecting (not required for rqlite) |
| SSH Tunnel | When enabled, encrypts the connection to intranet databases via an SSH jump host without exposing the database port |

### Database Navigation

The left-side tree panel displays the database hierarchy. After connecting, you can browse the three-level object structure: databases, tables, and views.

- **Expand/Collapse** -- Click the arrow to the left of a database name to expand the table list. Tables are shown with a grid icon, views with an eye icon for distinction
- **Search** -- Type keywords in the top search box to filter table and view names in real time. All databases are automatically expanded during search
- **Double-click to Open** -- Double-click a database name to enter its query page. Double-click a table or view name to enter the data browsing page

The context menu provides the following operations:

- **New** -- Create a new database or new table
- **Query** -- Open SQL query or data browsing
- **Manage** -- View table structure, truncate table, delete table/view/database
- **Refresh** -- Reload the table list

### Data Query

- **AI Natural Language** -- Describe the query in plain language in the AI input box at the top, and AI fetches table schemas to generate the SQL statement
- **SQL Query** -- Enter SQL and click Execute or press `Ctrl+Enter` to run. Results are displayed in a table; NULL values are shown in italics. Double-clicking a table name runs a default query for the first 100 rows
- **Insert Row** -- Automatically detects column types, default values, and auto-increment columns to generate a blank insert form
- **Edit Row** -- Click the pencil icon to open the edit form. Supports modifying all columns and toggling NULL (requires the query result to include the primary key column)
- **Delete Row** -- Click the delete icon and confirm to delete (requires the query result to include the primary key column)

### Table and View List

Double-click a database name to enter. Displays all tables and views in that database in table format, with support for search and sorting. Each row supports truncate table, delete table/view. Dangerous operations require entering the name for confirmation.

### Table Structure Browser

Double-click a table name to enter. The left tree panel displays column information and index structure, including column name, data type, nullable, default value, primary key, auto-increment, and other details.

- **Edit Column** -- Modify column type, default value, comment, and collation
- **Add Column** -- Add a new column with type auto-completion hints
- **Delete Column** -- Confirm to delete
- **Index Management** -- Add or delete indexes

## Redis

Redis provides key-value data browsing and management.

![Redis](/imgs/redis_light.webp)

### Connection Parameters

| Parameter | Description |
|------|------|
| Host | Redis server address |
| Port | Default 6379 |
| Username | Redis ACL username (optional, Redis 6+) |
| Password | Redis authentication password (optional) |
| SSH Tunnel | When enabled, encrypts the connection to intranet Redis via an SSH jump host |

### Key Browsing

The left panel shows a list of all keys, displaying key names and data types, with support for searching and filtering by name. Selecting a key displays its value content in the right panel, with different types rendered in their corresponding formats.

The context menu provides the following operations:

- **New** -- Create a new key
- **Edit** -- Modify value, rename
- **Manage** -- Delete key (requires confirmation), set or remove expiration time
- **Refresh** -- Reload the key list

### Key Operations

- **New Key** -- Select a data type and create a new key by entering the key name and value
- **View/Edit Value** -- Select a key to view its value content. Click edit to enter modification mode and save
- **Rename Key** -- Change the key name
- **Delete Key** -- Select a key and delete it (requires confirmation)

### TTL Management

- **Set Expiration** -- Set an expiration time when creating or editing a key
- **Remove Expiration** -- Remove the expiration time to make the key persistent


## MongoDB

MongoDB provides document database browsing, querying, and inline document editing.

![MongoDB](/imgs/mongodb_light.webp)

### Connection Parameters

| Parameter | Description |
|------|------|
| Host | MongoDB server address |
| Port | Default 27017 |
| Username | MongoDB login user (optional) |
| Password | MongoDB authentication password (optional) |
| SSH Tunnel | When enabled, encrypts the connection to intranet MongoDB via an SSH jump host. Connection auto-detection is available when the SSH host runs `mongod` |

### Database Navigation

The left-side tree panel displays the MongoDB hierarchy: databases, collections, and indexes.

- **Expand/Collapse** -- Click the arrow to browse databases and collections
- **Right-click Menu** -- Create/drop databases and collections via the context menu

### Query Editor

- **Filter Query** -- Enter a MongoDB Extended JSON filter and click Execute or press `Ctrl+Enter` to run. Results are displayed in a paginated table
- **Aggregation** -- Write aggregation pipelines for advanced data processing
- **AI Natural Language** -- Describe the query in plain language in the AI input box at the top, and AI generates the MongoDB filter automatically based on the collection schema

### Document Editing

- **View Document** -- Click a document to view its full content
- **Edit Document** -- Modify document fields inline and save
- **Insert Document** -- Insert a new document with JSON content
- **Delete Document** -- Delete a document with confirmation

### Index Management

View existing indexes, create new indexes with custom keys and options.

::: tip Related
- [Remote Terminal](/en/connections/remote-terminal) -- SSH tunnel configuration
:::
