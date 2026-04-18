# gorinth

## Work in progress

TODO: Update documentation, clean up backup code 

CLI tool for updating Minecraft server mods from Modrinth, written in Go.

```bash
Usage:
  gorinth [flags]
  gorinth [command]

Available Commands:
  check       Check for updates to your Minecraft server mods without applying them.
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  update      Check for updates and apply them to your Minecraft server mods.

Flags:
  -d, --debug                  Enable debug logging
      --dir string             Directory to store mods (default ".")
      --game-version string    Minecraft version to check for updates (default "1.20.4")
  -h, --help                   help for gorinth
      --loader string          Mod loader to check for updates (e.g. fabric, forge) (default "fabric")
      --mode local             Mode of operation: local or `sftp` (default "local")
      --sftp-host string       SFTP host for remote server (default "localhost")
      --sftp-password string   SFTP password for remote server (default "password")
  -p, --sftp-port int          SFTP port for remote server (default 22)
  -u, --sftp-user string       SFTP username for remote server (default "user")
```


### Configuration
You can configure gorinth using a `config.yaml` file or environment variables. The configuration options include:

**config.yaml:**
```yaml
  # Mode of operation: local filesystem or `sftp` (default: "sftp")
  mode:
  # SFTP host for remote server
  sftp-host`: 
  # SFTP port for remote server
  sftp-port`: 
  # SFTP username for remote server
  sftp-user`: 
  # SFTP password for remote server 
  sftp-password`: 
  # Minecraft version to check for updates (default: "1.20.4")
  game-version`: 
  # Mod loader to check for updates (e.g. fabric, forge) (default: "fabric")
  loader`: 
  # Directory to store mods (default: ".")
  dir`: 
```

**Environment variables:**
```bash
  GORINTH_SFTP_HOST=localhost
  GORINTH_SFTP_PORT=22
  GORINTH_SFTP_USER=user
  GORINTH_SFTP_PASSWORD=password
  GORINTH_GAME_VERSION=1.21.1
  GORINTH_LOADER=fabric
  GORINTH_DIR=mods
```