# Evolution GO - Changelog

## Unreleased (fork lucasgiovannibr/evolution-go)

Fixes and hardening on top of upstream v0.7.2. Full triage of the upstream issues
and pull requests in `FORK-TRIAGE.md`.

### Upgrade notes
- **whatsmeow updated** (30/06 → 29/09/2026, 72 commits) and **Go 1.26** is now
  required (Dockerfile updated). The whatsmeow schema moves from **v14 to v16**;
  the migrations are forward-only, so **back up `evogo_auth` before deploying** —
  the previous image cannot run on the upgraded database.
- Docker images are published to `ghcr.io/<owner>/<repo>` by the fork's workflow.
- **`poll_votes` migration** (idempotent, runs at startup): the unique constraint
  `(poll_message_id, voter_jid)` is replaced by a unique index that includes
  `instance_id`. Going back to an older image on that database breaks saving poll
  votes (its `ON CONFLICT` target no longer exists); everything else keeps working.
- New optional environment variables: `DISAPPEARING_AUTO_APPLY` (default on),
  `WEBHOOK_QUEUE_MAX_EVENTS` (1000), `WEBHOOK_QUEUE_MAX_MB` (64) and
  `WEBHOOK_QUEUE_WORKERS` (4); for calls, `CALL_MAX_CONCURRENT` (4), `CALL_RING_TIMEOUT`
  (90 s), `CALL_STREAM_GRACE` (10 s), `CALL_DIAL_LIMIT` (6 per minute) and
  `CALL_STREAM_ORIGINS`.
- **New database column** `instances.calls_enabled` (boolean, default false), added by
  the automatic migration at startup.

### Fixes
- **Process crashes**: shared instance maps are now synchronized (`fatal error:
  concurrent map writes`); WebSocket writes are serialized per connection;
  `events.Archive` no longer panics; duplicate concurrent reconnects are ignored;
  panics in service-owned goroutines are recovered and logged.
- **Postgres connection leak** (`too many clients already`): one shared whatsmeow
  store container on top of the bounded auth pool instead of a new pool per
  reconnect.
- **Lifecycle**: supervisor goroutines end when their client is replaced (one was
  leaked per reconnect); KeepAlive timeouts trigger a reconnect; instances caught
  mid-reconnect are restored on `CONNECT_ON_STARTUP`; the resolved WhatsApp version
  is applied to the handshake.
- **Security**: `/instance/{id}/advanced-settings` is scoped to the global key or
  the instance's own token; `ForceUpdateJid` uses a bound SQL parameter; global key
  comparison is constant-time and rejected WebSocket tokens are no longer logged.
- **Config**: `/instance/connect` and advanced-settings updates are partial (no
  more silent reset of events/RabbitMQ/flags); `Disconnect` keeps the event
  subscriptions.
- **Messages**: incoming edits and poll votes are decrypted before the LID/PN
  swap; `/group/participant` validation; mentionAll with documents; carousel
  button parameters; image `Width/Height`; animated stickers; avatar, edit and
  revoke with canonical JIDs; `/user/profileName`; history-sync request sent as a
  peer message; unpaired instances fail fast on send.
- **Events**: `Passkey*` follow the `QRCODE` subscription; `PICTURE`, `USER_ABOUT`
  and `BUTTON_CLICK` are published to NATS/AMQP; `KeepAliveTimeout/Restored`
  events under `CONNECTION`.

### Found in live testing (29/09/2026)
- **One runtime per instance**: `POST /instance/connect` followed by `GET /instance/qr`
  started the instance twice; the unpaired duplicate forced a logout and restarted the
  instance as a new device right after a successful pairing.
- **Deleted instances no longer restart** (a closed kill channel now means "stop for
  good"; a restart is skipped when the instance row is gone).
- `/user/info` timed out because of the `+` in the JID (now canonical, 0.3 s).
- `/group/participant` answered "success" when nothing was added; it now returns the
  per-participant result (`data`, `failed`).
- Poll votes from `@lid`-only voters store the real phone number; `viewOnce` and
  `quoted.text` also work in multipart `/send/media`.
- `POST /user/contacts`: optional `saveOnPrimaryAddressbook` (contacts cannot be
  removed through the API).

### Operational events from whatsmeow
- `ReachoutTimelock`, `StreamError` and `ClientOutdated` are now published under the
  `CONNECTION` subscription and shown in `GET /instance/{id}/runtime`.
- The bare "server returned error 463" of a send now explains the reachout restriction
  (and until when, if WhatsApp said so); the original error stays in the text.
- On `ClientOutdated` (405) the cached WhatsApp Web version is dropped so the next
  reconnection fetches the current one instead of retrying the refused version.

### Diagnostics
- `GET /health` (readiness: databases with a 2s bound, `slow`/`error` states, 503 when a
  database is down) next to the unchanged `GET /server/ok` (liveness).
- `GET /instance/{id}/runtime` and `GET /instance/runtimes`: what the process actually
  runs per instance vs the database, with coded warnings, plus process stats
  (goroutines, memory). `ENABLE_PPROF=true` exposes `/debug/pprof` behind the global key.
- The logger of a deleted instance is released (its log file descriptor was kept open
  for the life of the process); `qrcodeCount` is atomic.
- `docs/WHATSMEOW-CAPABILITIES.md`: what whatsmeow delivers, what the project uses
  (76 of 136 client methods, 70 of 75 event types, after the work listed below; the other five are never delivered on their own) and the
  hard limits of the library.

### Additions (small)
- Endpoints (details in `docs/wiki/guias-api/api-fork-additions.md`): `viewOnce` in
  `/send/media`; `POST /send/pollVote`; `POST /message/subscribe` (contact
  presence); `POST /user/lid`; `POST /user/contacts`; `PictureURL` in
  `/user/info`; `POST /group/requests` and `/group/requests/update`;
  `GET /instance/proxy/{id}` (runtime proxy status, no credentials) and
  `PROXY_FAIL_CLOSED`.
- `passkey-helper` 1.1.0: WebAuthn runs in the page's MAIN world so password
  managers (1Password, Bitwarden) work; requires Chrome/Edge 111+.
- `quoted.text` (optional) fills the quote card of replies.
- `/instance/qr` keeps returning the QR alongside the passkey fields.
- `REREQUEST_FROM_PHONE` (opt-in) re-requests undecryptable messages.
- CI (build, vet, `test -race`) and a Postgres integration test for the pool fix
  (`EVOGO_TEST_POSTGRES_DSN`).

### Pairing and chat-state events
- `PairError`, `QRScannedWithoutMultidevice` (under `QRCODE`) and `CATRefreshError`
  (under `CONNECTION`) are published, so a failed pairing is no longer silent.
- `Mute`, `Pin`, `Star`, `MarkChatAsRead`, `ClearChat`, `DeleteChat`, `DeleteForMe`,
  `UnarchiveChatsSetting` and `UserStatusMute` (changes made on another device) are
  published under `CHAT_PRESENCE`; the full sync after pairing is not.

### Disappearing messages (#79), group invites and channels
- Messages sent to a chat with disappearing messages now carry the chat's timer
  (learned from received messages, the `EPHEMERAL_SETTING` protocol message and group
  info; groups are re-read after a restart). `DISAPPEARING_AUTO_APPLY=false` turns it
  off. New `POST /chat/disappearing` and `POST /user/defaultDisappearing`.
- `POST /group/inviteinfo` (by link/code or invite card, without joining) and
  `POST /group/joininvite` (from an invite card).
- Channels: `POST /newsletter/{follow,unfollow,mute,markviewed,react}`.

### Messages that did not arrive
- The `UndecryptableMessage` event is published under `MESSAGE` (it used to be only a
  log line), and `POST /message/rerequest` asks the phone for another copy of it.

### Chat routes, presence and batch subscribe
- `/chat/pin|unpin|archive|unarchive|mute|unmute` (marked "not working" in the router)
  wrote the app-state patch under `+<number>@s.whatsapp.net`, a chat that does not
  exist, and the phone keys one-to-one chats by LID. The JID is now canonical and
  resolved to the LID; groups were never affected. Bad input is a 400 (was 500), the
  returned timestamp is real (was a zero time) and `/chat/mute` accepts an optional
  `duration` (`8h`, `1w`, `always`, `30m`; the default stays 1 hour). Chat and message
  labels had the same bug and now use the same helper.
- `POST /message/subscribe` accepts a list of numbers (up to 100) and reports each one
  (`data` / `failed`); a single string answers as before.
- Switching `alwaysOnline` at runtime takes effect immediately (presence mark and
  scheduler) instead of at the next reconnect; a guard prevents a second scheduler.

### Fixes from code-review rounds
- `POST /user/block|unblock` timed out because of the `+` in the JID.
- `GET /group/myall` always answered empty (the owner is a LID and was compared with a
  mangled own JID).
- `POST /community/add|remove` built the success/failed lists wrongly and sent
  unparseable groups as zero JIDs.
- A label deleted on the phone stayed in the local table and in `GET /label/list`.
- Outbound HTTP had no timeout (media URLs, link previews, the WhatsApp Web version
  lookup that holds a lock every instance start goes through, webhooks); everything now
  goes through clients with dial, header and total timeouts.
- An instance without proxy JSON made `/instance/connect` and the client start fail when
  a global proxy is configured in the environment.
- `/send/link`: a page that cannot be read, a relative `og:image` or a missing image no
  longer fail the send; values sent by the caller win over the scraped ones; `url` is
  honoured; the title is `og:title` or the first `<title>`; trailing punctuation is not
  part of the link.
- Media fetched from a URL rejects non-2xx answers (a 404 page was sent as the file) and
  is size-limited (100 MB media, 20 MB images, 2 MB link thumbnails); carousel and button
  header media that cannot be fetched are logged instead of vanishing silently.
- ffmpeg audio conversion times out after 3 minutes.
- `/send/poll`: 2 to 12 options, no empty or repeated option, `maxAnswer` within the
  options (a larger value was silently turned into "unlimited"). `/send/location`:
  latitude/longitude 0 are valid (only the pair 0,0 counts as missing) and ranges are
  checked.
- `SendMessage` looks the client up once, so an instance stopped mid-send no longer leaves
  a nil to dereference.
- Deleting an instance purges its stored device (session/identity/sender keys, cached
  contacts, LID map) and its poll votes; a paired instance that was not connected used to
  keep all of it in the auth database. The phone still lists the session as a linked
  device in that case.
- Poll votes are unique per instance: two instances in one group both receive a poll and
  the second used to overwrite the first's row.
- `POST /user/check` returns the error when the WhatsApp query fails (it answered
  `200` with `data: null`): 400 for an invalid number, 429/504 for rate limits and
  timeouts. `POST /message/delete` returns the real timestamp. `GET /user/contacts` is
  `[]` when empty and in a stable order.

### Remaining whatsmeow events
- `Blocklist`, `PrivacySettings`, `BusinessName` (under `CONTACT`); `CallPreAccept`,
  `CallTransport`, `CallReject`, `UnknownCallEvent` (`CALL`); `MediaRetry` (`MESSAGE`);
  `NewsletterLiveUpdate`, `NewsletterMuteChange` (`NEWSLETTER`) and `OfflineSyncPreview`
  (`CONNECTION`) are published instead of falling into the "Unhandled event" log line.
- `RotateADVSecret` no longer writes the session's old and new ADV secret into the
  instance log (the generic log line printed the whole event); it is handled without
  being published.

### Waiting for the connection, and three lookups
- Every service started an instance and then slept a fixed 2 s before checking that it
  had connected (3 s after a reconnect, 3 s + 2 s in `GET /instance/qr`, 2 s in
  `ForceReconnect`): a connection that took 2.1 s failed the request and one that took
  0.3 s still cost 2 s. They now wait for the connection itself (10 s upper bound), an
  unpaired instance still fails at once, and `GET /instance/qr` waits until a QR code,
  a passkey ceremony or a login exists (0.35 s instead of a fixed 3 s in the live test).
- `POST /user/devices` (linked devices of one or several users, device 0 is the phone),
  `GET /user/statusprivacy` (who sees my status) and `POST /user/business` (public profile
  of a Business account; 404 for an ordinary number), all bounded to 10 s.

### Webhook delivery queue
- Webhooks go through one bounded queue per destination URL instead of one goroutine per
  event: limits in events and bytes, at most a few workers, the OLDEST event is dropped
  (and counted) when full, retries back off 1 s / 5 s / 30 s / 2 min with jitter, and a
  destination whose event exhausted its retries gets a single attempt per event until one
  succeeds. `WEBHOOK_QUEUE_WORKERS=1` keeps strict order. The same URL as the global
  webhook is no longer delivered twice. `GET /instance/runtimes` reports the queues
  (`webhook`: pending, in flight, degraded destinations, sent, failed, dropped).
- `POST /instance/connect`: an empty `webhookUrl` still means "unchanged" (the bundled
  manager sends `""` on every reconnect); `"disabled"` or `"false"` now clears the
  webhook (stored empty; a legacy stored `"disabled"` is still ignored on delivery).

### `PUT /instance/{id}/integrations`
- Stores the webhook URL, the subscribed events and the RabbitMQ/WebSocket/NATS switches
  of an instance **without starting it** (`POST /instance/connect` did this but connected
  too). Takes effect at once when the instance runs, otherwise on the next connection.
  Empty fields keep their value, `webhookUrl: "disabled"` removes the webhook,
  `subscribe: ["ALL"]` selects every event.

### Manager panel rebuilt
- `/manager` was rebuilt from scratch (React 19, TypeScript, Vite, Tailwind v4; light and
  dark theme). The source is in `manager/`; the built `manager/dist` is versioned and
  what the Docker image ships (see `manager/README.md`).

### WhatsApp calls: answer, dial, video and a stream over WebSocket (experimental)
Until now the only call feature was `POST /call/reject`. Opt in per instance with
`callsEnabled` (`PUT /instance/{id}/advanced-settings`, **effective on the next
connection**). The media side uses the `purpshell/meowcaller` library, pinned to commit
`6d9b7b2c1807` (its `main` moved to a different whatsmeow fork).
- **Routes**: `GET /call/active`, `GET /call/{callId}`, `POST /call/answer`,
  `POST /call/dial`, `POST /call/hangup`, `POST /call/stream-ticket`,
  `GET /call/stream/{callId}` (WebSocket) and `POST /call/video`; `POST /call/reject`
  and `rejectCall` now go through the engine when it is on.
- **Stream**: audio is PCM 16-bit, 16 kHz mono in base64 JSON messages (Twilio Media
  Streams style); with `video: true` in the ticket, video is H.264 Annex-B, one access
  unit per message, with `keyframe_request` and `video_state` messages. Authentication is
  a one-time ticket (30 s) instead of the API key in the URL; browsers must come from an
  origin in `CALL_STREAM_ORIGINS`.
- **Video**: `start` asks the peer to turn an audio call into video (an iPhone accepts),
  `accept`, `stop`, `enable`/`disable` (mute and unmute), `orientation`. `enable` is refused
  with `409` on a call that never had video: the iPhone ignores "camera on" without the
  upgrade request. The rotation of each received picture is delivered as the clockwise
  quarter turns that make it upright (the library documents the RTP value as clockwise, but
  it counts counter-clockwise); `video_state.orientation` is the device's and must not be
  used to rotate.
- **Events** (`CALL`): `CallReady`, `CallEnded` (`reason`: `peer_hangup` when the other
  side hangs up, because WhatsApp sends no reason then; `hangup`, `rejected`,
  `rejected_busy`, `ring_timeout`, `stream_closed`, `server:<code>`) and `CallVideoState`
  (`state` names what the peer signalled: `enabled`, `disabled`, `stopped`,
  `upgrade_request`, `upgrade_accepted`, `upgrade_rejected`, `upgrade_cancelled`, `unknown`,
  plus the raw `stateCode`; the iPhone sends an unnamed code 2 right after pickup).
- **Safeguards**: instances with a **proxy** cannot use calls (the library opens a UDP
  socket that ignores the proxy, which would leak the IP): `state: "blocked_by_proxy"` and
  the `calls_blocked_by_proxy` warning; at most `CALL_MAX_CONCURRENT` calls per instance;
  `CALL_DIAL_LIMIT` calls placed per minute (failures count); a running call whose stream
  stays away for `CALL_STREAM_GRACE` is hung up; everything an operator can see is in
  `GET /instance/runtimes` (`runtime.calls` and warnings).
- **Side effect**: with the engine on, every incoming call is pre-accepted by the library.
- **Not done on purpose**: group calls, a maximum duration for an answered call, and
  recovering a stream that dropped after the grace period.
- **Tested live** with a real number and an iPhone: incoming and outgoing audio,
  incoming and outgoing video, audio-to-video upgrade in both directions, the phone turned
  through several positions, and a portrait (360x640) picture filling the phone's screen
  (a landscape one is letterboxed). `docker/fork-test/call-stream-test.py` repeats it.
- Guide, WebSocket protocol and examples: `docs/wiki/guias-api/api-call.md`.

### Documentation
- `docs/swagger.*` regenerated with swag v1.16.3 (`--parseDependency`; it had not been
  regenerated since the 0.7.2 sync): 28 routes added (calls, `/instance/{id}/integrations`,
  diagnostics, `/send/pollVote`, newsletters, etc.), none removed. The `/license/*` routes,
  registered with inline handlers that swag cannot annotate, are declared in
  `pkg/core/license_swagger.go` so they stay documented.
- A test that read `CallEnded` right after the call's `Done` channel closed could run
  before the event was published (and panic on the empty log); it now waits for the event.

## v0.7.2

**Docker:** `evoapicloud/evolution-go:0.7.2`

### 🆕 New Features
- **Passkey (WebAuthn) pairing** — support for linking accounts that the WhatsApp
  server locks behind a **passkey** (the *Shortcake* / CRSC flow). When the
  server demands a passkey, whatsmeow's `PairPasskeyRequest` is surfaced through a
  new ceremony flow: the backend mints a short-lived ceremony token, and a bundled
  browser extension (`passkey-helper`) runs the WebAuthn assertion on the
  `web.whatsapp.com` origin and posts it back. Three public endpoints drive it:
  `GET /passkey-ceremony/{token}`, `POST .../response`, `POST .../confirm`. The
  manager detects the passkey stage and shows an "Abrir WhatsApp Web" button.
  Confirmation is always manual (never auto-confirm on `SkipHandoffUX`).
  Configure the public API base via **`PASSKEY_PUBLIC_URL`**. Full guide:
  `docs/wiki/guias-api/passkey-pairing.md`. Note: there is no headless bypass —
  the ceremony requires the account owner's real authenticator; the extension is
  web-only.
- **Headless license auto-activation** — set `EVOLUTION_OPERATOR_EMAIL` to the
  email used in your first manual license registration; on startup the service
  silently calls `/v1/register/auto` and skips the browser flow (falls back to the
  manual flow if the email isn't registered yet).
- **Button message media support** — additional media handling for interactive
  button messages.

### 🔧 Improvements / CI
- **Dropped the whatsmeow fork — now uses official `go.mau.fi/whatsmeow`.** The
  project previously vendored a fork (`whatsmeow-lib` submodule) to carry a
  PostgreSQL pool patch; upstream rejected that patch in favor of `NewWithDB`
  (app-side config). Removing the fork also pulled in upstream's native passkey
  support. Pinned to the commit that adds passkeys
  (`v0.0.0-20260630180629-b572e5bcb92b`). The `sync-releases` workflow no longer
  re-adds the submodule.
- **QR pairing consumes `events.QR` directly** instead of `GetQRChannel`. The QR
  channel auto-confirms passkey on `SkipHandoffUX` and disconnects the socket when
  codes run out — both break an in-flight passkey ceremony. Connecting without it
  keeps the socket alive for as long as pairing (QR or passkey) needs. QR rotation
  now pauses while a passkey ceremony is active.
- **Public sync fixes** — the release workflow drops the obsolete whatsmeow-lib
  step, targets `evolution-foundation/*`, and now ships the `passkey-helper`
  extension to the public repo.

### 🐛 Bug Fixes
- **`POST /instance/pair` returned an empty `PairingCode`** — the handler
  swallowed `PairPhone` errors and returned HTTP 200 with `PairingCode: ""`, and
  the client wasn't connected/awaiting-auth before `PairPhone`. Now starts the
  instance, waits for the websocket, and surfaces real errors (#21).
- **`GET /instance/status` returned 400 after a manual disconnect** — now returns
  200 with the disconnected status instead of erroring until a container restart
  (#20).

### 🏷️ Org rename
- Repository references updated from **EvolutionAPI** to **evolution-foundation**
  (module path, imports, GitHub URLs, submodule URLs).

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
- **Dependency cleanup** — removed unused `github.com/evolution-foundation/evo-gate` from `go.mod`
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
- **GitHub**: [Evolution API Go](https://github.com/evolution-foundation/evolution-go)

---

## 🤝 Contributing

To contribute to the project:
1. Fork the repository
2. Create a branch for your feature
3. Commit your changes
4. Open a Pull Request

---

*Last updated: October 2025*
