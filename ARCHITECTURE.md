# Architecture & Maintenance 🛠️

This document explains the internal design of Gorinth and how to maintain it.

## The VFS Layer (`internal/vfs`)

Gorinth uses a Virtual File System (VFS) abstraction to handle Local, SFTP, and SSH-based operations uniformly.

### 1. `FileSystem` Interface
The core interface that defines all filesystem operations:
- `ListMods()`: Returns only `.jar` files.
- `HashMods()`: Returns a map of filename to SHA-1.
- `WriteMod()`, `DeleteMod()`, `RenameMod()`: Standard CRUD for mod files.
- `CleanupTmpFiles()`: Removes files ending in `.gorinth-tmp`.

### 2. Implementation Strategies
- **`LocalFS`**: Powered by `afero.Fs`. Extremely fast and easily testable using `afero.MemMapFs`.
- **`SSHFS`**: The performance optimizer. Instead of downloading files to hash them, it runs `sha1sum <file>` on the remote server via an SSH session.
- **`SFTPFS`**: The fallback. Many game panels (like Pterodactyl) block shell access. In this case, `SFTPFS` creates a local temporary directory using `afero` and syncs the remote mods to it for local hashing.

## Why things are the way they are

### Why `afero`?
We integrated `afero` specifically to improve **testability**. By abstracting the OS filesystem, we can write comprehensive unit tests in `local_test.go` and `hash_test.go` that run entirely in memory without ever touching the disk.

### Why separate SSH and SFTP?
Pterodactyl (Wings) uses a custom SFTP server that does NOT provide a shell. 
- Traditional SSH clients expect a shell.
- Our `sftpBase` struct in `base_sftp.go` shares the logic for listing and writing files via the SFTP protocol.
- `SSHFS` extends this by adding remote command execution for hashing.
- `SFTPFS` extends this by adding local caching for hashing.

## Testing & Integration

### Unit Tests
Always run `go test ./internal/vfs/...` after changing hashing logic. 
- `hash_test.go` ensures the SHA-1 calculations and filtering are correct.
- `archive_test.go` ensures the zip-based backup system works.

### Integration Tests
We use **Testcontainers** to verify real-world SSH/SFTP behavior.
- `integration_test.go` spins up a real Linux SSH server in Docker.
- This is critical for testing the "Zero-bandwidth" hashing path (`SSHFS`).

### Simulating Pterodactyl
To test against a restricted environment:
1. Use a container that has SFTP enabled but Shell disabled (like `atmoz/sftp`).
2. Verify that `SSHFS` fails gracefully and your connection logic can handle the lack of a shell.

## Modrinth API (`internal/modrinth`)

The client uses `httptest.NewServer` in its tests (`client_test.go`) to mock API responses. This allows us to verify the "Update Detection" logic without hitting rate limits or needing an internet connection.
