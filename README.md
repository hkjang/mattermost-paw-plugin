# mattermost-paw-plugin

Mattermost users can talk to their personal QwenPaw agent through the `@paw` bot.

## Behavior

- Public/private channel mention: `@paw` text is sent to the user's personal QwenPaw server and the response is posted in the original thread.
- Direct message with `@paw`: the full message is treated as the prompt, and replies are threaded so the RHS thread can stay open for the next turn.
- Group DM: only messages mentioning `@paw` are handled.
- Bot, webhook, plugin, and empty messages are ignored or answered with guidance.
- Attachments are not supported yet.

## User Mapping

The plugin maps the Mattermost message author to a personal QwenPaw URL:

```text
https://{userid}.{BaseDomainSuffix}
```

The default mapping uses the Mattermost username, lowercases it, converts dots/underscores to hyphens, and validates it as a DNS label.

## QwenPaw Integration

The plugin stores a Mattermost-scoped QwenPaw `session_id` in the Mattermost Plugin KV Store, then sends each prompt to the user's personal QwenPaw server with:

```text
POST /api/console/chat
Accept: text/event-stream
X-Agent-Id: default
```

The request body follows the QwenPaw REST API format with `input`, `session_id`, `user_id`, and `channel`. SSE `data:` events are parsed for `status`, `output`, `error`, and `session_id`. Qwen-style `<think>` / `</think>` output remains visible with the final answer.

## JupyterHub Integration

Users can control their personal JupyterHub-backed QwenPaw server from Mattermost:

- `start` or `/start`: start server
- `stop` or `/stop`: stop server
- `status` or `/status`: show server status

JupyterHub API calls use `Authorization: token <token>`.

## Admin Settings

- `BotUsername`
- `BaseDomainSuffix`
- `UserIDMappingMode`
- `RequestTimeoutSec`
- `QwenPawAgentID`
- `QwenPawAuthToken`
- `QwenPawChannel`
- `EnableAsync`
- `AsyncThresholdSec`
- `JupyterHubBaseURL`
- `JupyterHubAPIToken`
- `AutoStartServer`
- `ServerStartTimeoutSec`
- `ServerStopTimeoutSec`
- `ServerStatusPollIntervalSec`
- `AllowUserStopServer`
- `AllowUserStartServer`
