# Xiaozhi Voice Assistant Server (Go)

`xiaozhi-server-go` is a voice assistant server built for personal and home use. It communicates with compatible Xiaozhi clients over WebSocket, bridging speech recognition (ASR), large language models (LLM), and speech synthesis (TTS) into locally deployable voice chat sessions.

> Want to quickly get a device running? Check out the [Quick Start Guide](docs/quick-start.md).

<p align="center">
  <img src="https://github.com/user-attachments/assets/aa1e2f26-92d3-4d16-a74a-68232f34cca3" alt="Xiaozhi Server Architecture Diagram" width="600">
</p>

This project is compatible with the Xiaozhi WebSocket protocol and supports ESP32, Android, Python, and other compatible clients.

## Implemented Features

| Category | Features |
| --- | --- |
| Device Connectivity | WebSocket connection; PCM / Opus audio encoding/decoding; compatible with Xiaozhi protocol clients |
| Voice Conversations | Auto, Manual, and Real-time dialogue modes; voice interruption support |
| Model Integrations | ASR, LLM, TTS, and Vision models selectable via `config.yaml`; built-in support for OpenAI-compatible APIs, Ollama, Volcengine Doubao, Coze, Edge TTS, Deepgram, GoSherpa, iFlytek, StepFun, etc. |
| Vision Capabilities | Camera snapshot image recognition triggered via voice commands |
| Personas & Voices | Preset personas, preset voice profiles, and voice-command role switching |
| Short-term Memory | Restores short-term context upon reconnection for bound devices; isolated per device and agent, with clearing supported in the management console |
| Extensibility | Local MCP, Device MCP, and external Stdio MCP tools; support for weather, maps, etc. |
| Device Onboarding | OTA endpoint, firmware download endpoint, and WebSocket URL distribution |
| Operations & Data | Local SQLite configuration storage, logging, Swagger API documentation, Docker deployment |

The features above reflect the current codebase and configuration template. Upcoming features are tracked via [Issues](https://github.com/AnimeAIChat/xiaozhi-server-go/issues).

## 5-Minute Quick Start

See [docs/quick-start.md](docs/quick-start.md) for full instructions. Here is the shortest path:

1. Download the prebuilt server binary matching your operating system from [Releases](https://github.com/AnimeAIChat/xiaozhi-server-go/releases) and place `config.yaml` in the same directory.
2. Copy the template to a private configuration file and fill in at least one working ASR, LLM, and TTS configuration:

   ```powershell
   Copy-Item config.yaml .config.yaml
   ```

   ```bash
   cp config.yaml .config.yaml
   ```

3. Set `web.websocket` to an address accessible by your device, such as `ws://192.168.1.10:8000` (do not use `localhost` for client devices).
4. Run the executable and open `http://127.0.0.1:8080` in your browser. For local network devices, use `http://<server-lan-ip>:8080/api/ota/` as the OTA address.
5. Connect your device to the network and initiate a voice conversation. The server logs will display connection, recognition, model response, and audio playback events.

If `.config.yaml` is missing, the server will generate a copy from `config.yaml` on first launch and exit. Simply edit the model credentials and restart. Once started successfully, configuration is stored in `config.db`. Subsequent edits can be performed via the web console or by recreating the directory after backing up. See [Configuration & Troubleshooting](docs/quick-start.md#configuration--troubleshooting) for details.

## Configuration Highlights

Configuration template: [`config.yaml`](config.yaml). It is recommended to retain existing fields and only edit values.

```yaml
server:
  port: 8000

web:
  port: 8080
  # Address for local network devices; replace with your server LAN IP or domain
  websocket: ws://192.168.1.10:8000

selected_module:
  ASR: DoubaoASR
  TTS: EdgeTTS
  LLM: OllamaLLM
  VLLLM: ChatGLMVLLM
```

- Names in `selected_module` must match provider keys defined in `ASR`, `TTS`, `LLM`, and `VLLLM` sections.
- Credentials are stored only in your local `.config.yaml` or SQLite database. Never commit `.config.yaml`, `config.db`, logs, or MCP secrets to git.
- When client devices and server are on different machines, allow TCP `8000` and `8080` through your system firewall.
- WebSocket is the default client connection transport; ensure `transport.websocket.enabled` remains `true`.

### Using DSH Dialogue Bridge

Start the `@xuanfeng/dsh-xiaozhi-bridge` service first, then add and select the following provider in your `.config.yaml` (or via the Admin UI):

```yaml
selected_module:
  LLM: DSHBridge

LLM:
  DSHBridge:
    type: dsh
    # DSH bridge WebSocket address (not HTTP Harness address)
    url: ws://127.0.0.1:17980/xiaozhi
    # Must match bridge service XIAOZHI_BRIDGE_TOKEN
    api_key: your_bridge_token
```

The provider uses Xiaozhi's `session_id` as the bridge `device_id` and forwards the latest user turn. DSH handles conversation history, Harness tools, and text streaming. The stream's `assistant_delta` and fallback responses feed directly into TTS synthesis.

## Running from Source

Running from source is ideal for development and debugging. Requires Go `1.24.2` or later and a working C compiler for the SQLite driver. Opus audio processing is implemented in pure Go and requires no separate system Opus library.

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

If Windows reports a missing C compiler, install MSYS2 UCRT64 or MINGW64 toolchain, ensure `gcc` is in your `PATH`, and retry. Do not commit or share config files containing real secrets.

Build the binary:

```bash
go build -o xiaozhi-server ./src/main.go
```

## Docker Deployment

A [`docker-compose.yml`](docker-compose.yml) is included for running prebuilt Linux releases. Place the following files in the same directory and set `command` in `docker-compose.yml` to match your binary name:

- Linux server binary
- `.config.yaml`
- `docker-compose.yml`

Then run:

```bash
docker compose up -d
docker compose logs -f
```

Ensure ports `8000` and `8080` are forwarded to the host, and set `web.websocket` in `.config.yaml` to the host IP or domain reachable by devices.

## MCP Configuration

For MCP tool usage, external Stdio MCP examples, and device MCP details, see [src/core/mcp/README.md](src/core/mcp/README.md). External MCP configurations are stored in `.mcp_server_settings.json`, which should also remain private.

## Endpoints & Troubleshooting

- Web Console: `http://127.0.0.1:8080`
- Swagger API Docs: `http://127.0.0.1:8080/swagger/index.html`
- OTA Health Check: `http://127.0.0.1:8080/api/ota/`
- Runtime Logs: `logs/server.log`
- Device setup & FAQ: [docs/quick-start.md](docs/quick-start.md)
- Upgrades & releases: [docs/release.md](docs/release.md)
- Short-term memory architecture: [docs/short-term-memory.md](docs/short-term-memory.md)

## Commercial Edition

The commercial edition provides expanded capabilities on top of the same protocol and models, including multi-tenant device management, knowledge bases, voice cloning, customized voices, workflow orchestration, and distributed high-availability deployment.

## Community & Feedback

Issues, pull requests, and feature suggestions are welcome on GitHub!

## License

This repository is licensed under the `Xiaozhi-server-go Open Source License` (based on an enhanced Apache 2.0).
