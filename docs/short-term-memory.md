# Short-Term Memory

The server maintains lightweight short-term dialogue context for bound devices, allowing devices to resume conversations seamlessly across WebSocket reconnects.

## How It Works

- Memory is isolated by key: `Device ID + Agent ID`. Different devices and agents never share conversation state.
- Stores up to 12 non-system messages and approximately 3000 tokens based on local heuristics. When limits are exceeded, oldest messages are trimmed first.
- System prompts are never written to short-term memory. Memory resides in RAM and resets when the server process restarts.
- Context expires 30 minutes after device disconnect and is cleaned up on subsequent accesses.

## Clearing Memory

In the Web Console, go to **Agents -> Manage Devices**, and click **Clear Short-term Memory** on the device card. This clears the memory buffer for that specific device and agent without altering configuration, bindings, or persisted dialog history.
