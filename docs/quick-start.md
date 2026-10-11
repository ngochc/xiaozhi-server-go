# Quick Start: Launching and Connecting a Device

This guide walks you through setting up a compatible Xiaozhi protocol device on your local network to verify a full voice interaction cycle: speaking -> recognition -> response -> audio playback.

## Prerequisites

Prepare the following:

- A Windows, Linux, or macOS computer;
- A Xiaozhi protocol-compatible device on the same reachable network as your computer;
- At least one working set of ASR, LLM, and TTS service credentials;
- Your computer's local network IP address (e.g. `192.168.1.10`).

Client devices connect via WebSocket. Addresses like `127.0.0.1` and `localhost` are only usable on the host machine itself and cannot be used by external client devices.

## Method 1: Running Prebuilt Releases

1. Download the release binary matching your operating system from [Releases](https://github.com/AnimeAIChat/xiaozhi-server-go/releases) and extract it to a folder.
2. Copy [`config.yaml`](../config.yaml) into that directory as `.config.yaml`.
3. Open `.config.yaml` in an editor and configure the minimal settings:

   ```yaml
   web:
     websocket: ws://192.168.1.10:8000

   selected_module:
     ASR: DoubaoASR
     TTS: EdgeTTS
     LLM: OllamaLLM
   ```

   - The IP or domain in `web.websocket` must be reachable by your device.
   - Names in `selected_module` must match configured providers with valid credentials.
   - Replace placeholder values such as `your_api_key` and `your_access_token` with real credentials.

4. Launch the binary. On Windows, execute the `.exe` directly. On Linux/macOS, grant execute permissions and run:

   ```bash
   chmod +x ./linux-amd64-server-upx
   ./linux-amd64-server-upx
   ```

5. Open `http://127.0.0.1:8080/api/ota/` on the server computer. If it returns an OK status and the WebSocket URL, the HTTP service is running properly.

If `.config.yaml` does not exist on first launch, the server creates it from `config.yaml` and exits. Simply populate your credentials in `.config.yaml` and restart. On the first normal start, the server will display a temporary password for the local `admin` user in the logs. Log in to the web console and change it immediately.

## Method 2: Running from Source

```bash
git clone https://github.com/AnimeAIChat/xiaozhi-server-go.git
cd xiaozhi-server-go
cp config.yaml .config.yaml
go run ./src/main.go
```

Windows PowerShell:

```powershell
git clone https://github.com/AnimeAIChat/xiaozhi-server-go.git
Set-Location xiaozhi-server-go
Copy-Item config.yaml .config.yaml
go run ./src/main.go
```

The project requires Go `1.24.2` or later and a C compiler for SQLite. On Windows, install MSYS2 UCRT64/MINGW64 and ensure `gcc` is in your `PATH`. Opus is implemented in pure Go and requires no external libraries.

## Device Configuration

1. In your device's firmware settings, set the OTA URL to:

   ```text
   http://192.168.1.10:8080/api/ota/
   ```

2. When the device calls the OTA endpoint, it receives the WebSocket address configured in `web.websocket`.
3. Ensure firewall rules allow incoming TCP traffic on ports `8000` and `8080`.
4. Power on the device and monitor `logs/server.log` for incoming connection events.
5. Speak a short phrase to the device. A successful conversation logs client connection, ASR recognition, LLM response, and TTS playback in order.

## Verification Endpoints

| Purpose | Address |
| --- | --- |
| Web Console | `http://127.0.0.1:8080` |
| Swagger API | `http://127.0.0.1:8080/swagger/index.html` |
| OTA Health Check | `http://127.0.0.1:8080/api/ota/` |
| Device OTA Endpoint | `http://<server-lan-ip>:8080/api/ota/` |

## Configuration & Troubleshooting

### Changes to `.config.yaml` do not take effect

On first launch, configuration is persisted into `config.db`. Subsequent boots read configuration from this SQLite database. Once initialized, changes should be made via the Web Admin Console. If you want a fresh start, back up your data and remove `config.db`.

### Device cannot connect to the server

Verify the following:

1. `web.websocket` is set to the server's LAN IP or reachable domain, not `localhost`;
2. The device and server can ping/reach each other on the local network;
3. Ports `8000` and `8080` are open in firewalls, routers, or container mappings;
4. `transport.websocket.enabled` is set to `true`;
5. `http://<server-ip>:8080/api/ota/` is accessible from another device on the network.

### Device connects, but no voice reply

1. Verify provider names in `selected_module` match valid entries under `ASR`, `LLM`, and `TTS`;
2. Confirm API keys, URLs, and model names are active and have sufficient balance/quota;
3. Check `logs/server.log` for error traces;
4. Test with a short phrase and isolate issues by testing ASR, LLM, and TTS independently.

### Sharing configuration and logs

Do not publicly share `.config.yaml`, `config.db`, `.mcp_server_settings.json`, or raw server logs as they contain API secrets, tokens, and device identifiers. Redact sensitive values before posting to GitHub Issues.

## Next Steps

- Configure local, device, or external MCP tools: [MCP Guide](../src/core/mcp/README.md)
- Explore OTA endpoint details: [OTA Module Guide](../src/httpsvr/ota/README.md)
- Submit issues or requests: [GitHub Issues](https://github.com/AnimeAIChat/xiaozhi-server-go/issues)
