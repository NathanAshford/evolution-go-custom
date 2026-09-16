# Evolution GO - Changelog

## Unreleased (custom fork)

### 🆕 New Features
- **MCP can now send every message type** — the connector went from 8 tools to 20:
  `send_media_message` (image/video/audio/document), `send_link_message`,
  `send_location_message`, `send_contact_message`, `send_sticker_message`,
  `send_poll_message`, `send_buttons_message`, `send_list_message`,
  `send_carousel_message`, `send_status`, plus `place_call`/`end_call`. Each schema
  mirrors the real `/send/*` body field-for-field.
  **Button parameters are now correct and validated up front:** `reply` uses
  display_text+id, `url` uses url, `call` uses phone_number, `copy` uses copy_code and
  `pix` uses currency/name/key_type/key. The 3-reply cap, the reply-vs-CTA exclusivity
  and the pix-alone rule are checked before sending, with an error that names the
  offending button. Carousel buttons are documented as the exception they are — no
  url/phone_number fields, the value goes in `id`, and pix is unsupported.
  Argument validation runs *before* the connectivity check so a malformed payload
  reports the actual mistake instead of "instance not connected".
- **WhatsApp voice calls** — the VoIP stack from
  [WaCalls](https://github.com/JotaDev66/WaCalls) ported into `pkg/voip/` (MLow codec,
  RTP/SRTP, STUN, SCTP/WebRTC relay, `<call>` signaling), wired to our vendored whatsmeow
  and exposed as 7 new endpoints: `/call/offer`, `/accept`, `/terminate`, `/hangup`,
  `/list`, `/history`, `/status/{callId}`. Up to 5 concurrent calls per instance; further
  inbound offers are auto-rejected. Swagger regenerated (91 paths) and the API Tester picks
  them up automatically since it reads the live spec. The manager's test button gained a
  "Ligacao de voz" scenario that rings the peer and hangs up after ~6s.
  **Audio:** the call rings, connects and reports state, but the HTTP API does not stream
  microphone audio — the call runs on silence keepalive. See `docs/CALLS.md`.
  **Proxy:** signaling travels on the whatsmeow websocket, which already honours the
  instance proxy, so **calls ring normally behind an HTTP proxy**; only the SRTP media path
  needs UDP. SOCKS5 UDP ASSOCIATE (RFC 1928) is implemented in
  `pkg/voip/transport/socks5udp.go` and plugged into pion via `SettingEngine.SetNet`, so a
  SOCKS5 proxy carries audio too. A failed association falls back to direct UDP.
- **MCP tab in the manager** (below Instâncias) — shows the endpoint URL, the token-scoped
  URL, all four auth forms, per-key scope, connect instructions for Claude/Claude Code/
  ChatGPT, the live tool list and the search filters, plus a "Testar conexão" probe that
  runs a real `initialize` + `tools/list` handshake.
- **MCP server for Claude / ChatGPT** — new `POST /mcp` endpoint speaking Model Context
  Protocol over Streamable HTTP (JSON-RPC 2.0). Lets AI assistants send WhatsApp messages
  and search conversations. Tools: `list_instances`, `send_text_message`, `search_messages`,
  `get_chat_history`, `list_chats`, `fetch_message`, plus `search`/`fetch` aliases for
  ChatGPT deep research. Auth accepts an `apikey` header, `Authorization: Bearer`, a
  `/mcp/:token` path segment or `?apikey=`. An **instance token scopes the assistant to that
  instance**; the admin key requires an explicit `instance` argument. See `docs/MCP.md`.
- **Searchable message archive** — new `archived_messages` table storing the text of every
  message in both directions (incoming via the event handler, outgoing via the send path,
  including buttons). Searchable by content, date range, chat, sender, direction, group and
  type. Media bytes are never stored, only captions; content is capped at 16k chars.
  Only messages exchanged after this upgrade are searchable.
- **Reconnect instance button** in the manager, plus **bounded automatic reconnection**:
  an instance that drops is brought back up to **3 times** with 5s/15s/30s backoff. The budget
  is refilled on every successful connect, so a flapping connection keeps recovering while a
  permanently broken one stops retrying instead of looping forever (the previous code retried
  unconditionally). A logout does not trigger reconnects, and an operator-triggered reconnect
  refills the budget.
- **Reconnect proxy button** — `POST /instance/proxy/:instanceId/reconnect` rebuilds the
  socket through the already-saved proxy without changing the stored configuration.
- **Proxy password reveal (eye toggle)** in the proxy modal.

### 🐛 Bug Fixes
- **QR pairing was broken (405 "client outdated") — fixed permanently by dropping the
  vendored whatsmeow fork.** The fork was pinned at client version `2.3000.1035920091`
  while WhatsApp had moved on, and every new connection was rejected before a QR could be
  emitted. Bumping the version number alone did not help: upstream ships version bumps
  *together with* protobuf schema changes, so `2.3000.1044440921` failed on the fork yet
  worked on a stock upstream build from the same machine — which is how the cause was
  isolated.
  The `replace go.mau.fi/whatsmeow => ./whatsmeow-lib` directive and the 5.4 MB vendored
  copy are gone; whatsmeow is now an ordinary module dependency, so keeping current is a
  `go get -u` instead of a manual re-port. This is viable because every fix we had
  patched into the fork is upstream now: passkey pairing, `cstoken.go`, the tctoken LID
  keying and the NCT-salt migration (upstream's `14-nct-salt.sql` matches ours).
  The only piece that was genuinely ours — the reachout-timelock and new-chat-quota MEX
  queries — moved to `pkg/walimits`, reaching the wire through
  `DangerousInternals().SendMexIQ`. Verified: QR generates, and buttons, calls, MCP, proxy
  and the 463 diagnostics all still work.
- **WhatsApp client version was never applied to the handshake** — the code set
  `store.DeviceProps.Version` (the pairing payload) but never `store.WAVersion`, which is
  what the websocket handshake advertises. Both the `WHATSAPP_VERSION_*` env vars and the
  version fetched from WhatsApp Web were therefore ignored, and the stale hardcoded
  `2.3000.1035920091` was sent. Now applied via `store.SetWAVersion`.
  This was a real bug but not the cause of the 405 — see the entry above.
- **Saved proxy no longer disappears from the UI** — `GET /instance/all` and `/instance/info`
  were wiping the proxy field before returning it, so the form always reopened blank. Both
  are admin-only routes, so the saved values are now returned and pre-fill the modal. New
  `GET /instance/proxy/:instanceId` returns the stored configuration; rows holding the legacy
  literal `"null"` are reported as "no proxy".
- **`POST /instance/reconnect` failed exactly when it was needed** — it went through
  `ensureClientConnected`, which errors with "client disconnected" when the socket is down.
  It now calls the reconnect path directly, which tears down and restarts the client.
- **Pairing code vanished after 10 seconds** — the QR modal's auto-refresh rebuilt the
  `qrcode` object without `pairingCode`, so the code disappeared on the first refresh. It is
  now preserved across refreshes (including the QR-fetch failure path), shown grouped as
  `XXXX-XXXX` with a copy button, and the instructions switch to the phone-number flow.
  `pairInstance` also tolerates both `PairingCode` and `pairingCode` and raises a clear error
  instead of silently returning an empty code.
- **Reconnect reported success even when it failed** — a 200 from `/instance/reconnect` only
  means the reconnect was dispatched, so the manager always showed "reconectada!". It now
  polls the real connection state for up to 15s and reports an actionable error when the
  instance never comes back (dead proxy, unlinked device).

## v0.7.2 (custom fork)

### 🆕 New Features
- **Passkey (WebAuthn / Shortcake / CRSC) pairing** — ported from the official v0.7.2
  into our vendored `whatsmeow-lib` (instead of switching to upstream whatsmeow, which
  would have dropped all our custom fixes). Adds `pair-passkey.go` + `types/passkey.go`,
  the `PairPasskeyRequest/Confirmation/Error` events and notification routing. On the app
  side: an in-memory ceremony store, the PUBLIC endpoints `GET /passkey-ceremony/{token}`,
  `POST .../response`, `POST .../confirm`, wiring in the whatsmeow event handler, the
  bundled `passkey-helper` browser extension, and a manager "Abrir WhatsApp Web" stage in
  the QR modal. Public API base is configured via **`PASSKEY_PUBLIC_URL`**.
- **QR-channel keeps the socket alive during a passkey ceremony** — instead of the upstream
  ~1000-line QR-flow rewrite, `qrchan.go` no longer disconnects when QR codes run out while
  a passkey ceremony is in flight, preserving our existing QR/connect flow untouched.

### 🐛 Bug Fixes
- **`POST /instance/pair` returned an empty `PairingCode`** (#21) — now starts the instance,
  waits for the websocket before `PairPhone`, and surfaces real errors instead of HTTP 200
  with an empty code.

### 🔧 Notes
- Deliberately kept our whatsmeow fork and module path; skipped the upstream module rename,
  the whatsmeow-fork removal, and the license-core rework to avoid breaking our customizations
  (multi-webhook, 463/tctoken/cstoken fixes, reachout-timelock diagnostics + UI timer, NativeFlow).

## v0.7.1

**Docker:** `evoapicloud/evolution-go:0.7.1`

### 🆕 New Features
- **Test-send modal in Manager** — new modal in the embedded manager UI to test message sending directly from the panel, covering text, media and interactive message types. Useful for validating an instance right after pairing without leaving the manager.

### 🔧 Improvements / CI
- **whatsmeow-lib SHA now pinned in the public sync** — the `sync-releases` workflow previously re-cloned whatsmeow `main` on every run, so the SHA listed in the CHANGELOG could drift from what the public repos actually built against. The workflow now captures the SHA from the dev submodule and checks out that exact commit in the target, restoring release reproducibility.
- **Repository cleanup** — dropped tracked binaries (`evolution-go`, `build/server`), IDE config (`.idea/`) and scratch files (`DIFF-COMPLETO.txt`, `API-INTERACTIVE-DOCS.txt`, `carousel-sender.html`). Expanded `.gitignore` to prevent reincidence.

### 📝 Docs
- **Postman collection** — added `Set Proxy` request and multipart hints on `/send/media`; collection file renamed from `Evolution GO.postman_collection (2).json` to `Evolution GO.postman_collection.json`.
- **Interactive messages docs** — additional examples and corrections.

## v0.7.0

**Docker:** `evoapicloud/evolution-go:0.7.0`

### 🆕 New Features
- **Multi-platform interactive messages** — Buttons, lists and carousel working on Android, iOS and WhatsApp Web/Desktop
  - **SendButton**: removed `ViewOnceMessage` wrapper that blocked rendering on iOS and WhatsApp Web; `Footer` and `Header` are now conditional
  - **SendList**: migrated from `InteractiveMessage`/`NativeFlowMessage` to legacy `ListMessage` (native protobuf) for broad compatibility
  - **SendCarousel**: new endpoint `POST /send/carousel` with cards (image, text, footer, buttons) and automatic JPEG thumbnail generation for instant image loading
  - `whatsmeow-lib`: added `biz` node for `InteractiveMessage` and pinned `product_list` type on the `biz` node for `ListMessage`
- **Base64 media support on `/send/media`** — The `url` field on `POST /send/media` now also accepts base64-encoded media. When the value does not start with `http://` or `https://`, it is treated as base64 and decoded; reuses the existing `SendMediaFile` flow
- **WhatsApp status endpoints** — new `POST /send/status/text` and `POST /send/status/media` publish text/image/video status to `status@broadcast`. Media endpoint supports both JSON (with URL) and multipart/form-data (file upload). Thanks @Eduardo-gato (#15)
- **Webhook routing for GROUP / NEWSLETTER** — when the primary `MESSAGE` / `SEND_MESSAGE` / `READ_RECEIPT` subscription is absent, events from `@g.us` chats are forwarded to `GROUP` subscribers and events from `@newsletter` chats to `NEWSLETTER` subscribers. Thanks @oismaelash (#18)

### 🔧 Improvements
- **Proxy protocol** — new optional `protocol` field (and `PROXY_PROTOCOL` env) supporting `http`, `https`, `socks5`. Replaces the hardcoded SOCKS5 dialer with `client.SetProxyAddress`, fixing HTTP-proxy QR pairing (#12). Thanks @TBDevMaster (#13)
- **WhatsApp Web version cache** — `fetchWhatsAppWebVersion` now caches the result for 1 hour with a mutex instead of issuing one request per instance startup. Thanks @VitorS0uza (#24)
- **Manager flicker fix** — instance page no longer replaces the list with skeleton cards on every 5s polling cycle (`hasLoaded` flag). Thanks @TBDevMaster (#14), closes #11
- **`WEBHOOKFILES` → `WEBHOOK_FILES`** — `.env.example`, docker-compose and docs aligned with the env var the runtime actually reads. Thanks @VitorS0uza (#22)
- **Dependency cleanup** — removed unused `github.com/EvolutionAPI/evo-gate` from `go.mod`
- **whatsmeow-lib** bumped to `0923702fb`
- **Telemetry removed** — dropped legacy `pkg/telemetry`

### 🐛 Bug Fixes
- **`/message/edit`** — was silently ignored because the edit payload used `Conversation` while the original message was sent as `ExtendedTextMessage`. WhatsApp requires matching types; now the edit uses `ExtendedTextMessage` and the response returns the actual server timestamp instead of the zero value. Closes #16
- **Sticker upload to S3/MinIO** — when `webp.Decode` or `png.Encode` failed, the whole media pipeline aborted and the sticker was lost from the webhook. Now we log a warning and keep the raw `.webp` bytes so the sticker still reaches the bucket. Closes #5
- **Multipart `/send/media`** — the binary-upload branch silently dropped `mentionAll`, `mentionedJid` and `quoted`. These fields now parse from the form (with `mentionedJid` accepting repeated or comma-separated values) and reach the send service. Closes #2

### ⚠️ Breaking changes
- **Proxy** — previously all proxies were forced through SOCKS5. If you run SOCKS5 on a non-standard port (anything outside 1080/2080/42000-43000), set `PROXY_PROTOCOL=socks5` in the env or pass `"protocol": "socks5"` in the proxy body explicitly — otherwise the new protocol inference will fall back to HTTP.

### 📝 Docs
- **README** — updated WhatsApp support number and issue templates
- **Interactive messages guide** — new `docs/wiki/guias-api/api-interactive.md`
- **Proxy docs** — environment variables, configuration guide and API reference updated with the new `protocol` field

## v0.6.1

### 🆕 New Features
- **Group invite info endpoint** — `GET /group/invite-info` to get group details from invite link
- **Enhanced media sending** — GIF playback, video stickers, and transparent sticker support

### 🐛 Bug Fixes
- **Admin revoke** — Allow deleting messages from others in groups (admin revoke)

### 🔧 Improvements
- **Version management** — Reads version from `VERSION` file with ldflags fallback
- **CORS global middleware** — Applied before all routes
- **Makefile compatibility** — Fixed `$(shell)` syntax for GNU Make 3.81 (macOS default)
- **CI/CD cleanup** — Removed `develop` branch trigger and `homolog` tag from Docker workflow
- **README updated** — New links, documentation, and hosting info

## v0.6.0

### 🆕 New Features
- **Version from VERSION file** — Reads version from `VERSION` file at startup instead of hardcoded value

### 🔧 Improvements
- **Makefile compatibility** — Fixed `$(shell)` syntax for GNU Make 3.81 (macOS default)

## v0.5.4

### 🔧 Improvements
- **Update whatsmeow lib**

## v0.5.3

**Docker:** `evoapicloud/evolution-go:0.5.3`

### 🔧 Improvements

- **Update context handling in service methods** 
  - Refactored multiple service methods across various packages to include `context.Background()` as the first argument in client calls. This change ensures that all client interactions are properly context-aware, allowing for better cancellation and timeout management.
  - Updated methods in `call_service.go`, `community_service.go`, `group_service.go`, `message_service.go`, `newsletter_service.go`, `send_service.go`, `user_service.go`, and `whatsmeow.go` to enhance consistency and reliability in handling requests.
  - This adjustment improves the overall robustness of the API by ensuring that all client calls can leverage context for better control over execution flow and resource management.

## v0.5.2

**Docker:** `evoapicloud/evolution-go:0.5.2`

### 🆕 New Features
- **SetProxy Endpoint**: New endpoint `POST /instance/proxy/{instanceId}` to configure proxy for instances
  - Support for proxy with/without authentication
  - Validation of required fields (host, port)
  - Automatic cache update via reconnection
  - Integrated Swagger documentation

### 🔧 Improvements
- **CheckUser Fallback Logic**: Implemented intelligent fallback logic
  - If `formatJid=true` returns `IsInWhatsapp=false`, automatically retries with `formatJid=false`
  - Significant improvement in valid user detection
  - Added `RemoteJID` field to use WhatsApp-validated JID
- **LID/WhatsApp JID Swap**: Automatic handling of special cases
  - When `Sender` comes as `@lid` and `SenderAlt` comes as `@s.whatsapp.net`
  - Automatic inversion: `Sender` and `Chat` receive `@s.whatsapp.net`, `SenderAlt` receives `@lid`
  - Detailed logs for tracking swaps

### 🐛 Bug Fixes
- **SendMessage**: Standardization of WhatsApp-validated `remoteJID` usage
- **User Validation**: Improvement in phone number validation and formatting

---

## v0.5.1

**Docker:** `evoapicloud/evolution-go:0.5.1`

### 🔧 Improvements
- **Instance Deletion**: Enhance instance deletion and media storage path resolution
- **Media Storage**: Improvements in media storage and path resolution

---

## v0.5.0

**Docker:** `evoapicloud/evolution-go:0.5.0`

### 🔧 Improvements
- **Media Storage**: Enhance media storage and logging in Whatsmeow event handling
- **Retry Logic**: Implement retry logic for client connection and message sending
- **Media Handling**: Enhance media handling in event processing

---

## v0.4.9

**Docker:** `evoapicloud/evolution-go:0.4.9`

### 🔧 Improvements
- **Connection Handling**: Add instance update test scenarios and improve connection handling
- **FormatJid Field**: Update FormatJid field to pointer type for better handling in message structures
- **Dependencies**: Update dependencies and fix presence handling in Whatsmeow integration

---

## v0.4.8

**Docker:** `evoapicloud/evolution-go:0.4.8`

### 🔧 Improvements
- **Audio Duration**: Improve audio duration parsing in convertAudioToOpusWithDuration function

---

## v0.4.7

**Docker:** `evoapicloud/evolution-go:0.4.7`

### 🔧 Improvements
- **Phone Number Formatting**: Improve phone number formatting and validation in user service
- **Brazilian/Portuguese Numbers**: Update Brazilian and Portuguese number formatting in utils

### 🆕 New Features
- **Media Handling**: Enhance media handling in event processing

---

## v0.4.6

**Docker:** `evoapicloud/evolution-go:0.4.6`

### 🆕 New Features
- **User Existence Check**: Add user existence check configuration and JID validation middleware

---

## v0.4.5

**Docker:** `evoapicloud/evolution-go:0.4.5`

### 🔧 Improvements
- **Dependencies**: Update dependencies and enhance audio conversion functionality

---

## v0.4.4

**Docker:** `evoapicloud/evolution-go:0.4.4`

### 🆕 New Features
- **CLAUDE.md**: Add CLAUDE.md for project documentation and enhance RabbitMQ connection handling

---

## v0.4.3

**Docker:** `evoapicloud/evolution-go:0.4.3`

### 🔧 Improvements
- **PostgreSQL Connection**: Fix in PostgreSQL connection configuration for session auth
  - Controlled configuration of pool, idle, etc.
  - Adjustment on top of whatsmeow lib
- **User Endpoints**: Fix in 'User Info' and 'Check User' endpoints
  - Now return with contact's LID information

---

## v0.3.0

### 🆕 New Features
- **Own Message Reactions**: Additional 'fromMe' parameter using Chat id
- **CreatedAt Field**: CreatedAt field added to instances table

---

## v0.2.0

### 🆕 New Features
- **Advanced Settings**: Advanced configurations in instance creation
  - `alwaysOnline` (still to be implemented)
  - `rejectCall` - Automatically reject calls
  - `msgRejectCall` - Call rejection message
  - `readMessages` - Automatically mark messages as read
  - `ignoreGroups` - Ignore group messages
  - `ignoreStatus` - Ignore status messages
- **Advanced Settings Routes**: New routes for get and update of advanced settings
- **QR Code Control**: `QRCODE_MAX_COUNT` variable to control how many QR codes to generate before timeout
- **AMQP Events**: `AMQP_SPECIFIC_EVENTS` variable to individually select which events to receive in RabbitMQ

### 🔧 Improvements
- **Reconnect Endpoint**: Fix in reconnect endpoint
- **Sender Info**: `Sender` and `SenderAlt` no longer come with session id, only the id

### 🐛 Bug Fixes
- **QR Code Generation**: Fix to not generate QR code automatically after disconnection or logout

---

## v0.1.0

### 🆕 Initial Features
- Base implementation of Evolution API in Go
- WhatsApp integration via whatsmeow
- Instance system
- Basic message sending endpoints
- Webhook support
- RabbitMQ and NATS integration
- Authentication system
- Swagger documentation

---

## 📋 Migration Notes

### v0.5.2
- The new `SetProxy` endpoint requires admin permissions (`AuthAdmin`)
- The `CheckUser` fallback logic is automatic and transparent
- LID/WhatsApp JID handling is automatic

### v0.4.3
- Check PostgreSQL connection settings if using postgres auth

### v0.2.0
- Review advanced settings configurations if necessary
- Configure `QRCODE_MAX_COUNT` if you want to limit QR codes
- Configure `AMQP_SPECIFIC_EVENTS` for specific RabbitMQ events

---

## 🔗 Useful Links

- **Docker Hub**: `evoapicloud/evolution-go`
- **Documentation**: Swagger available at `/swagger/`
- **GitHub**: [Evolution API Go](https://github.com/EvolutionAPI/evolution-go)

---

## 🤝 Contributing

To contribute to the project:
1. Fork the repository
2. Create a branch for your feature
3. Commit your changes
4. Open a Pull Request

---

*Last updated: October 2025*

