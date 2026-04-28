# gorinth 🐮

A simple, fast, and secure CLI tool for updating Minecraft server mods directly from the [Modrinth API](https://docs.modrinth.com/).

## Work in progress
AI-generated README.

## Core Concepts

### 📂 The Mods Folder is the Source of Truth
Gorinth looks at your existing `.jar` files to determine which mods you have installed. It computes their SHA-1 hashes and asks Modrinth: "What is this, and is there an update for my specific Minecraft version and loader?"

### 🚀 Hybrid Hashing Strategy
Gorinth is designed to be fast, even over slow connections:
- **Local Mode**: Uses `afero` for standard filesystem operations.
- **SSH Mode**: Executes `sha1sum` directly on the remote server. This is **zero-bandwidth** hashing—it doesn't download the mods just to check for updates.
- **SFTP Mode (Fallback)**: If the server restricts shell access (common in Pterodactyl), Gorinth syncs a small local cache to perform hashing, ensuring compatibility with restricted environments.

### 🛡️ Safety First
- **Automatic Backups**: Creates a `.zip` or `.tar` of your mods folder before applying any changes.
- **Atomic-ish Updates**: Downloads new mods to a temporary `.gorinth-tmp` file before swapping them, preventing corrupted mods if a download is interrupted.

---

## Installation

```bash
go install github.com/Frank-/gorinth/cmd/gorinth@latest
```

---

## Usage

<!-- START_COMMAND_REFERENCE -->
```text
Gorinth is a command-line tool that helps you keep your Minecraft server mods up to date by fetching the latest versions from Modrinth.

Usage:
  gorinth [flags]
  gorinth [command]

Available Commands:
  check       Check for updates to your Minecraft server mods without applying them.
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  update      Check for updates and apply them to your Minecraft server mods.

Flags:
      --config-file string    Path to the configuration file (default "config.yaml")
  -d, --debug                 Enable debug logging
      --direct                Directly apply updates without staging area.
      --force                 Bypass safety checks and force updates. May break everything. Use with caution.
      --game-version string   Minecraft version to check for updates (default "1.20.4")
  -h, --help                  help for gorinth
      --host string           SFTP host for remote server (default "localhost")
      --loader string         Mod loader to check for updates (e.g. fabric, forge) (default "fabric")
      --mode local            Mode of operation: local or `sftp` (default "sftp")
      --mods-dir string       Directory to store mods (default "mods")
      --no-truncate           Disable truncation of mod names in the update table for better readability
      --password string       SFTP password for remote server (default "password")
  -p, --port int              SFTP port for remote server (default 22)
      --skip-backup           Skip backup creation before applying updates. Not recommended.
      --upload-backup         Upload backup to remote server after creation
  -u, --user string           SFTP username for remote server (default "user")

Use "gorinth [command] --help" for more information about a command.
```
<!-- END_COMMAND_REFERENCE -->

---

## Configuration

You can use a `config.yaml` file to save your server details:

```yaml
mode: sftp
host: "play.example.com"
user: "admin"
password: "secure-password"
game-version: "1.21.1"
loader: "fabric"
mods-dir: "mods"
```

Or use environment variables:
`GORINTH_HOST`, `GORINTH_USER`, `GORINTH_PASSWORD`, etc.

---

## Developer Notes

See [ARCHITECTURE.md](./ARCHITECTURE.md) for details on the VFS implementation and how to test against restricted environments like Pterodactyl.
