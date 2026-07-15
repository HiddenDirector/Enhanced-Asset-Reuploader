# Enhanced Asset-Reuploader

A maintained build of [kartFr/Asset-Reuploader](https://github.com/kartFr/Asset-Reuploader)
with the HTTP/upload layer reworked for reliability. All credit for the
original tool — the Roblox Studio plugin, the reupload workflow, and the Go
server design — belongs to [kartFr](https://github.com/kartFr); this
repository focuses on fixing transport-level bugs in the upload paths.

**What it does:** reuploads Roblox assets (animations, audio, meshes) to an
account or group you control, via a Roblox Studio plugin talking to a local
Go server. Needed because assets owned by another account can't be used
directly in your own experiences.

## What this version changes

All changes are in the Go server's HTTP/upload layer (see commit history for
full detail):

- **Retries actually work now.** Request handlers used to resend a shared
  `http.Request` whose body had already been consumed — every retry after a
  failure sent an *empty* POST/PATCH and stacked duplicate `Cookie` headers.
  Each attempt now builds a fresh request.
- **Connection pooling.** The HTTP transport keeps a 32-per-host idle
  connection pool (Go's default is 2), avoiding a TLS handshake per request
  during concurrent uploads.
- **Upload bodies built in memory** with a real `Content-Length` and
  `GetBody`, replacing an `io.Pipe` goroutine per attempt.
- **Audio fixes:** encoding no longer drains the buffer (the moderated-name
  retry used to upload an empty file); renames go through handler state
  instead of a stale request.
- **Crash fixes:** missing return on place-cache failure in the sound path
  (nil dereference); mesh handling guards against location entries with no
  URLs and no errors.
- **Regression tests** covering per-attempt request construction and
  multipart bodies, plus release-workflow fixes (version `ldflags`).

Everything else — plugin behaviour, supported asset types, configuration —
matches upstream. For general usage documentation and community support, see
the upstream [README](https://github.com/kartFr/Asset-Reuploader#readme);
that project's Discord and release channels belong to the original author.

## Usage

1. Install the [Asset-Reuploader plugin](https://create.roblox.com/store/asset/89096096219225/Asset-Reuploader)
   in Roblox Studio.
2. Build and run the server:

   ```bash
   go build -o asset-reuploader ./cmd/assetreuploader
   ./asset-reuploader
   ```

3. Configure via `config.ini` (port, cookie, reupload target). The startup
   message prints the port the plugin should connect to.

## Project structure

```
cmd/assetreuploader/   server entry point + router
internal/app/          per-asset handlers (animation, audio, mesh)
internal/roblox/       Roblox API client, upload requests, rate limiting
internal/retry/        retry primitives
plugin/                Roblox Studio plugin source (Luau)
plugin_tests/          plugin test suite
```

## License

GPL-3.0, inherited from the original project by kartFr. See [LICENSE](LICENSE).
