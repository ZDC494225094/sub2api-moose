# Bot Notify

Module ID: `bot-notify`. Default: disabled for both native and legacy installs.
The admin enables it in the existing customization page, then opens the
「消息机器人」 item in the admin sidebar. No QQ transport (yet), webhook or
user/account binding.

## Ownership and wiring

- Backend domain, SQL repository, HTTP API and Telegram transport: this folder.
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
  single-bot schema) and `202610170002_bot_notify_multi_bot.sql` (multi-bot).

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
broadcast; rows and rules remain intact. The first broadcast occurs after the
configured interval, and hot restart resets that interval.

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

## Admin API

All routes inherit existing admin auth, audit, compliance and rate limiting,
plus `customize.Require("bot-notify")`:

| Method | Path under /api/v1/admin/bot-notify |
| --- | --- |
| GET, POST | /bots |
| GET, PUT, DELETE | /bots/:id |
| GET | /bots/:id/status |
| POST | /bots/:id/test-send (chat_id, optional text) |
| GET | /bots/:id/messages (page, page_size <=100, chat_id, keyword) |
| GET, POST | /bots/:id/rules |
| PUT, DELETE | /bots/:id/rules/:ruleId |

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
