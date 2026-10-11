# Releases, Upgrades, and Rollbacks

Official release tags automatically generate distribution packages for Windows, Linux (AMD64/ARM64), and macOS (Intel/Apple Silicon). Each package includes the prebuilt executable, `config.yaml` template, web admin console static assets, startup scripts, quick start guide, and these release notes. Release packages do not require Go, CGO, or external Opus libraries to be installed.

## Packages and Launching

| OS | Package | Launch Command |
| --- | --- | --- |
| Windows x64 | `xiaozhi-server-go-<version>-windows-amd64.zip` | Extract and double-click `start.bat` |
| Linux x64 / ARM64 | `xiaozhi-server-go-<version>-linux-*.tar.gz` | Extract and run `./start.sh` |
| macOS Intel / Apple Silicon | `xiaozhi-server-go-<version>-darwin-*.tar.gz` | Extract and run `./start.sh` |

If `.config.yaml` is not present on first run, the server initializes this file from `config.yaml` and exits. Fill in your ASR, LLM, TTS, and `web.websocket` settings, then run the startup script again. Once started, open:

```text
http://127.0.0.1:8080/api/ota/
```

An OK response confirms the HTTP service is running. The web console is available at `http://127.0.0.1:8080`.

## Upgrading

1. Stop the running service.
2. Back up your existing runtime folder, keeping `.config.yaml`, `config.db`, `.mcp_server_settings.json`, `ota_bin/`, and `logs/`. Do not upload these files to public repositories as they contain credentials and session history.
3. Extract the new version into a **fresh directory**; do not extract directly over an existing installation.
4. Copy your `.config.yaml`, `config.db`, and optional `.mcp_server_settings.json` and `ota_bin/` into the new directory. Do not overwrite the new `web/` directory with older web assets.
5. Launch the new version and test the OTA health check, web console, and a sample device voice session.

## Rolling Back

If issues occur after upgrading:

1. Stop the new version;
2. Switch back to your backed-up runtime directory;
3. Start the service with the previous binary;
4. Confirm `http://127.0.0.1:8080/api/ota/` and voice interactions are working properly.

Preserving the old installation directory prevents database corruption and avoids manual database rollbacks.
