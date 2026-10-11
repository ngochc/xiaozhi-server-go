# MCP Usage Guide

## ESP32 Client MCP
ESP32 clients support device-level MCP tools. By default, 4 built-in device tools are supported:
- `self.get_device_status`
- `self.audio_speaker.set_volume`
- `self.screen.set_brightness`
- `self.screen.set_theme`

No server-side configuration changes are required for device MCP. When an ESP32 connects, the server logs confirm tool registration. You can test voice control directly by telling the assistant to adjust the speaker volume.

## Server-Side External MCP
External MCP servers can be configured via `.mcp_server_settings.json` located in the root repository or executable directory:

```json
{
  "mcpServers": {
    "amap-maps": {
      "command": "npx",
      "args": [
        "-y",
        "@amap/amap-maps-mcp-server"
      ],
      "env": {
        "AMAP_MAPS_API_KEY": "your_api_key"
      }
    },
    "filesystem": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-filesystem",
        "/allowed/path"
      ]
    },
    "playwright": {
      "command": "npx",
      "args": ["-y", "@executeautomation/playwright-mcp-server"],
      "des": "run 'npx playwright install' first"
    },
    "windows-cli": {
      "command": "npx",
      "args": ["-y", "@simonb97/server-win-cli"]
    }
  }
}
```

Running NPX-based MCP tools requires Node.js installed on the server host.

The server currently uses Stdio transport for external MCP processes. If connecting to SSE-based MCP endpoints, you can use `mcp-proxy`:

```json
{
  "mcpServers": {
    "zapier": {
      "command": "mcp-proxy",
      "args": [
        "https://actions.zapier.com/mcp/****/sse"
      ]
    }
  }
}
```

An MCP Manager instance is allocated per active client connection upon session start and automatically freed on disconnect. Check server logs to verify MCP tools initialization.
