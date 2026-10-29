# Bot Notify

Module ID: `bot-notify`. Default: disabled for both native and legacy installs.
The admin enables it in the existing customization page, then opens the
「消息机器人」 item in the admin sidebar. Transports: Telegram (long polling)
and QQ (official bot API: REST v2 send + WebSocket gateway receive). No
webhook or user/account binding.

Announcement sync: a per-bot switch makes a watcher poll the active
announcements every 30 seconds and push newly published or content-edited
ones to the bot's push targets; the watermark is `bot_notify_bots.
announce_state`, seeded silently on the first pass so history never replays.
Announcement bodies are Markdown and are flattened to plain text
(`PlainText`, markdown.go) before anything is sent, including the
`{announcements}` template placeholder. QQ credentials are AppID +
AppSecret (encrypted JSON blob in the token field; AppID also in clear in
`config.app_id`); QQ targets are `group:<group_openid>` / `user:<user_openid>`
and proactive sends may hit QQ quotas — rejections surface in bot status.

## Ownership and wiring

- Backend domain, SQL repository, HTTP API and Telegram/QQ transports: this folder.
- Internal-service adapters and worker construction: `customize/wiring/bot_notify.go`.
- Lifecycle switch notification: `customize/lifecycle.go`; changes from other
  processes are reconciled every two seconds. Workers are canceled and joined
  before the next configuration starts. Shutdown joins workers before DB close.
- Registration: customize catalog/provider, extension handler container, existing
  custom route registrar, Wire composition/cleanup; no upstream business edits.
- Frontend: `frontend/src/extensions/modules/bot-notify` (list + per-bot
  management page), registered through the extension route/catalog and one
  admin sidebar entry.
- Migrations: `migrations/sql/bot-notify/202610170001_bot_notify.sql` (v1
  single-bot schema), `202610170002_bot_notify_multi_bot.sql` (multi-bot) and
  `202610170003_bot_notify_announce_state.sql` (announcement watermark).

## Storage

`bot_notify_bots`: one row per configured bot — admin label, platform
(`telegram`), JSONB config and that bot's persistent Telegram offset. Only the
token is encrypted, using the existing SecretEncryptor (AES-256-GCM). GET
returns `****` plus the final four characters; PUT with an empty token
preserves it. Changing bot identity resets the offset; rotating its token does
not. `getMe` validates replacements and fills username and bot ID. The v1
singleton table `bot_notify_config` is kept for history; migration 002 copies
its row into `bot_notify_bots` as the first bot.

`bot_messages`: platform/bot/update uniqueness, owning bot row (`bot_row_id`),
chat/sender/text/message IDs, nullable rule FK, reply status/content and
ingestion timestamp. All received events are stored, including unaddressed
group messages. Rule deletion preserves messages and clears their FK;
deleting a bot cascades its messages and rules. Retention is 30 days AND at
most 2000 messages, pruned hourly.

`bot_reply_rules`: owning bot (`bot_id`), keyword, contains/exact/command,
template, enable, sort and timestamps. Match order: sort then ID ascending,
first match only. Every newly created bot is seeded with the two default
entries (contains 渠道状态 and the /status command).

## Runtime

One worker per configured bot, each holding its own PostgreSQL advisory lock
(`20261017, 770000 + bot row id`), so multiple instances never consume the same
bot's offset or duplicate its broadcasts. Reconciliation runs every two
seconds: workers start, stop and restart as bots are created, edited, deleted
or the module switch changes. Disabled/unconfigured bots never poll or
broadcast; rows and rules remain intact. Broadcasts are aligned to wall-clock
boundaries: the interval counts whole multiples after local midnight, so a
60-minute interval fires exactly on the hour and a hot restart never drifts
the phase. `POST /bots/:id/push-now` broadcasts the saved template on demand.

Telegram uses net/http with 30-second long polling, a 40-second per-attempt
deadline, at most three attempts, bounded backoff and sanitized errors.
Messages are split into 2000-rune chunks. Group mentions are parsed from
Telegram UTF-16 entities, including `/status@bot`; replies check bot numeric
ID. Plain `/status` in a group requires replying to the bot. Private chats
match normally. Bot privacy mode/permissions must allow Telegram to deliver
the events the administrator expects; events Telegram does not deliver cannot
be stored.

Inbound rows are persisted before matching/sending; only completed sends set
`replied=true`. Telegram has no idempotency key: ambiguous network failures
can duplicate a send during bounded retries; a previously persisted update is
not sent again on restart. Thus replies are best-effort, not exactly-once
delivery.

Both scheduled pushes and rules call the same renderer. It fetches only
referenced data and substitutes once, leaving unknown placeholders and
inserted text literal. Channels use `ChannelMonitorService.ListUserView`, the
same source as the V1 user monitor endpoint; the heading label is the
monitor's group_name, falling back to its name when group_name is empty. Its
primary-model timeline is sorted and trimmed to ten. Announcements use
`AnnouncementService.List`, active status plus `IsActiveAt`, latest
publication time (starts_at, falling back to created_at), at most five. This
is a global bot feed, including targeted active announcements; use trusted
destination chats. Site name is read through the settings repository.

## AI customer service

Keyword rules stay authoritative; AI only answers addressed messages no rule
matched. Code: `ai*.go`, `image.go` here; the provider and tools are in
`customize/wiring/bot_ai_provider.go` and `bot_ai_tools.go`. Migration
`202610290001_bot_notify_ai_assistant.sql` adds the switches below plus
`sender_id`/`image_count` audit columns.

- Calls go to this server's own `/v1/chat/completions` on loopback with the
  admin-selected key, so billing and model access match a direct API call.
  Reasoning effort (default low), image parts and tools are optional: on a
  400/413/415/422 the provider drops them one at a time and retries; rejected
  effort/tools are remembered per model for 30 minutes.
- Knowledge: the system prompt (`ai_prompt.go`) holds fixed policy, support
  contacts from site settings, the admin-editable knowledge base
  (`ai_knowledge.go`; empty = built-in default) and, when `use_user_guide` is
  on and the user-guide module is enabled, the docs-center manual. It contains
  no live data, so unchanged settings give byte-identical prompts for upstream
  prompt caches.
- Tools (read-only, public data, exclusive groups never listed): group status,
  groups, model prices (plaza display prices × group rate; only while the plaza
  is public), subscription plans and recharge info (only with
  `allow_recharge`; campaigns follow the recharge-campaigns switch), group
  cache hit rate (only with `allow_usage_stats`; site-wide last 24 h,
  withheld below 50 requests / 200k prompt tokens, cached 10 minutes) and
  announcements.
- Images: Telegram photos/image documents (getFile) and QQ image attachments
  (QQ media hosts only, redirects re-checked, private addresses refused) are
  downloaded only when AI answers; at most 3 per answer, 5 MB each, type
  verified by magic bytes; never stored, only counted.
- Answers run on a per-bot dispatcher, off the poll loop: one batch at a time
  per conversation (private chat, or chat + sender in groups), messages within
  a few seconds are merged (screenshot, then question), the cooldown delays
  rather than drops, at most 3 model calls run concurrently. Group members get
  separate history (last 4 turns within 6 hours).
- Replies thread to the newest message: Telegram quotes it in groups, QQ sends
  a passive reply with `msg_id`/`msg_seq`. QQ only accepts passive replies for
  a few minutes, so a long cooldown can make QQ answers fail.
- Guards (`ai_guard.go`): input patterns block only attack shapes (injection,
  other people's data, fraud, imperative destructive commands), never
  everyday wording such as "怎么获取密钥". Output is flattened to plain text,
  then rejected for secrets (placeholders such as `sk-你的密钥` pass),
  infrastructure addresses, prompt canaries and links to hosts not found in
  the knowledge base, manual, contacts, tool results or a built-in vendor list.
  Every non-model reply (blocked, busy, unavailable, daily limit, unreadable
  attachment) is fixed text and audited as blocked.
- `POST /bots/:id/ai/test` runs the same pipeline for the admin console with
  the saved config; nothing is sent or stored.

## Admin API

All routes inherit existing admin auth, audit, compliance and rate limiting,
plus `customize.Require("bot-notify")`:

| Method | Path under /api/v1/admin/bot-notify |
| --- | --- |
| GET, POST | /bots |
| GET, PUT, DELETE | /bots/:id |
| GET | /bots/:id/status |
| POST | /bots/:id/test-send (chat_id, optional text) |
| POST | /bots/:id/push-now |
| GET | /bots/:id/messages (page, page_size <=100, chat_id, keyword) |
| GET, POST | /bots/:id/rules |
| PUT, DELETE | /bots/:id/rules/:ruleId |
| GET, PUT | /bots/:id/ai/config |
| GET | /bots/:id/ai/keys (the admin's own keys, metadata only) |
| GET | /bots/:id/ai/records (page, keyword, blocked) |
| POST | /bots/:id/ai/test (text, optional image data URI ≤ 5 MB) |

Frontend config receives the masked token in a label; the password input
stays empty, so editing other fields never submits the mask as a new secret.

## Verification

Run `go build ./...`, `go test ./internal/customize/...` in backend.
In frontend run `pnpm --config.verify-deps-before-run=false run typecheck` and
the bot TemplateInput/extension catalog Vitest suites. Unit tests cover
rendering, status blocks/chronology/abnormal thresholds, addressing/rules,
UTF-16 Telegram parsing, error redaction, configuration encryption and token
preservation, duplicate updates, unaddressed persistence, retention and
lifecycle. Real PostgreSQL and Telegram acceptance requires a running DB and
a valid token.
