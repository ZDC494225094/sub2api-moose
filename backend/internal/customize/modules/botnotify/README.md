# Bot Notify

Module ID: `bot-notify`. Default: disabled for both native and legacy installs.
The admin enables it in the existing customization page, then follows the
module's management link. No QQ transport, webhook or user/account binding.

## Ownership and wiring

- Backend domain, SQL repository, HTTP API and Telegram transport: this folder.
- Internal-service adapters and worker construction: `customize/wiring/bot_notify.go`.
- Lifecycle switch notification: `customize/lifecycle.go`; changes from other
  processes are reconciled every two seconds. Workers are canceled and joined
  before the next configuration starts. Shutdown joins workers before DB close.
- Registration: customize catalog/provider, extension handler container, existing
  custom route registrar, Wire composition/cleanup; no upstream business edits.
- Frontend: `frontend/src/extensions/modules/bot-notify`, registered through the
  extension route/catalog and existing layout slot on the customization page.
- Migration: `migrations/sql/bot-notify/202610170001_bot_notify.sql` only.

## Storage

`bot_notify_config`: singleton JSONB config and persistent Telegram offset.
Only the token is encrypted, using the existing SecretEncryptor (AES-256-GCM).
GET returns `****` plus the final four characters; PUT with an empty token
preserves it. Changing bot identity resets the offset; rotating its token does
not. `getMe` validates replacements and fills username and bot ID.

`bot_messages`: platform/bot/update uniqueness, chat/sender/text/message IDs,
nullable rule FK, reply status/content and ingestion timestamp. All received
events are stored, including unaddressed group messages. Rule deletion preserves
messages and clears their FK. Retention is 30 days AND at most 2000 messages,
checked after each received batch and hourly while workers run.

`bot_reply_rules`: keyword, contains/exact/command, template, enable, sort and
timestamps. Match order: sort then ID ascending, first match only. Two default
entries implement the requested contains keyword and command using one match
type per row.

## Runtime

Telegram uses net/http with 30-second long polling, a 40-second per-attempt
deadline, at most three attempts, bounded backoff and sanitized errors.
Messages are split into 2000-rune chunks. Group mentions are parsed from
Telegram UTF-16 entities, including `/status@bot`; replies check bot numeric ID.
Plain `/status` in a group requires replying to the bot. Private chats match
normally. Bot privacy mode/permissions must allow Telegram to deliver the events
the administrator expects; events Telegram does not deliver cannot be stored.

A pinned PostgreSQL advisory lock elects one active worker across instances.
It consumes one DB connection while active, in addition to normal DB operations.
Lock health is checked during reconciliation. Disabled/unconfigured modules
never poll or broadcast; rows and rules remain intact. An admitted send can
finish while a switch changes. The first broadcast occurs after the configured
interval, and hot restart resets that interval.

Inbound rows are persisted before matching/sending; only completed sends set
`replied=true`. Telegram has no idempotency key: ambiguous network failures can
duplicate a send during bounded retries; a previously persisted update is not
sent again on restart. Thus replies are best-effort, not exactly-once delivery.

Both scheduled pushes and rules call the same renderer. It fetches only referenced
data and substitutes once, leaving unknown placeholders and inserted text literal.
Channels use `ChannelMonitorService.ListUserView`, the same source as the V1 user
monitor endpoint; its primary-model timeline is sorted and trimmed to ten.
Announcements use `AnnouncementService.List`, active status plus `IsActiveAt`,
latest publication time (starts_at, falling back to created_at), at most five.
This is a global bot feed, including targeted active announcements; use trusted
destination chats. Site name is read through the settings repository.

## Admin API

All routes inherit existing admin auth, audit, compliance and rate limiting,
plus `customize.Require("bot-notify")`:

| Method | Path under /api/v1/admin/bot-notify |
| --- | --- |
| GET, PUT | /config |
| GET | /status |
| POST | /test-send (chat_id, optional text) |
| GET | /messages (page, page_size <=100, chat_id, keyword) |
| GET, POST | /rules |
| PUT, DELETE | /rules/:id |

Frontend config receives masked token in a label; the password input stays
empty, so editing other fields never submits the mask as a new secret.

## Verification

Run `go build ./...`, `go test ./internal/customize/...` in backend.
In frontend run `pnpm --config.verify-deps-before-run=false run typecheck` and
the bot TemplateInput/extension catalog/slot Vitest suites. Unit tests cover
rendering, status blocks/chronology/abnormal thresholds, addressing/rules,
UTF-16 Telegram parsing, error redaction, configuration encryption and token
preservation, duplicate updates, unaddressed persistence, retention and lifecycle.
Real PostgreSQL and Telegram acceptance requires a running DB and a valid token.
