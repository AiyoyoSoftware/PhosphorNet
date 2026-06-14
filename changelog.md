# changelog.md

## 2026-06-14 - Installed Layout And Installer Modes

- Added `install.sh` with `--client`, `--node`, `--full`, `--uninstall`, and safe `--purge` handling for the default curl-based install route.
- Added a tag-triggered GitHub Actions release workflow that builds Linux and Windows `phosphor`, `phosphord`, and `switchboard` archives and uploads them as release assets.
- Added a manual release workflow trigger so missed tag events can still publish an existing tag such as `v0.1`.
- Fixed manual release publishing so it checks out the workflow branch instead of requiring the release tag to exist before checkout.
- Fixed release publishing so generated release notes run inside a checked-out repository.
- Standardized installed locations for binaries, bundled doors, node config, and SQLite state under `/usr/local/bin`, `/usr/local/share/phosphornet`, `/etc/phosphornet`, and `/var/lib/phosphornet`.
- Pointed installer release downloads at `AiyoyoSoftware/PhosphorNet` GitHub release assets, with `PHOSPHORNET_ARTIFACT_URL` available for exact archive testing.
- Made `phosphord` default to the installed node config while allowing user-level `~/.config/phosphornet` and `~/.local/share/phosphornet` overrides when present.
- Added `phosphor init` to create or reuse the default passport as the friendly client setup command.
- Updated setup, configuration, user, and README docs for the installed route, source-checkout route, uninstall behavior, and default paths.
- Added tests for installed path resolution, node init path selection, client init passport reuse, and installer install/uninstall modes.

## 2026-06-06 - Reconnect Session Recovery Semantics

- Defined the public-alpha reconnect rule: reconnects create fresh sessions, briefly reopen the previous door only when it is still safe, otherwise return to `lobby`.
- Added delayed disconnect cleanup so `on_leave` runs after a short grace window, while reconnecting within that window cancels the pending leave and avoids a duplicate `on_join`.
- Kept presence live-only and documented that scroll position, focus, and unsent input drafts are not recovered across reconnects.
- Added node integration coverage for same-door recovery within the grace window and leave cleanup after the grace expires.

## 2026-06-06 - Event Ordering And Double-Submit Guards

- Added render revisions to node render messages and active-door/render/event metadata to client event messages.
- Made `phosphord` reject mismatched session or active-door event envelopes, duplicate event IDs within a short live-session window, and stale render revisions for submit-like events.
- Updated the TUI client to attach the active door, current render revision, and generated event ID to each outbound event.
- Documented the shipped event-ordering behavior in the architecture and todo docs, with focused node tests for stale and duplicate event rejection.

## 2026-06-06 - Test Index

- Added `docs/PhosphorNet_test_index.md` as the release-confidence test index, mapping the current protocol, security, runtime, admin, moderation, audit, and compatibility promises to the tests that cover them.
- Updated the todo test-matrix section to point at the index and document the remaining high-signal coverage gaps instead of implying the whole matrix is still unimplemented.

## 2026-06-06 - Dependency Refresh

- Updated Go module dependencies to their latest stable versions selected by `go get -u ./...`.
- Tidied `go.mod` and `go.sum` after the dependency refresh.
- Verified the refreshed dependency graph with the full Go test suite.

## 2026-05-30 - Alpha Audit Trail

- Added a durable SQLite `audit_events` table with timestamp, actor public key/fingerprint, action, target, result, and JSON metadata fields.
- Made audit events append-only-ish with storage append APIs, SQLite row-update rejection, and controlled oldest-row retention when `--audit-log-max-bytes` is set.
- Added `phosphord serve --audit-log-file` to mirror audit events to an optional JSON Lines file for operators, with size-based rotation through the shared `--audit-log-max-bytes` knob and backup count through `--audit-log-file-max-backups`.
- Audited admin role and door policy changes, door setting edits, manifest reloads, maintenance changes, event-log clearing, moderation operations, failed/denied auth, door access denials, denied privileged effects, and detected node key changes at startup.
- Added focused audit tests plus websocket integration coverage for auth-denial, admin-change, and SQLite max-byte trimming audit behavior.
- Updated configuration, runtime contract, database lifecycle, and todo docs for the server-side alpha audit trail.

## 2026-05-17 - README Use Case Clarity

- Rewrote the README "What It Is Good For" section in simpler first-time-reader language, with concrete use cases and less architecture jargon.
- Added a compact list of currently shipped doors to the README overview.

## 2026-05-17 - High-Impact Integration Coverage

- Added a node integration test suite that exercises current high-impact shipped features from the changelog through full websocket sessions, temp Lua doors, Ed25519 auth, and SQLite-backed state.
- Covered client compatibility rejection before auth, signed node challenge verification, successful auth plus lobby render, user state persistence across reconnects, admin door setting updates rerendering active sessions, and moderation bans disconnecting active users while blocking reconnects.
- Expanded the integration suite to cover admin door-policy refreshes and disabled-door denial, muted-user event enforcement with navigation carve-outs, forged render-event rejection, and typed door-crash errors over websocket sessions.
- Added integration coverage for invite-only station admission, admin bypass and allowlisted users, role-policy updates that unlock role-gated doors for live sessions, profile-update fan-out into other users' presence views, and contract-invalid runtime output returning `runtime_bad_output`.
- Added integration coverage for live manifest reload publishing new door lists, room notification fan-out, broadcast-triggered peer rerenders over shared room state, per-user event rate limits, and transition budget exhaustion.
- Added integration coverage for manifest capability enforcement on state writes, raw key delivery for `capture_keys` doors, invite-only door allowlists using fingerprints, maintenance admin state flowing into runtime context, and per-user open-door rate limits.

## 2026-05-15 - Database Lifecycle, Runtime Errors, And Client Compatibility

- Added `docs/PhosphorNet_database_lifecycle.md` with the station backup bundle, stopped-daemon backup flow, restore procedure, migration expectations, schema-version reporting, and explicit outcomes for deleted SQLite databases or deleted node keys.
- Added SQLite schema version reporting through `PRAGMA user_version`, exposed the opened database path from storage, and logged database path plus schema version during `phosphord serve` startup.
- Seeded the default station policy during `serve` as well as `init` so a freshly recreated database gets the same default `strategy_demo` disabled policy.
- Added client compatibility negotiation to the pre-auth `hello`, including client version, runtime protocol version, JSON UI schema version, supported components, supported style features, supported event kinds, and render limits.
- Added server-side hello validation that rejects incompatible clients with `client_incompatible` before auth and before any door UI is sent.
- Added stable node-to-client error codes for runtime availability, missing Podman images, runtime timeouts, malformed runtime output, policy denial, resource limits, invalid manifests, and door crashes.
- Updated the client to display typed node error codes in trusted chrome.
- Added protocol, runtime, and storage tests for compatibility negotiation, coded runtime failures, and schema version reporting.
- Updated the runtime contract, architecture, configuration, setup, and todo docs for the shipped lifecycle, compatibility, and error-taxonomy behavior.
- Ignored the local `.gocache/` directory used for sandboxed Go test runs.
- Added trusted-client gradient backgrounds to every Station Admin door panel, plus the admin screen shell, with runtime coverage for the admin page set.

## 2026-05-15 - Stdio Podman Default

- Changed explicit `runtime = "stdio"` isolation resolution so missing `isolation.mode` defaults to Podman instead of host direct execution.
- Removed the deprecated `runtime = "python"` compatibility profile; Python doors now use `runtime = "stdio"` and include/invoke the Python SDK themselves.
- Removed the stdio host invoker's implicit Python SDK `PYTHONPATH` injection.
- Converted the bundled chat and strategy demo doors from stdio Python to embedded Lua.
- Added `sdk/python/Dockerfile` and `.dockerignore` as a reusable base image for containerized Python stdio doors.
- Kept host stdio execution as the opt-out path through `[isolation] mode = "host"`.
- Added runtime regression tests for implicit Podman mode, missing-image rejection, and host opt-out behavior.
- Updated stdio isolation docs to describe Podman as the default profile and host execution as the explicit opt-out.

## 2026-05-14 - README Screenshots

- Moved the current station screenshots to the top of the README, rendered them as smaller inline thumbnails, and added captions for the lobby, chat, forum, forum thread, and first-connect trust screen.

## 2026-05-14 - Podman Stdio Isolation Profile

- Capped the chat door's rendered backlog to the UI contract child limit while preserving the newest entries and showing an older-entry marker.
- Added manifest `[isolation]` parsing and validation for `runtime = "stdio"` doors with explicit `mode = "host"` for trusted direct execution or `mode = "podman"` for containerized execution.
- Added a Podman-backed stdio invoker path that constructs `podman run` as explicit argv, pipes canonical request JSON to stdin, and decodes canonical response JSON from stdout.
- Kept Podman as a launch wrapper under the existing stdio ABI rather than adding a new runtime protocol.
- Added a harsh default Podman profile with no network, read-only rootfs, dropped capabilities, no-new-privileges, keep-id user namespace, memory/CPU/pids limits, and no default door-directory mount.
- Added runtime tests for Podman argv generation, malformed JSON diagnostics, timeout handling, stderr caps, and missing image validation.
- Updated bundled stdio doors to declare `[isolation] mode = "host"`.
- Updated README, architecture, runtime contract, technology stack, authoring, configuration, and todo docs to separate manifest capabilities from host/container isolation.

## 2026-05-14 - Public Station Moderation Primitives

- Added a dedicated public-station moderation document that separates local survival primitives from advanced reputation and social-governance systems.
- Defined the minimum local moderation kit: ban key, mute key, content hide/delete, door freeze, maintenance notice, per-user rate limiting, abuse-relevant activity inspection, and later appeal/unban paths.
- Added node-owned station moderation policy for banned keys, muted keys, per-user event/open-door rate limits, and moderation notes.
- Added `admin:moderate_users` plus `ban_key`, `unban_key`, `mute_key`, `unmute_key`, `set_user_rate_limit`, and `record_moderation_note` admin ops.
- Enforced banned keys during auth and door access, disconnecting active sessions when a ban is applied.
- Exposed `ctx.permissions.muted` to doors and rejected generic muted write-like events while allowing navigation actions.
- Added a Station Admin Moderation page for key controls, ban/mute lists, rate limits, and recent notes.
- Updated architecture, runtime contract, authoring, user guide, README, and todo docs to point moderation work at the implemented primitive spec.

## 2026-05-14 - JSON UI Contract Schema Ownership

- Added the canonical `phosphornet.ui.v1` JSON UI schema under `internal/protocol/schema`.
- Added golden UI contract fixtures for all current components, fields, defaults, SDK output examples, invalid cases, and size-limit boundaries.
- Added client render goldens so trusted-client rendering expectations are tested alongside the protocol corpus.
- Added runtime tests that compare real Lua and Python SDK helper output against the SDK golden fixtures.
- Added typed protocol validation for UI trees and runtime responses, and enforced it at the door runtime boundary before node render state is accepted.
- Updated architecture, runtime-contract, and todo docs to make `internal/protocol` the owner of the JSON UI contract.

## 2026-05-13 - Container Gradient Backgrounds

- Added a typed `style.background` protocol shape for content-bearing UI components, with solid and gradient backgrounds limited to `screen`, `panel`, and `log`.
- Rendered container gradients in the trusted client while leaving leaf controls such as buttons on local client styling.
- Exposed optional container style arguments in the Lua and Python UI helpers.
- Updated the lobby screen, Station, Who Is Here, and Station Notice panels to use gradient backgrounds.
- Updated the forum, chat, and profile doors to use gradient-styled content containers.
- Made screen backgrounds fill the remote viewport area and kept panel-local backgrounds from painting their outer margins.
- Matched the station-view remote render size to the viewport panel content box so styled screen backgrounds reach the right and bottom edges.
- Removed the trusted `Remote View` / `Door View` header inside the remote content frame and let door content expand to the protected panel border.
- Kept panel backgrounds inside the rounded border so gradient panels no longer paint square corner cells.
- Reduced the extra top margin before styled panels so lobby content sits closer to the door header.
- Made Enter submit and clear bottom-docked composer inputs such as chat while keeping profile-style form inputs on local focus cycling.
- Made Enter insert a newline in textarea components so multiline forum posts can be composed locally before submission.
- Documented the gradient background contract and added client regression coverage.

## 2026-05-13 - Door Transition Render Fix

- Fixed `open_door` transitions so the source door no longer overwrites the target door render after a successful handoff.
- Added node regression coverage for the lobby-to-profile transition so the profile door opens on the first activation.
- Changed remote text input Enter handling to advance focus locally, leaving button activation to submit profile edits and other form-like door actions.
- Added client regression coverage for Enter cycling through profile fields before activating the Save Profile button with edited values.

## 2026-05-13 - Architecture Gap Backlog

- Added a PhosphorNet architecture gap list to the active todo document covering protocol compatibility, message ordering, reconnect recovery, UI schema ownership, error taxonomy, audit logging, database lifecycle, door provenance, lifecycle concurrency, backpressure, privacy, local identity management, node key rotation, public-station moderation, compatibility testing, and the Bubbles-backed client component model.

## 2026-05-13 - First-Connect Trust TUI

- Added a Bubble Tea first-connect trust screen before known-node pinning so users review transport encryption, certificate status, hostname verification, signed station name, and Ed25519 station identity as separate facts.
- Added `node_name` to the Ed25519-signed node challenge payload so the trust screen can show the station name as a signed station claim instead of deriving it from certificate metadata or delayed session chrome.
- Kept changed station keys as a hard refusal unless `--replace-known-node` is supplied, and improved the replacement/status path around the same trust summary.
- Updated README, setup, user-guide, and todo docs now that first-connect trust UX is implemented.
- Added client, identity, and node regression coverage for signed station-name challenge verification and the trust-screen model.

## 2026-05-12 - Documentation Truth Alignment

- Aligned architecture and configuration docs around station admission, session roles, door access, door visibility, and door capabilities as separate concepts.
- Documented the current role vocabulary as `member`, `admin`, and `sysop`, with guest-like display names treated separately from station-policy roles.
- Added a runtime-contract authority table for every current `admin_op`, including required role, capability, and target.
- Expanded setup and user docs to explain the trust stack: TLS encryption, certificate/hostname status, and Ed25519 known-node station identity.
- Marked the MVP implementation roadmap as historical at the top of the file and pointed active backlog readers to `docs/PhosphorNet_todo.md`.
- Added todo entries for first-connect trust UX, admin-op authority review, role/access vocabulary drift, and stdio containment.
- Cleaned up stale Python-subprocess wording and removed a duplicated Python runner snippet from the door authoring guide.

## 2026-05-12 - Generic Stdio Door Runtime

- Replaced the Python-specific subprocess invoker with a generic `stdio` runtime backend that sends canonical runtime request JSON to stdin and reads canonical runtime response JSON from stdout.
- Added manifest `command = [...]` support for stdio doors, allowing command-only manifests such as `runtime = "stdio"` with `command = ["python3", "app.py"]`.
- Kept `runtime = "python"` as a deprecated compatibility profile implemented through the stdio backend.
- Updated the Python SDK runtime with `run_module(...)` so Python door files can be direct stdio commands.
- Converted bundled Python doors to `runtime = "stdio"` command manifests and executable SDK-backed scripts.
- Added stdio runtime tests for canonical request handling, malformed JSON with stderr diagnostics, command-only manifest loading, and timeout enforcement.
- Updated README and runtime/config/authoring/setup docs to describe stdio as the non-Lua runtime bridge.

## 2026-05-12 - Node-Owned Door Capabilities And Admin Ops

- Added manifest `capabilities = [...]` with validation and compatibility mapping from deprecated `permissions = [...]` values.
- Enforced door capabilities for state writes, broadcasts, notifications, `open_door` transitions, key capture, and node-owned admin operations.
- Added node-owned SQLite `node_state` storage for station policy and migrated legacy admin-door global policy into it on first read.
- Added typed `admin_ops` to the runtime response envelope and wired Lua/Python SDK helpers so doors request privileged mutations instead of writing magic global state.
- Moved Station Admin policy changes, door settings, maintenance controls, manifest reload, and event-log clearing onto admin ops guarded by admin/sysop role plus manifest capability.
- Split admin runtime context into `ctx.admin`, only populated for admin/sysop sessions when the door has matching read capabilities.
- Updated bundled manifests, runtime/config/authoring docs, and regression tests for the capability and node-owned policy model.

## 2026-05-11 - README Value Proposition Refresh

- Reworked the README introduction to lead with PhosphorNet as a self-hostable platform for interactive terminal apps rather than relying primarily on BBS framing.
- Added a concrete "What It Is Good For" section covering private stations, terminal-native communities, admin tools, games, self-hosted policy, and safe remote UI prototyping.
- Repositioned the BBS comparison as flavor while foregrounding trusted client rendering, portable Ed25519 identity, and local station control as the general value proposition.
- Tightened the top of the README around the SSH-versus-private-web-app use case, moved the detailed status material below Quick Start, and removed the tentative project-name note.
- Polished final public-alpha README wording around station operators, non-shell access, machine dashboards, MVP caveats, authority boundaries, and roadmap ordering.
- Added an Apache-2.0 license file and replaced the README license placeholder with a link to it.

## 2026-05-11 - Container-Fit Input Rendering

- Made panel rendering pass the bordered panel's inner content width to child components so inputs and textareas fit their container instead of sizing against the outer viewport.
- Added a focused panel input regression test covering placeholder rendering inside a bordered container.

## 2026-05-11 - Manifest-Declared Door Settings

- Added manifest-declared door settings with typed schema/default support for `string`, `textarea`, `bool`, `int`, `select`, and `markdown`.
- Exposed resolved per-door settings to Lua and Python doors through `ctx.settings`, overlaying Station Admin edits on top of manifest defaults.
- Added a generic Station Admin Door Settings page that renders manifest schemas and stores station-specific edited values in SQLite-backed admin global state instead of rewriting manifests.
- Added bundled door settings for lobby MOTD/presence/tagline, chat title/topic/backlog/presence/join-log behavior, forum welcome/policy/display behavior, profile prompts/help, and admin display/action controls.
- Updated the bundled doors to read those values from `ctx.settings`.
- Updated README and runtime/config/authoring/user docs, plus manifest/runtime/admin regression coverage for the new settings flow and bundled manifest parsing.
- Removed duplicate Station Admin per-door reorder cards now that the Doors page uses one dynamic-list-based navigation ordering flow.

## 2026-05-10 - Arrow-Only Navigation Copy

- Removed `j` / `k` keyboard navigation from the trusted client so arrow keys remain the only directional navigation keys.
- Updated trusted chrome help text, Station Admin reorder guidance, README controls, and the user guide to remove `j` / `k` navigation references.

## 2026-05-10 - Alpha Boundary Hardening For Doors, Events, And State

This entry documents the pre-alpha safety pass that tightens path authority, protocol authority, and resource bounds without changing the core PhosphorNet architecture.

- Made node config and door manifest loading strict about unknown TOML keys and added validation for required fields, enum-like values, and Lua sandbox/runtime settings.
- Rejected duplicate door IDs during manifest scans so door identity no longer depends on ambiguous first-match behavior.
- Tightened door entry confinement by resolving real paths through symlinks, anchoring entry resolution to the manifest's actual directory, and rejecting door directories or entry files that escape the configured `doors_dir`.
- Added node-side render-tree event validation so `phosphord` now rejects mismatched session IDs, forged component targets, invalid actions, key events without `capture_keys`, and malformed submit payloads before door `update` handlers run.
- Added transition guards that cap transitions per response, treat same-door `open_door` transitions as no-ops, and prevent transition recursion from running unbounded.
- Added state-op bounds for batch size, key syntax, per-value JSON size, total batch JSON size, and persisted scoped-state blob size.
- Tightened invite-only station admission so explicit station allowlists and configured admins control entry; station role assignments no longer bypass admission by themselves.
- Made `phosphord init` seed the default station policy so `strategy_demo` starts disabled on fresh stations until an admin explicitly enables it.
- Updated the Station Admin access page copy and configuration docs to describe the stricter invite-only policy more precisely.
- Added regression coverage for strict config loading, duplicate door IDs, symlink escapes, forged events, transition limits, and bounded state operations.

## 2026-05-10 - Stable Focused Input Rendering At Exact Width

This entry documents a small trusted-client rendering fix for focused inputs and textareas.

- Made the focused-field cursor padding width-aware so empty or placeholder-only fields stay one line tall when they exactly fill the render width.
- Added a renderer regression test covering a focused textarea at the exact width boundary that previously produced an extra wrapped line.

## 2026-05-10 - AGENTS.md Navigation And Fidelity Refresh

This entry documents the `AGENTS.md` rewrite that makes the repo easier for coding agents to navigate without broad context loading.

- Reworked `AGENTS.md` into a compact agent-operating guide with a strict read-minimally workflow, task-to-doc source map, and subsystem navigation map.
- Corrected stale guidance so the file reflects the current repo more accurately, including Lua as the default door runtime, encrypted transport by default, and mutual Ed25519 identity proof.
- Added explicit reality-check guidance for partial or easy-to-misread areas such as informational manifest permissions, partial transition support, implicit per-door rooms, in-memory presence, scaffold-level switchboard status, and the partly historical MVP roadmap.
- Consolidated duplicate change-tracking guidance into one section and tightened the file around implementation-safe rules instead of repeated project prose.

## 2026-05-10 - Doc Clarifications For Permissions, Transitions, And Backlog Ownership

This entry documents the documentation cleanup pass that removes a few easy-to-misread statements.

- Updated the configuration guide to say plainly that manifest `permissions = [...]` is currently informational metadata and does not, by itself, enforce capability checks.
- Tightened the authoring and runtime-contract docs so they distinguish implemented `open_door` transitions from other declared transition kinds that are still reserved future work.
- Marked the file-level MVP roadmap as partly historical and pointed readers at `docs/PhosphorNet_todo.md` for the current loose-end backlog.
- Updated the README MVP-status section to reflect partial transition support and the new backlog ownership guidance.

## 2026-05-10 - Signed Node Challenge Verification

This entry documents the authentication hardening that makes the station prove its Ed25519 identity back to the client during login.

- Extended the `challenge` message so the node signs a typed challenge payload containing `purpose`, `node_id`, `client_public_key`, `nonce`, and `timestamp`.
- Made `phosphor` verify the signed node challenge before pinning or trusting the server identity.
- Kept known-node pinning in place, but moved it behind successful signed challenge verification so the client no longer trusts an unsigned `node_id` claim.
- Added identity and client tests covering signed node challenges and invalid challenge signatures.
- Updated README, setup, configuration, and architecture docs to describe the mutual Ed25519 identity proof more precisely.

## 2026-05-10 - Default TLS For Station Sessions

This entry documents the transport-security pass that makes station sessions encrypted by default without changing PhosphorNet's Ed25519 identity model.

- Made `phosphord` serve `wss://` and `https://` by default through an in-memory self-signed TLS certificate derived from the station's existing Ed25519 node key.
- Added `[tls].enabled` to node configuration, defaulting to `true`, so operators can still intentionally disable TLS for plain local or proxied setups.
- Switched `phosphor connect` defaults from `ws://127.0.0.1:7707/ws` to `wss://127.0.0.1:7707/ws`.
- Made the client accept self-signed station certificates by default while continuing to treat the Ed25519 challenge-response plus known-node pinning as the real station identity check.
- Simplified the trusted-client status copy for TLS-backed sessions to `Session encrypted.`.
- Added node and client test coverage for the new TLS defaults and updated README, setup, configuration, user, architecture, technology-stack, and todo docs to match the new transport default.

## 2026-05-10 - Consolidated Todo Document

This entry documents the new in-repo backlog document for scattered MVP loose ends.

- Added `docs/PhosphorNet_todo.md` as a single place to track current unfinished edges, documentation drift, and explicitly deferred later work.
- Consolidated loose ends that were previously spread across README notes, architecture docs, the MVP roadmap, authoring/config docs, and changelog history.
- Called out two especially easy-to-misread areas in the new backlog: manifest `permissions = [...]` being informational today, and transition support being partially implemented rather than uniformly complete.

## 2026-05-07 - Vertical Protected Chrome Navigation

This entry documents the protected-chrome navigation consistency tweak for the station rails and remote viewport.

- Changed protected-chrome navigation so `tab` cycles between chrome sections, `left` / `right` cycle selectable components inside the focused section, and `up` / `down` scroll the focused panel.
- Added a scroll offset for the side rail so the left door panel scrolls independently while keeping door selection on `left` / `right`.
- Made the split-view remote panel use the same blue focus border as fullscreen whenever `tab` moves focus into the main content area.
- Added explicit door key capture for screens that set `capture_keys`, while keeping trusted client shortcuts and focused text inputs local to the client.
- Added Station Admin door-order controls for enabled doors, including `=` / `-` reordering that updates the trusted navigation menu order across connected sessions, while still accepting `+` as an alias.
- Sorted the Station Admin Doors page from persisted `door_order` state immediately so the admin content view matches the trusted left rail after one move, and replaced the single order menu with direct per-door reorder controls.
- Restored the visible enabled-door order list inside the Station Admin navigation-order panel so the current trusted menu order remains readable at a glance.
- Replaced the rendered text block for door ordering with a dedicated `dynamic_list` component so the order view no longer draws a mangled nested border and each row is independently focusable and selectable.
- Whitelisted `dynamic_list` in the trusted client sanitizer so it renders as an approved component instead of collapsing to `[unsupported component]`.
- Updated client help text, README controls, the user guide, and TUI navigation tests to match the new section-and-scroll behavior.

## 2026-05-07 - Station Identity And Lobby Onboarding

This entry documents the station-identity pass focused on display names, arrival flow, and clearer social presence.

### Identity and runtime

- Added station-local profile storage for a single human-facing `display_name` plus optional `bio` and `status_line`, while keeping the passport fingerprint as the actual identity anchor.
- Added runtime profile update effects so doors can request profile changes without bypassing node-owned validation and persistence.
- Extended runtime context, presence snapshots, and known-user records with display-name and guest metadata for Lua and Python doors.
- Added migration `004_user_profiles.sql` for the new profile fields.

### Bundled doors

- Reworked the lobby into a cleaner three-panel station homepage with Station, Who Is Here, and Station Notice.
- Moved profile editing into a standalone `profile` door so it loads through the normal manifest path and appears automatically in door inventories such as Station Admin.
- Made the lobby link to the `profile` door only when the current passport does not yet have a station display name.
- Hot-wired Station Admin manifest reload so newly added doors can be loaded into the running node and then enabled or disabled from the Doors page without a restart.
- Changed chat to use station display names instead of room-local nicknames, added `/who`, and made `/nickname` update the station profile display name.
- Updated the forum to resolve post authors through station profile identity while still showing the passport fingerprint on post cards.
- Updated the admin Users view to surface display names, guest identity, profile fields, and clearer signed-in identity copy.

### Documentation and verification

- Updated the user guide for station profiles, guest mode, lobby onboarding, and the new chat/forum identity behavior.
- Added storage tests covering profile persistence and reserved display-name validation.

## 2026-05-06 - Forum Door And Markdown Rendering

This entry documents the forum slice that turns the station into a place with durable conversation and locally rendered markdown.

### UI protocol and client

- Added a declarative `markdown` UI component that carries plain markdown source from doors and renders it locally in the trusted client.
- Added Lua and Python `ui.markdown()` helpers plus a Go `protocol.Markdown()` helper for test and runtime construction.
- Rendered markdown locally with Glamour in the trusted client, with clamping and sanitization applied before rendering and plain-text fallback on renderer failure.
- Kept markdown rendering client-side only so doors still send plain text instead of raw ANSI or HTML.

### Bundled doors

- Added the bundled `forum` Lua door with home, thread list, thread view, new thread, reply, and moderation confirmation subviews.
- Seeded the forum with a pinned welcome thread covering rules, vibe, and what doors are.
- Added latest-activity sorting, per-user drafts, reply posting, pinned welcome handling, and admin/sysop post hide/delete controls.
- Reshaped the forum thread page into a classic board view with the starter post at the top, replies below, and the reply action at the bottom.
- Simplified the forum home page down to a single thread list panel plus a new-thread button, with thread actions kept on the thread page itself.
- Removed thread counts and per-thread previews from the forum landing page so it shows only the thread list and new-thread entry point.
- Kept poster names in the forum thread view while trimming the thread list back to title-only entries.
- Removed duplicate author text from forum post cards so the poster name appears only once per post.
- Reset the forum door navigation stack on join so returning to the door always lands on the thread list instead of reopening a stale thread subview.
- Flattened moderation controls inside the forum thread card and kept them visible only to station admins.
- Updated the lobby door to feel like a station homepage with direct entry points to chat, forum, and station admin.
- Added `open_door` transitions for lobby/forum handoff so the station shell can move between doors without client-side routing.

### Runtime and docs

- Updated the door runtime contract and authoring guide for the new markdown component and the `open_door` transition path.
- Added the Glamour dependency and aligned the Charmbracelet terminal helper packages to a compatible version set.
- Updated tests for markdown rendering, forum seeding, forum posting, and the new lobby state-op behavior.

## 2026-05-06 - Client And Sandbox Security Hardening

This entry documents the security audit pass for hostile remote UI content and door runtime boundaries.

### Client rendering and trust chrome

- Added recursive UI-tree sanitization before rendering so hostile node strings cannot inject ANSI/OSC/control sequences into the terminal.
- Clamped remote text, labels, notifications, status lines, and trust chrome fields to bounded lengths before display.
- Made unknown components render as inert warning text instead of recursing or crashing on malformed trees.
- Added tree-depth, child-count, item-count, and grid-size caps to keep hostile render trees finite and inspectable.
- Added render-rate and notification-rate enforcement in the trusted client so noisy doors are disconnected safely.

### Runtime boundaries

- Disabled unsafe Lua library loading paths so `io`, `os`, `debug`, `package`, and related unsafe loaders are not exposed through the sandbox profile switch.
- Kept Lua execution bounded with the existing timeout path and explicit denylisting of unsafe globals.
- Added Python door subprocess time limits, a door-directory working directory, a minimal environment, and capped stdout/stderr capture.
 - Added websocket read limits and kept handshake timeouts, while preserving quiet steady-state sessions.

### Verification

- Added hostile-tree renderer tests for escape sequences, bidi controls, zero-width spam, huge emoji sequences, unknown components, deep trees, wide children arrays, and grid truncation.
- Added regression tests for Lua sandbox behavior and Python door timeout handling.

## 2026-05-06 - IRC-Style Chat Scrollback

This entry documents the chat door overhaul.

### Chat door

- Reworked the bundled `chat` door into an IRC-like channel view with a topic/presence header, full transcript log, and bare message input at the bottom.
- Removed the Ping button and compose panel chrome from the main chat surface.
- Added slash commands: `/nickname <name>` for room-visible nicknames, `/tell <nickname> <message>` for private notices, and `/help` for local command help.
- Changed local chat command output, including `/help`, `/tell` confirmations, and command errors, to render only in the current response instead of persisting in user state.
- Increased retained room message history from 30 to 250 messages so scrolling up can reveal earlier events.
- Changed join/leave notices into channel event lines and removed extra send notifications for normal messages.
- Added a `screen.scroll = "bottom"` hint so the chat door opens at the newest messages.

### Client behavior

- Added trusted-client handling for bottom-pinned render trees.
- Added explicit door-owned `dock = "bottom"` support for final input/textarea composers in bottom-pinned transcript screens, with Lua and Python SDK support.
- Kept bottom-pinned views at the bottom on new renders only when the user is already at the bottom or the door was just opened.
- Preserved the user's scrollback position when they scroll upward before new messages arrive.
- Made focused remote inputs consume printable keypresses before global shortcuts so letters like `f` and `q` can be typed normally.

### Documentation and verification

- Updated runtime, authoring, and user docs for bottom-pinned transcript views.
- Added client tests for bottom-pinned render behavior and runtime coverage for the chat door's bottom scroll hint.

## 2026-05-06 - Admin Console Pages

This entry documents the Station Admin door becoming a multi-page sysop console.

### Admin door

- Rebuilt the admin door around door-owned subviews: Home, Doors, door detail, Users, user detail, Access Control, Storage, storage detail/scope, Runtime, Logs, Maintenance, and confirmation pages.
- Preserved existing station notices, maintenance mode, maintenance checkpoints, role assignment, door role visibility, and door enable/disable controls under the new page structure.
- Added admin-visible manifest details including runtime, entry file, allowlist count, and Lua sandbox profile.
- Added admin-visible known-user, online-session, runtime, config, database path, and scoped state summary panels.
- Added an in-memory node event ring for auth, door open, access-denied, runtime-error, and admin-action events shown on the Logs page.
- Moved reset maintenance through a confirmation subview.
- Kept not-yet-implemented unsafe operations, such as cross-door state clearing and hot manifest reload, as explicit admin notices instead of silent one-click actions.

### Node context and storage

- Extended runtime context for admin-capable sessions with database path, doors directory, default runtime, Lua sandbox summary, station allowlist/admin entries, known users, and scoped state summaries.
- Added user `last_seen` tracking and scoped state inventory queries to SQLite storage.
- Extended door summaries with runtime, entry, allowlist count, and sandbox profile metadata.
- Added migration `003_user_last_seen.sql` for the new user timestamp column.

### Documentation and verification

- Updated user/config docs for the new admin console shape and `last_seen` user metadata.
- Added storage and node tests for user timestamps, state summary listing, and enriched door manifest metadata.

## 2026-05-06 - Door Storage Helpers And Subviews

This entry documents durable door data helpers and door-owned subview navigation.

### Door SDKs

- Added Lua `ctx.store` helpers for scoped state `get`, `set`, `append`, `delete`, `clear`, `replace`, and `all`.
- Added Python `ctx.store` helpers with the same scoped state operations.
- Added Lua and Python `ctx.nav` helpers for per-user door subview stacks with `current`, `push`, `back`, `reset`, and semantic event handling.
- Added Lua and Python UI helpers for `nav_button` and `back_button`.
- Added a Python `checkbox` UI helper to match the existing protocol component.

### Documentation and verification

- Updated the runtime contract, architecture, and door-authoring docs with explicit persistent storage and subview examples.
- Added runtime tests covering forum-like post persistence and subview navigation helpers in both Lua and Python.

## 2026-05-05 - Station Role Policy In Admin Door

This entry documents role-based station administration controls.

### Admin door

- Added station role assignment controls to the `Station Admin` door.
- Added controls to remove a role assignment from a public key.
- Added door role visibility controls that let admins set the roles that can see and open a door.
- Added hosted-door display of door IDs, explicit visibility/access labels, active role visibility policy, and enable/disable checkboxes.
- Kept disabled doors visible inside the admin panel for re-enablement while hiding them from normal navigation.
- Made admin door checkbox rendering use the freshly updated disabled-door state so re-enabled doors appear checked immediately after one action.
- Preserved maintenance state reset without wiping station roles or door role visibility policy.

### Node enforcement

- Added station policy loading from the admin door's global state.
- Made `phosphord` assign roles from the station policy during authentication, while keeping configured admins authoritative.
- Allowed role-assigned users into invite-only stations.
- Made role-gated doors visible and openable only to sessions with one of the configured roles, with `admin` and `sysop` retaining override access.
- Hid disabled doors from all normal door lists and non-admin runtime door inventories.
- Kept the admin door itself always enabled to avoid station lockout.
- Refreshed connected sessions' roles after station role policy changes.
- Refreshed connected clients' door lists after role policy changes.
- Rechecked active-door access before routing door events.
- Extended door summaries with role and disabled metadata for admin/runtime views.

### UI protocol

- Added a semantic `checkbox` component with Lua SDK support.
- Added client rendering and action events for checkboxes.
- Added live client handling for refreshed `door_list` messages.

### Verification

- Added node tests for role-assigned invite-only admission, role-gated door access, disabled-door hiding, admin inventory visibility, policy-change detection, and connected-session role refresh.
- Added runtime tests for admin-door role assignment, door role visibility state updates, and door enable/disable updates.
- Added client renderer tests for checkbox rendering and toggle events.

## 2026-05-05 - Stable Input Rendering

This entry documents a client rendering fix for input components inside scrollable door panels.

### Client rendering

- Changed the station header to a compact two-line layout with the title on line one and tabular node/user metadata on line two.
- Made station chrome and remote component backgrounds explicit so panel interiors do not show transparent gaps inside borders.
- Left top and bottom bar text backgrounds transparent so they no longer paint rectangular strips over the chrome surface.
- Made top and bottom bar rows full-width chrome surfaces so stale panel-colored background cannot remain to the right of short text.
- Changed `input` and `textarea` component rendering from bordered multi-line boxes to stable single-line controls.
- Preserved focus styling while avoiding broken partial borders in fullscreen and scrolled door views.
- Made `tab` focus the remote content even when a view has no focusable controls.
- Made `up` and `down` always scroll remote content while the main content panel is focused.
- Made `left` and `right` select visible remote controls in the current viewport.
- Kept `tab` as the escape hatch between the door menu and main content panel.
- Limited `j` and `k` to focused menu/list choices so regular text input can still receive those characters.
- Removed `h` and `l` as component-selection aliases in remote content so they can be typed into inputs.
- Cleared remote component focus when scrolling so `enter` cannot activate a stale control after the viewport moves.
- Removed the debug-like `Remote Focus` side panel from station view.
- Made the left rail a single door menu beside the main content panel.
- Added renderer and TUI navigation tests for the compact header, single-line inputs, remote-content tab focus/unfocus, left/right control selection, and scroll clearing stale focus.

### Local files

- Added `node.toml` to `.gitignore` because generated node configs contain private keys.

## 2026-05-05 - Known Node Replacement Flag

This entry documents a local testing escape hatch for regenerated node identities.

### Client

- Added `phosphor connect --replace-known-node` to replace a changed known-node key for the selected address.
- Preserved the default SSH-like protection that rejects changed node keys unless the flag is explicitly set.

### Documentation and verification

- Updated README, setup, configuration, and user docs with the replacement flow for local testing.
- Added client tests for changed-key rejection and explicit replacement.

## 2026-05-05 - Admin Door And Admin Roles

This entry documents the first bundled station administration door.

### Access control

- Added `access.admins` node configuration for passport public keys or fingerprints that should authenticate as `admin`.
- Updated `phosphord init` to create or reuse a local admin passport and seed the new node config with admin access.
- Made configured admins implicitly allowed into invite-only stations.
- Added an `admin` door access mode that only `admin` and `sysop` sessions can open.

### Doors

- Added the bundled `admin` Lua door as `Station Admin`.
- The admin door is visible only to admin-capable sessions because `phosphord` filters door lists by access before the client renders the rail.
- Added station inventory, connected-session visibility, maintenance mode, station-wide notices, health-check recording, maintenance checkpoints, notice clearing, and resetting admin global state.
- Added an `all` notify target so admin actions can send notices to every connected session.

### Documentation and verification

- Updated README, configuration, user, and door-authoring docs for admin roles and admin-only doors.
- Added tests for admin passport creation/reuse, init-time admin seeding, admin role assignment, invite-only admin admission, admin-only door access, station-wide notifications, and admin door global state effects.

## 2026-05-05 - Door Visibility And Invite-Only Access

This entry documents the first station and door access-control pass.

### Access control

- Added station-level access configuration with `public` and `invite_only` modes.
- Added station allowlists that accept passport public keys or fingerprints.
- Added door manifest metadata for `visibility`, `access`, and per-door `allowlist`.
- Enforced invite-only station access during authentication.
- Enforced invite-only door access during door listing and door opening.

### Client filtering

- Extended door summaries with visibility and access metadata.
- Filtered `private` and `hidden` doors out of the client door rail.
- Added an empty-door-rail message when no public doors are visible.

### Documentation and verification

- Updated configuration, user, and door-authoring docs for station allowlists and door visibility/access metadata.
- Added tests for station allowlists, door invite-only access, door summary metadata, and client hidden/private door filtering.

## 2026-05-05 - Setup, Configuration, Usage, And Door Authoring Docs

This entry documents the addition of practical project documentation for operators, users, and door authors.

### Documentation

- Added `docs/PhosphorNet_setup.md` with local setup, health checks, build commands, reset guidance, and setup troubleshooting.
- Added `docs/PhosphorNet_configuration.md` covering node TOML, runtime defaults, Lua sandbox profiles, door manifests, client files, SQLite state, and switchboard scaffold behavior.
- Added `docs/PhosphorNet_user_guide.md` covering passports, known-node pinning, station view, controls, bundled doors, notifications, and client safety boundaries.
- Added `docs/PhosphorNet_authoring_doors.md` covering Lua and Python door authoring, lifecycle hooks, runtime context, UI components, events, state scopes, effects, presence, and authoring guardrails.
- Updated README documentation links to include the new guides.

## 2026-05-04 - Embedded Lua Door Runtime

This entry documents the addition of Lua as the default embedded door runtime.

### Runtime

- Added `github.com/yuin/gopher-lua` as the embedded Lua VM used by `phosphord`.
- Added a Lua invoker that executes door lifecycle functions in-process through the existing runtime contract.
- Made Lua the default runtime for manifests without an explicit runtime, while keeping `.py` manifests and `runtime = "python"` on the Python backend.
- Added an embedded `phosphornet.ui` Lua SDK table and Lua `ctx.effects` helpers for state, notify, broadcast, and transition effects.
- Added configurable Lua sandbox options with strict defaults for libraries, memory, execution timeout, registry size, and call stack size.
- Added node-level runtime defaults and door-level sandbox overrides through TOML configuration.
- Tightened door entry resolution so manifests cannot point outside their own door directory.

### Doors

- Migrated the bundled `lobby` door from Python to Lua.
- Kept `chat` and `strategy_demo` on the Python runtime as compatibility proof doors.

### Documentation and verification

- Updated README, architecture, technology stack, and runtime contract docs for Lua as the default runtime and Python as an optional runtime.
- Added runtime tests for Lua invocation, default runtime resolution, Python compatibility, and Lua sandbox library behavior.
- Verified Go packages with `GOCACHE=/tmp/phosphornet-go-build GOMODCACHE=/tmp/phosphornet-go-mod go test ./...`.

## 2026-05-04 - README Refresh

This entry documents the README update to reflect the current implementation state.

### Documentation

- Replaced the placeholder `README.md` with a complete project overview.
- Documented the current MVP feature set, quick-start flow, client controls, repository layout, door model, runtime contract, security boundary, development commands, and docs index.
- Called out what remains intentionally MVP or unfinished.

## 2026-05-04 - Fullscreen Door Chrome

This entry documents the trusted-client fullscreen mode for door-focused experiences.

### Client chrome

- Added a local fullscreen door mode toggled with `f`.
- Added `esc` handling to leave fullscreen mode and return to the station shell.
- Fullscreen mode hides the door rail and remote-focus rail while keeping slim trusted client chrome visible.
- Fullscreen mode gives the remote viewport more width and height for game-like doors.
- Entering fullscreen moves focus to remote controls when the door exposes focusable components.
- Updated station-view key help to advertise fullscreen mode.

### Tests and verification

- Added a client test confirming fullscreen mode allocates a larger remote viewport than station mode.
- Verified Go packages with `GOCACHE=/tmp/phosphornet-go-build GOMODCACHE=/tmp/phosphornet-go-mod go test ./...`.

## 2026-05-04 - Client Idle Session Stability

This entry documents a client idle-disconnect fix.

### Client connection loop

- Removed the 30-second timeout from the background WebSocket read loop.
- The client now waits indefinitely for node messages while connected instead of treating quiet/idle sessions as read failures.
- Handshake and write operations still keep bounded timeouts.

### Verification

- Verified Go packages with `GOCACHE=/tmp/phosphornet-go-build GOMODCACHE=/tmp/phosphornet-go-mod go test ./...`.

## 2026-05-04 - Remote Viewport Scrolling

This entry documents the client viewport fix for tall door screens.

### Client rendering

- Added height-aware clipping for the remote door viewport so oversized UI trees no longer push trusted chrome off-screen.
- Added remote scroll offset tracking in the TUI model.
- Reset remote scroll to the top when opening a different door.
- Reduced inner component render widths so nested panel borders fit inside the remote viewport instead of wrapping and visually breaking.
- Added scroll controls:
  - `pgup` / `ctrl+u`
  - `pgdown` / `ctrl+d`
  - `home`
  - `end`
- Added a scroll position label to the remote viewport title when content exceeds the visible pane.
- Clipped the doors and remote-focus side panels so the whole app remains bounded by the terminal height.
- Preserved existing remote focus, input, and action routing behavior while making scroll independent of focus area.

### Tests and verification

- Added client tests for scroll clipping, scroll labels, and remote scroll clamping.
- Verified Go packages with `GOCACHE=/tmp/phosphornet-go-build GOMODCACHE=/tmp/phosphornet-go-mod go test ./...`.

## 2026-05-04 - Component Rendering And Interaction Model

This entry documents the client-side rendering and interaction pass that moves `phosphor` beyond flattening remote UI into text.

### Protocol and SDK UI components

- Extended `protocol.UINode` with typed fields for button actions, input values/placeholders, and grid rows.
- Added Go UI constructors for `button`, `input`, `textarea`, and `grid`.
- Added Python SDK helpers for `button`, `input`, `textarea`, `grid`, and `log`.

### Component-aware client rendering

- Replaced the flattened text renderer with a component-aware renderer in `internal/client/render.go`.
- Added local rendering for `screen`, `header`, `text`, `status`, `panel`, `menu`, `list`, `input`, `textarea`, `button`, `grid`, and `log`.
- Remote door content now renders in-place with styled panels, focused controls, grid blocks, log areas, and input fields.
- The sidebar now shows all focusable remote components rather than only the first menu.

### Interaction model

- Added remote focus tracking across multiple interactive components.
- Added per-component selection state for menus and lists.
- Added local input buffers for input and textarea components.
- Added routed events by component ID:
  - menus and buttons emit `action`
  - lists emit `select`
  - inputs and textareas emit `submit`
- Updated event messages to use the session ID received from the node instead of the previous hard-coded session ID.
- Preserved spacebar action activation for buttons/menus while allowing spaces to be typed inside inputs.

### Proof doors

- Updated `doors/lobby` to use focused buttons for actions.
- Updated `doors/chat` to use an input field, buttons, and a log component for room messages.
- Updated `doors/strategy_demo` to use a grid component and focused buttons for shared-game actions.

### Tests and verification

- Added client renderer tests covering focusable component discovery.
- Added client interaction tests covering submit/action routing for inputs, buttons, and menus.
- Verified Python files with `python3 -m py_compile`.
- Verified Go packages with `GOCACHE=/tmp/phosphornet-go-build GOMODCACHE=/tmp/phosphornet-go-mod go test ./...`.

## 2026-05-04 - Node Architecture, Presence, Broadcasts, And Proof Doors

This entry documents the second runtime implementation pass after the contract and scoped-state foundation.

### Node architecture split

- Refactored `internal/node/cli.go` back toward command/bootstrap responsibility.
- Added `internal/node/server.go` for WebSocket session orchestration.
- Added `internal/node/auth.go` for hello/challenge/auth handling.
- Added `internal/node/messages.go` for typed client message decoding and routing.
- Added `internal/node/doors.go` for door lifecycle dispatch.
- Added `internal/node/broadcast.go` for structured effect fanout.
- Added `internal/node/session.go` for live session tracking and presence snapshots.
- Added `internal/node/util.go` for node helper functions.

### Multiplayer primitives

- Added in-memory live session registry.
- Added session IDs generated per authenticated connection.
- Added implicit per-door room membership tracking.
- Added room and door presence snapshots in runtime context.
- Added room/door/user broadcast targeting.
- Added room/door/user/self notify targeting.
- Broadcast effects now trigger matching live sessions to re-render after shared state changes.
- Disconnects now invoke `on_leave` for the active door before session cleanup.

### Python SDK and proof doors

- Updated Python SDK effects so `ctx.effects.set_state`, `delete_state`, `clear_state`, and `replace_state` also update the in-memory context used for the returned view.
- Upgraded `doors/lobby` with station identity display, presence display, and join/leave notifications.
- Upgraded `doors/chat` into a room-scoped shared message log with presence, join/leave system messages, and broadcast re-rendering.
- Upgraded `doors/strategy_demo` into a room-scoped shared tactics proof with players, turn count, grid state, center-claim action, and broadcast re-rendering.

### Tests and verification

- Added node session registry tests for presence snapshots and room broadcast targeting.
- Verified Python files with `python3 -m py_compile`.
- Verified Go packages with `GOCACHE=/tmp/phosphornet-go-build GOMODCACHE=/tmp/phosphornet-go-mod go test ./...`.

## 2026-05-04 - Door Runtime Contract And Scoped State

This entry documents the first keystone runtime-contract implementation pass.

### Runtime contract

- Added `docs/PhosphorNet_runtime_contract.md` as the canonical MVP door runtime contract.
- Linked the runtime contract from `docs/PhosphorNet_architecture.md`.
- Added typed Go runtime contract models in `internal/protocol/runtime.go`.
- Standardized lifecycle names: `init`, `view`, `update`, `on_join`, `on_leave`, `tick`.
- Standardized UI event kinds: `action`, `select`, `submit`, `key`, `focus`.
- Added typed structured effects for `state_ops`, `broadcasts`, `notifies`, and `transitions`.
- Changed `protocol.UIEvent.Kind` from a loose string to the typed event-kind model.

### Runtime layer

- Refactored `internal/runtime/invoker.go` around canonical runtime request/response envelopes.
- Updated the Python invoker to send the full runtime envelope to the Python SDK shim.
- Updated `sdk/python/phosphornet/runtime.py` to consume the canonical envelope and return structured runtime responses.
- Added Python SDK effect helpers through `DoorEffects`.
- Preserved existing Python door compatibility by converting legacy `ctx.state` mutations into user-scope `replace` state ops.

### Scoped state

- Added first-class scoped state persistence for `user`, `room`, and `global` scopes.
- Added atomic multi-scope `state_ops` application in SQLite storage.
- Enforced admin/sysop-only writes for global scoped state.
- Added `migrations/002_scoped_state.sql` for normalized scoped door state.
- Kept legacy per-user door state readable while routing new writes through scoped state.

### Node integration

- Updated `phosphord` door dispatch to build typed runtime contexts with session, user, node, room, state, and permission metadata.
- Added lifecycle dispatch for `on_join` and `on_leave` around door switches.
- Applied returned `state_ops` through the storage layer.
- Applied self-targeted notify effects.

### Tests and verification

- Added runtime tests covering structured state-op responses from Python doors.
- Added storage tests covering multi-scope state writes and global-write permission rollback.
- Verified Python files with `python3 -m py_compile`.
- Verified Go packages with `GOCACHE=/tmp/phosphornet-go-build GOMODCACHE=/tmp/phosphornet-go-mod go test ./...`.

## 2026-05-03 - Initial Greenfield Implementation

This entry documents the full initial implementation pass that turned the repository from a docs-only workspace into a runnable PhosphorNet codebase scaffold.

### What existed before this implementation

- Architecture documentation in `docs/PhosphorNet_architecture.md`
- Technology stack documentation in `docs/PhosphorNet_technology_stack.md`
- `AGENTS.md`
- No application code, no Go module, no Python SDK, no doors, no migrations

### Repository structure created

The following top-level implementation directories were created to match the documented monorepo direction:

- `cmd/`
- `internal/`
- `sdk/`
- `doors/`
- `migrations/`

The following executable roots were created:

- `cmd/phosphor/`
- `cmd/phosphord/`
- `cmd/switchboard/`

The following internal packages were created:

- `internal/app/`
- `internal/client/`
- `internal/config/`
- `internal/identity/`
- `internal/knownnodes/`
- `internal/node/`
- `internal/protocol/`
- `internal/relay/`
- `internal/runtime/`
- `internal/storage/`

The following Python SDK and sample door directories were created:

- `sdk/python/phosphornet/`
- `doors/lobby/`
- `doors/chat/`
- `doors/strategy_demo/`

### Dependency and module setup

#### `go.mod`

Created the Go module declaration:

- module name: `phosphornet`
- Go version: `1.26`

Added direct module dependencies for the implementation:

- `github.com/coder/websocket`
- `github.com/pelletier/go-toml/v2`
- `github.com/spf13/cobra`
- `modernc.org/sqlite`

#### `go.sum`

Generated the Go checksum lockfile after resolving module dependencies with `go mod tidy`.

This records the exact dependency graph needed for the new implementation to build and test reproducibly.

### Go executable entrypoints implemented

#### `cmd/phosphor/main.go`

Implemented the `phosphor` executable entrypoint.

Behavior:

- delegates execution to `internal/client`
- prints CLI errors to stderr
- exits non-zero on failure

#### `cmd/phosphord/main.go`

Implemented the `phosphord` executable entrypoint.

Behavior:

- delegates execution to `internal/node`
- prints CLI errors to stderr
- exits non-zero on failure

#### `cmd/switchboard/main.go`

Implemented the `switchboard` executable entrypoint.

Behavior:

- delegates execution to `internal/relay`
- prints CLI errors to stderr
- exits non-zero on failure

### Shared application path helpers

#### `internal/app/paths.go`

Implemented reusable path helpers for PhosphorNet config and parent-directory creation.

Added:

- `ConfigDir()`
- `EnsureConfigDir()`
- `EnsureParentDir(path string)`
- `DefaultPassportPath()`
- `DefaultKnownNodesPath()`

Purpose:

- centralize default config locations
- support writing explicit alternate paths cleanly
- fix client bootstrap so non-default passport and known-node paths do not force writes into `~/.config/phosphornet`

### Identity and authentication implementation

#### `internal/identity/passport.go`

Implemented the first identity layer around Ed25519 passports.

Added `Passport` TOML model with:

- `display_name`
- `public_key`
- `private_key`
- `created_at`
- `schema_version`

Implemented:

- `Generate(displayName string)`
- `Load(path string)`
- `Save(path string, passport *Passport)`
- `(*Passport).PublicKeyBytes()`
- `(*Passport).PrivateKeyBytes()`
- `(*Passport).Fingerprint()`
- `EncodePublicKey(publicKey ed25519.PublicKey)`
- `DecodePublicKey(encoded string)`
- `Fingerprint(encodedPublicKey string)`
- `SignLogin(passport *Passport, payload LoginPayload)`
- `VerifyLogin(payload LoginPayload, signature string)`

Implemented login payload support with:

- `identity.LoginPayload`

Behavior added:

- Ed25519 keypair generation
- TOML serialization/deserialization of local passports
- base64-encoded key storage
- deterministic login payload signing
- Ed25519 signature verification
- human-facing public-key fingerprint generation

#### `internal/identity/passport_test.go`

Added an identity test covering:

- passport generation
- login payload signing
- signature verification success path

### Known-node persistence

#### `internal/knownnodes/store.go`

Implemented the initial SSH-like known-node store.

Added types:

- `KnownNodes`
- `Record`

Implemented:

- `Load(path string)`
- `Save(path string, store *KnownNodes)`
- `(*KnownNodes).Upsert(record Record)`
- `(*KnownNodes).Find(address string)`

Behavior added:

- TOML-backed storage of trusted node identities
- address-to-node-key pinning support
- update/replace behavior for explicit upserts

### Node configuration

#### `internal/config/node.go`

Implemented node configuration loading and saving.

Added `NodeConfig` fields:

- `name`
- `listen_addr`
- `node_id`
- `private_key`
- `doors_dir`
- `database`

Implemented:

- `DefaultNodeConfig()`
- `LoadNodeConfig(path string)`
- `SaveNodeConfig(path string, cfg NodeConfig)`

Behavior added:

- starter node configuration generation
- TOML-backed config loading
- default listen address, door root, and database path

### JSON protocol and UI types

#### `internal/protocol/types.go`

Implemented the initial shared wire protocol and declarative UI types.

Added protocol type constants for:

- `hello`
- `challenge`
- `auth`
- `auth_ok`
- `auth_denied`
- `door_list`
- `open_door`
- `render`
- `notify`
- `error`
- `event`

Added message structs:

- `HelloMessage`
- `ChallengeMessage`
- `AuthMessage`
- `AuthOKMessage`
- `AuthDeniedMessage`
- `DoorListMessage`
- `OpenDoorMessage`
- `RenderMessage`
- `ErrorMessage`
- `NotifyMessage`
- `EventMessage`

Added supporting structs:

- `DoorSummary`
- `UIEvent`
- `UINode`
- `Item`

Added UI builder helpers:

- `Screen`
- `Header`
- `Status`
- `Text`
- `Panel`
- `Menu`

Behavior added:

- explicit typed protocol surface instead of loose `map[string]any`
- shared render-tree representation for node and client code
- shared semantic message types for the first auth/render loop

### Door manifest runtime support

#### `internal/runtime/manifest.go`

Implemented door manifest loading.

Added:

- `DoorManifest`
- `LoadDoorManifest(path string)`
- `LoadDoorManifests(root string)`

Behavior added:

- TOML manifest parsing
- bulk scan of `doors/*/manifest.toml`
- deterministic sorting by door id

#### `internal/runtime/manifest_test.go`

Added a unit test covering:

- successful manifest load
- parsing of a minimal sample door manifest

### SQLite storage

#### `internal/storage/sqlite.go`

Implemented the first persistent storage layer using `modernc.org/sqlite`.

Added:

- `Store`
- `OpenSQLite(path string)`
- `(*Store).Close()`
- `(*Store).RecordUser(ctx, publicKey string)`

Implemented internal schema setup:

- `users` table creation

Behavior added:

- SQLite open/close lifecycle
- automatic schema bootstrap
- first-seen recording for authenticated public keys

### Node daemon CLI and server behavior

#### `internal/node/cli.go`

Implemented the first real `phosphord` command tree and WebSocket node behavior.

Added top-level command wiring:

- `Execute()`
- `NewRootCommand()`

Implemented `phosphord init`:

- generates a starter node identity using the same Ed25519 passport generation flow
- writes a TOML node config file
- populates `name`, `node_id`, and `private_key`

Implemented `phosphord serve`:

- loads node config
- loads door manifests from the configured door directory
- opens the SQLite store
- serves HTTP endpoints
- exposes `/ws`
- exposes `/healthz`

Added `Server` type with:

- config
- loaded door manifests
- SQLite store

Implemented WebSocket session flow in `handleWS`:

- accepts a WebSocket connection
- reads `hello`
- generates a random base64 nonce
- sends `challenge`
- reads `auth`
- validates login purpose
- validates node id, nonce, and client public key match
- verifies Ed25519 signature
- records the authenticated user in SQLite
- sends `auth_ok`
- sends `door_list`
- sends an initial `render` tree
- loops on `open_door`
- sends a follow-up `render` tree confirming the opened door

Added helpers:

- `randomNonce()`
- `stringsToUpper()`

Behavior added:

- first end-to-end node-side auth protocol
- first node-side render tree generation
- first node-side door discovery from manifests
- first placeholder per-door open flow

Current limitation documented by the implementation:

- Python door subprocess invocation is not yet connected into `phosphord`
- opened doors currently render a scaffold message rather than running `app.py`

### Client CLI and text renderer

#### `internal/client/cli.go`

Implemented the first `phosphor` command tree and connection flow.

Added top-level command wiring:

- `Execute()`
- `NewRootCommand()`

Implemented `phosphor passport create`:

- creates a local passport
- writes it to a configurable path
- prints the resulting fingerprint

Implemented `phosphor passport show`:

- reads a passport from disk
- prints its public key
- prints its fingerprint

Implemented `phosphor connect`:

- accepts configurable websocket address
- accepts configurable passport path
- accepts configurable known-node path
- normalizes and validates websocket URL
- loads or creates a passport
- loads known nodes
- dials the node over WebSocket
- sends `hello`
- reads `challenge`
- pins the node public key
- builds a typed `phosphornet.login.v1` payload
- signs the login payload
- sends `auth`
- reads `auth_ok`
- reads `door_list`
- reads the initial `render`
- prints the render tree
- automatically opens the first listed door
- reads and prints the follow-up `render`
- closes the WebSocket cleanly

Implemented helper functions:

- `ensurePassport(path string)`
- `pinNode(address, publicKey string, store *knownnodes.KnownNodes, path string)`
- `normalizeWebSocketURL(rawAddress string)`

Important path-handling behavior that was explicitly implemented and then corrected:

- the client now creates parent directories for explicit passport and known-node file paths
- it no longer assumes it must write into `~/.config/phosphornet` when alternate paths are supplied

#### `internal/client/render.go`

Implemented a minimal text-mode render interpreter for the JSON UI tree.

Added:

- `RenderText(node protocol.UINode)`
- internal recursive renderer `renderNode`

Behavior added:

- renders `screen`
- renders `header`
- renders `status`
- renders `text`
- renders `panel`
- renders `menu`

Current limitation:

- this is a plain text renderer, not the final Bubble Tea UI client
- it exists to prove the protocol flow and make the first smoke test visible

### Relay CLI scaffold

#### `internal/relay/cli.go`

Implemented the initial `switchboard` command tree as a service scaffold.

Added:

- `Execute()`
- `NewRootCommand()`
- `switchboard serve`

Behavior added:

- configurable listen address
- `/healthz` endpoint
- minimal runnable relay stub matching the documented executable layout

Current limitation:

- no relay frame forwarding or rendezvous logic yet

### Python SDK implementation

#### `sdk/python/phosphornet/__init__.py`

Implemented SDK exports for:

- `ui`
- `DoorContext`

#### `sdk/python/phosphornet/ui.py`

Implemented minimal declarative UI helpers:

- `screen(children)`
- `header(text)`
- `text(value)`
- `status(value)`
- `panel(title, children)`
- `menu(identifier, items)`
- `item(label, action="")`

Behavior added:

- Python-side construction of the documented JSON UI shape

#### `sdk/python/phosphornet/ctx.py`

Implemented a minimal `DoorContext` dataclass.

Added fields:

- `user`
- `state`

#### `sdk/python/phosphornet/runtime.py`

Implemented the initial Python door runtime scaffold.

Added:

- `load_module(path: str)`
- `invoke(entry_path: str, method: str, ctx: dict, event: dict | None = None)`
- async `main()`

Behavior added:

- dynamic loading of a door `app.py`
- invocation of a requested coroutine method
- single-request JSON stdin/stdout runtime path

Current limitation:

- the Go node does not yet call this runtime
- it is a ready scaffold for the next runtime integration step

### Sample doors implemented

#### `doors/lobby/manifest.toml`

Implemented the `lobby` door manifest:

- `id = "lobby"`
- `name = "Lobby"`
- `entry = "app.py"`

#### `doors/lobby/app.py`

Implemented the first lobby door scaffold.

Behavior:

- renders a `LOBBY` header
- renders a welcome panel
- explains that it is the default station landing door

#### `doors/chat/manifest.toml`

Implemented the `chat` door manifest:

- `id = "chat"`
- `name = "Chat"`
- `entry = "app.py"`
- `permissions = ["shared_state"]`

#### `doors/chat/app.py`

Implemented the first chat door scaffold.

Behavior:

- renders a `CHAT` header
- renders a room panel
- explains that real-time broadcast wiring still needs implementation
- shows a status line that `shared_state` is declared

#### `doors/strategy_demo/manifest.toml`

Implemented the `strategy_demo` door manifest:

- `id = "strategy_demo"`
- `name = "Iron Orchard"`
- `entry = "app.py"`
- `permissions = ["raw_keys", "shared_state"]`

#### `doors/strategy_demo/app.py`

Implemented the first strategy demo door scaffold.

Behavior:

- renders an `IRON ORCHARD` header
- renders a strategy demo panel
- explains that this door is the placeholder for the 10x10 grid/game proof
- shows a status line that `raw_keys` is declared for future controls

### Migration added

#### `migrations/001_init.sql`

Added the first SQL migration file.

Contents:

- `users` table creation

Purpose:

- document the intended schema independently of runtime bootstrap
- match the documented migration-based direction

### Verification and testing implemented

#### Go formatting

Ran `gofmt` across the Go sources created in `cmd/` and `internal/`.

#### Python syntax verification

Verified the Python SDK and sample doors with `python -m py_compile`.

#### Go tests

Ran `go test ./...` successfully after:

- redirecting `GOCACHE` and `GOMODCACHE` to writable paths under `/tmp`
- resolving Go module dependencies

#### End-to-end smoke test

Executed a real local smoke test of the new implementation:

- ran `phosphord init`
- ran `phosphord serve`
- ran `phosphor connect`

Observed successful behavior:

- node config generation
- node startup
- WebSocket handshake
- challenge-response authentication
- node-key pinning
- door list transmission
- initial render transmission
- `open_door` roundtrip
- follow-up render output

### Runtime-generated local artifacts created during verification

These were created by running the new implementation, not by hand-authoring source:

- `phosphornet.db`
  Created by the SQLite-backed node smoke test.
- `doors/*/__pycache__/...`
  Created by Python bytecode compilation during verification.
- `sdk/python/phosphornet/__pycache__/...`
  Created by Python bytecode compilation during verification.

These artifacts are part of the local workspace state from implementation verification, not part of the hand-written source design.

### Current implemented scope summary

The initial implementation now provides:

- a real Go module
- runnable `phosphor`, `phosphord`, and `switchboard` executables
- typed protocol structs
- Ed25519 passport generation and login signing
- known-node pinning
- TOML node config
- manifest-based door discovery
- SQLite-backed user recording
- a minimal text renderer for the declarative UI
- a Python SDK scaffold
- three sample door definitions
- a migration file
- passing tests and a successful local smoke test

### Known limitations after this implementation

The following items were intentionally left incomplete:

- Bubble Tea TUI rendering is not yet implemented
- Python door subprocesses are not yet invoked by `phosphord`
- node-side `open_door` currently renders a scaffold message instead of the Python door output
- `switchboard` does not yet relay traffic
- richer event handling, state API, and session lifecycle are still future work

## 2026-05-03 - MVP Slice: Bubble Tea Client + Generic Door Invoker + Python Door Opens

This entry documents the next MVP implementation slice focused on three previously known gaps:

- Bubble Tea TUI rendering
- Python door subprocess invocation from `phosphord`
- replacing the scaffolded `open_door` path with real Python door renders

It also includes an architectural refinement requested during implementation:

- the door invoker layer must be generic enough to support future interpreters such as Lua

### Dependency changes

#### `go.mod`

Added a new direct dependency:

- `github.com/charmbracelet/bubbletea`

Purpose:

- move `phosphor connect` from plain stdout rendering to a real Bubble Tea-driven terminal UI

#### `go.sum`

Refreshed the module lockfile after fetching Bubble Tea and its transitive dependencies.

This brought in the checksum entries needed for:

- Bubble Tea
- its transitive Charm dependencies
- terminal helpers used by the Bubble Tea runtime

### Generic door invoker abstraction

#### `internal/runtime/invoker.go`

Replaced the previous Python-only invocation entrypoint with a generic invoker abstraction.

Added:

- `DoorContextData`
- `Invoker` interface
- invoker registry map
- `InvokeDoorView(...)`
- `resolveInvoker(...)`
- `normalizeRuntimeName(...)`

Behavior added:

- door execution now dispatches through a runtime registry rather than hard-coding Python at the node call site
- manifest runtime names can explicitly choose an interpreter backend
- runtime selection can also fall back to file extension inference

Current runtime selection behavior:

- explicit manifest runtime wins
- `.py` implies `python`
- `.lua` implies `lua`
- unknown/no extension currently defaults to `python`

Architectural purpose:

- allow future backends such as Lua to be added by registering another `Invoker`
- keep `phosphord` generic with respect to the door interpreter

#### `internal/runtime/python_invoker.go`

Moved the Python-specific subprocess execution into its own backend implementation.

Added:

- `PythonInvoker`
- internal `pythonDoorRequest`
- `PythonInvoker.InvokeView(...)`
- `resolveDoorEntryPath(...)`
- `sdkRuntimePath()`
- `sdkPythonRoot()`

Behavior added:

- resolves a door entry path from the manifest and doors root
- finds `python3`
- runs the Python SDK runtime script as a subprocess
- passes a JSON request on stdin
- sets `PYTHONPATH` to the SDK root
- parses the resulting JSON view into `protocol.UINode`
- returns detailed stderr-backed errors when door execution fails

This separated:

- generic interpreter dispatch
- Python-specific subprocess mechanics

#### `internal/runtime/manifest.go`

Extended `DoorManifest` with:

- `Runtime string \`toml:"runtime"\``

Purpose:

- make interpreter choice explicit in door manifests
- support future non-Python door backends cleanly

#### `internal/runtime/invoke_test.go`

Updated the runtime invocation test to exercise the manifest with an explicit:

- `Runtime: "python"`

This keeps the test aligned with the new generic invoker model.

### Python SDK runtime improvements

#### `sdk/python/phosphornet/runtime.py`

Updated the Python runtime script to better support packaged execution and richer door contexts.

Changes made:

- added `sys.path` insertion for the SDK root so `from phosphornet import ui` works reliably when the runtime is executed as a script
- imported and constructed `DoorContext`
- changed stdin handling from `input()` to `sys.stdin.read()`

Behavior change details:

- doors now receive a `DoorContext` object instead of a raw dict
- the runtime works correctly when launched as an external interpreter process by Go
- larger/more flexible JSON requests can be read without relying on single-line input behavior

### Node runtime integration

#### `internal/node/cli.go`

Substantially upgraded the node behavior to use the new door invoker path.

Changes made to startup and auth flow:

- kept the existing auth and door-list flow
- removed the server-side static menu render as the only first render path
- added initial lobby rendering through the Python door invoker when a `lobby` door exists
- retained a Go fallback render when the lobby door cannot be invoked

New helper methods added:

- `defaultLobbyView(...)`
- `findDoor(id string)`
- `invokeDoorView(...)`

Behavior changes:

- the initial post-auth `render` now comes from the Python `lobby` door when available
- `open_door` now looks up the requested manifest and invokes that door through the generic runtime layer
- unknown doors now produce a protocol error
- Python door invocation failures now produce a protocol error or warning instead of silently falling back to a scaffolded fake door

Additional details:

- `invokeDoorView(...)` resolves the configured `doors_dir`
- it uses a timeout for door invocation
- it passes user-oriented context values such as:
  - `public_key`
  - `fingerprint`
  - `node_name`

Most important outcome:

- `phosphord` no longer uses the placeholder “Door runtime wiring is scaffolded” render for successful `open_door` flows
- the node now returns the actual Python door’s JSON UI

### Bubble Tea client implementation

#### `internal/client/cli.go`

Refactored `phosphor connect` away from the previous plain text one-shot output flow.

Removed behavior:

- immediate printing of auth success and render text to stdout
- automatic fire-once open of the first door followed by synchronous printout

Added behavior:

- after handshake and initial messages, `phosphor` now creates a Bubble Tea program
- starts a background WebSocket read loop
- sends incoming protocol messages into the Bubble Tea program with `Program.Send`
- exits cleanly when the user quits

Added:

- `readLoop(...)`

Behavior of `readLoop(...)`:

- reads raw protocol messages from the live WebSocket connection
- converts them into Bubble Tea messages
- forwards them into the running program
- exits on connection close or read error

#### `internal/client/tui.go`

Added the first real Bubble Tea TUI implementation for PhosphorNet.

New types added:

- `errMsg`
- `connectionClosedMsg`
- `doorOpenedMsg`
- `tuiModel`

New functions added:

- `newTUIModel(...)`
- `(tuiModel) Init()`
- `(tuiModel) Update(...)`
- `(tuiModel) View()`
- `openDoorCmd(...)`
- `readRawMessage(...)`
- `preferredDoorIndex(...)`

Bubble Tea behaviors implemented:

- alt-screen terminal UI
- keyboard navigation with:
  - `up`
  - `down`
  - `j`
  - `k`
  - `enter`
  - `space`
  - `q`
  - `ctrl+c`
- external message injection from the WebSocket read goroutine
- live current-view replacement on incoming `render`
- surfacing protocol `notify` and `error` messages
- connection-close handling

UI shape implemented:

- trusted local header showing:
  - node name
  - node id
  - user fingerprint
- local door list selector
- remote render viewport
- local status/help footer

Important UX behavior:

- the selected door defaults to `lobby` if present
- the client opens the selected door on `enter`
- the remote viewport updates when the node sends a new `render`

This is still intentionally simple, but it is now a real TUI event loop rather than a print-and-exit command.

#### `internal/client/render.go`

Kept the text renderer and reused it inside the Bubble Tea viewport.

Its role changed from:

- the entire client output mechanism

to:

- the renderer for the remote viewport inside the Bubble Tea screen

### Sample door manifest updates

The sample door manifests were updated to make interpreter choice explicit.

#### `doors/lobby/manifest.toml`

Added:

- `runtime = "python"`

#### `doors/chat/manifest.toml`

Added:

- `runtime = "python"`

#### `doors/strategy_demo/manifest.toml`

Added:

- `runtime = "python"`

Purpose:

- make the current backend choice obvious
- show how future interpreter-specific doors could declare another runtime such as Lua

### Verification performed

#### Formatting

Ran `gofmt` across the updated Go sources.

#### Python verification

Ran:

- `python -m py_compile` for the SDK runtime and sample doors

This confirmed the Python runtime changes and door files remain syntactically valid.

#### Go verification

Ran:

- `go mod tidy`
- `go test ./...`

Result:

- full Go test suite passed after refreshing the module graph

#### Live end-to-end smoke test

Ran a live local socket smoke test with the updated implementation.

Verified steps:

- started `phosphord`
- connected with the Bubble Tea `phosphor` client
- confirmed the initial render came from the Python `lobby` door
- opened `chat` from the Bubble Tea door list
- confirmed the `chat` door render came from the Python invoker path

Observed outcomes:

- Bubble Tea screen rendered correctly
- lobby content displayed from `doors/lobby/app.py`
- `chat` content displayed from `doors/chat/app.py`
- `open_door` no longer returned the old scaffold text

### Effect on previously known limitations

These previously documented gaps are now addressed:

- Bubble Tea TUI rendering is now implemented
- Python door subprocesses are now invoked by `phosphord`
- node-side `open_door` now returns Python door output instead of the scaffold message

### Remaining limitations after this MVP slice

The following items still remain open:

- the Bubble Tea client currently uses a simple text viewport rather than richer Lip Gloss layout/styling
- the node still only invokes the `view` lifecycle and does not yet route door `update` events
- there is no persisted per-door state bridge yet
- `switchboard` still remains a stub
- a Lua backend is not implemented yet, only the generic invoker shape that will allow it later

## 2026-05-03 - MVP Slice: Lip Gloss Layout + Door Update Routing + Persisted Door State

This entry documents the next MVP slice aimed at the three remaining gaps from the previous section:

- richer Lip Gloss layout/styling in the Bubble Tea client
- node-side routing of door `update` events
- a persisted per-door state bridge

### Runtime contract changes

#### `internal/runtime/invoker.go`

Extended the generic runtime layer so invokers can return both a rendered UI tree and updated state.

Added:

- `DoorResponse`

Changed:

- `Invoker` interface now exposes a generic `Invoke(...)` method
- `InvokeDoorView(...)` now returns `DoorResponse` instead of a bare `protocol.UINode`
- added `InvokeDoorUpdate(...)`

Behavior added:

- runtime backends can now support both `view` and `update`
- runtime backends can now send mutated state back to the node for persistence
- the node remains interpreter-agnostic while supporting richer lifecycle methods

#### `internal/runtime/python_invoker.go`

Updated the Python backend to support the richer runtime contract.

Changes made:

- `PythonInvoker` now implements the generic `Invoke(...)` contract
- the Python request now carries:
  - `method`
  - `ctx`
  - optional `event`
- the Go side now decodes a full `DoorResponse`

Behavior added:

- Python doors can now participate in both `view` and `update`
- Python doors can now return mutated state through the shared runtime contract

### Python runtime response contract

#### `sdk/python/phosphornet/runtime.py`

Changed the Python runtime result shape from:

- plain rendered UI only

to:

- `{"view": ..., "state": ...}`

Behavior added:

- builds `DoorContext`
- invokes either `view` or `update`
- always returns the current `ctx.state` alongside the resulting view

Architectural outcome:

- interpreters can remain responsible for door lifecycle execution
- the node can remain responsible for persistence and session orchestration

### Persisted per-door state bridge

#### `internal/storage/sqlite.go`

Extended the SQLite schema and storage API for door state persistence.

Added schema:

- `door_state`

Schema fields:

- `door_id`
- `scope_key`
- `value_json`

Added methods:

- `LoadDoorState(ctx, doorID, scopeKey string)`
- `SaveDoorState(ctx, doorID, scopeKey string, state map[string]any)`

Behavior added:

- per-door state can now be loaded from SQLite before invoking a door
- per-door state can now be written back to SQLite after `view` or `update`
- state is stored as JSON blobs

Current scoping model:

- state is keyed by `door_id` plus the authenticated user's public key

This gives each user their own persisted state per door.

### Node-side event routing and state persistence

#### `internal/node/cli.go`

Upgraded `phosphord` from a view-only door opener to a session-aware door lifecycle router.

Added:

- `sessionState`
- `readClientMessage(...)`
- `invokeDoorUpdate(...)`

Changed connection/session behavior:

- the server now tracks the active door for the current connection
- incoming client messages are decoded by type instead of assuming everything is `open_door`
- supported client message types now include:
  - `open_door`
  - `event`

Changed door invocation flow:

- `invokeDoorView(...)` now:
  - loads persisted state from SQLite
  - invokes the door with that state
  - saves the returned state back to SQLite
  - returns the rendered `view`

- `invokeDoorUpdate(...)` now:
  - loads persisted state from SQLite
  - invokes the door `update` lifecycle with a `UIEvent`
  - saves the returned state back to SQLite
  - returns the rendered `view`

Behavior added:

- the node now routes `protocol.EventMessage` to the active door’s `update` lifecycle
- door state survives beyond one render and beyond one connection
- client actions can now mutate persisted door data through the runtime bridge

### Styled Bubble Tea client

#### `internal/client/tui.go`

Substantially upgraded the client UI from a plain text screen to a richer Lip Gloss layout.

Added imports and usage:

- `github.com/charmbracelet/lipgloss`

Added new TUI concepts:

- `eventSentMsg`
- `focusArea`
- `focusDoors`
- `focusRemoteActions`
- `remoteMenu`

Extended `tuiModel` with:

- focus tracking
- remote action menu state
- selected remote action index
- width/height from `tea.WindowSizeMsg`

New behavior added:

- local focus can switch between:
  - the hosted door rail
  - the current remote door’s first menu
- `tab`, `left/right`, and `h/l` change focus
- `up/down` navigate within the focused pane
- `enter` on the remote actions pane sends a `protocol.EventMessage`
- `tea.WindowSizeMsg` is handled for layout sizing

Added:

- `sendEventCmd(...)`
- `firstMenu(...)`
- `maxInt(...)`

Visual changes made:

- styled app shell
- styled metadata header
- rounded bordered panels
- highlighted focused panel
- separated left and right columns
- dedicated “Doors” panel
- dedicated “Remote Actions” panel
- dedicated “Remote View” panel
- styled status and error footer

Result:

- the client no longer looks like a plain text dump
- the remote view remains text-rendered, but it now lives inside a styled terminal layout

### Sample doors upgraded to use update + persisted state

The sample doors were changed from static renderers into minimal stateful doors that exercise the new bridge.

#### `doors/lobby/app.py`

Added behavior:

- reads persisted `visit_count`
- displays the current visit count
- exposes a remote menu with:
  - `Record Visit`
  - `Reset Visit Counter`
- `update(...)` now mutates `ctx.state`

Result:

- the lobby now proves persisted state and update routing

#### `doors/chat/app.py`

Added behavior:

- reads persisted `ping_count`
- displays the current ping count
- exposes a remote menu with:
  - `Ping Room`
  - `Clear Ping Counter`
- `update(...)` now mutates `ctx.state`

Result:

- the chat door now proves a simple stateful interaction loop

#### `doors/strategy_demo/app.py`

Added behavior:

- reads persisted `turn`
- displays the current turn counter
- exposes a remote menu with:
  - `End Turn`
  - `Reset Turn Counter`
- `update(...)` now mutates `ctx.state`

Result:

- the strategy demo now proves a minimal turn-style state transition

### Verification performed

#### Formatting

Ran `gofmt` across the updated Go files.

#### Python verification

Ran Python bytecode compilation again for:

- `sdk/python/phosphornet/*.py`
- `doors/lobby/app.py`
- `doors/chat/app.py`
- `doors/strategy_demo/app.py`

This also produced additional local `__pycache__` artifacts for Python 3.14 in the workspace.

#### Go verification

Ran:

- `go test ./...`

Result:

- full Go test suite passed after the runtime/state/event changes

#### Live end-to-end verification

Ran a live local verification against `phosphord` and the Bubble Tea client.

Verified flow:

1. started `phosphord`
2. connected with `phosphor`
3. focused the remote action pane in `lobby`
4. triggered `Record Visit`
5. disconnected
6. reconnected
7. confirmed the lobby now rendered `Recorded visits: 1`

What this proved:

- the Bubble Tea client sent a real `event`
- `phosphord` routed that event to the door `update` lifecycle
- the door mutated `ctx.state`
- the node persisted the returned state in SQLite
- reconnecting reloaded that persisted state into the door

### Effect on the previously remaining limitations

These previously listed gaps are now addressed:

- the Bubble Tea client no longer uses only a simple text viewport; it now has a richer Lip Gloss-based shell and pane layout
- the node now routes door `update` events
- there is now a persisted per-door state bridge

### Remaining limitations after this slice

The following items still remain open:

- the remote viewport still renders door content as text rather than richer component-specific widgets
- only the first remote `menu` is surfaced as an actionable remote control pane
- there is still no richer multi-user state/broadcast model for doors such as chat
- `switchboard` still remains a stub
- a Lua backend is still not implemented, though the generic invoker layer is now in better shape for it

## 2026-05-03 - CLI Testing Ergonomics: Quick Connect + Clear Passport Validation

This entry documents a small but important CLI ergonomics pass prompted by a real test failure:

- passing a known-nodes TOML file as `--passport` produced a confusing `invalid private key length: 0` error
- local self-testing still required too much manual path management

### Quick self-test paths

#### `internal/app/paths.go`

Added quick-testing path helpers:

- `QuickTestDir()`
- `QuickTestPassportPath()`
- `QuickTestKnownNodesPath()`

Behavior added:

- PhosphorNet now has a conventional temp-directory location for quick local testing
- quick testing defaults live under:
  - `/tmp/phosphornet-quick/passport.toml`
  - `/tmp/phosphornet-quick/known_nodes.toml`

### Passport validation hardening

#### `internal/identity/passport.go`

Added:

- `(*Passport).Validate()`

Changed:

- `Load(path string)` now validates the parsed passport before returning it

Validation now checks:

- `public_key` must be present
- `private_key` must be present
- `public_key` must decode correctly
- `private_key` must decode correctly

Behavior change:

- invalid or wrong-kind TOML files now fail much earlier and with clearer errors
- using a known-nodes file or another unrelated TOML file as `--passport` no longer falls through to a late private-key failure during signing

Example improvement:

- before: `invalid private key length: 0`
- now: an invalid passport file error that points at the actual missing passport fields

#### `internal/identity/passport_test.go`

Added a test for invalid passport validation:

- rejects a passport missing `private_key`

### Quick connect mode

#### `internal/client/cli.go`

Added a new `phosphor connect` flag:

- `--quick`

Behavior added:

- `phosphor connect --quick` automatically uses:
  - `app.QuickTestPassportPath()`
  - `app.QuickTestKnownNodesPath()`
- the client prints which temp files it is using
- if the quick-test passport is missing, it is auto-created through the existing passport bootstrap path

Purpose:

- make local self-testing much faster
- avoid the need to manually spell out temp file locations every time
- reduce user error around mixing up passport and known-nodes file paths

### Verification performed

Ran:

- `gofmt`
- `go test ./...`

Result:

- tests passed after the validation and quick-connect additions

## 2026-05-03 - MVP File-Level Roadmap Documentation

This entry documents a planning artifact added to keep the next implementation phase in-tree and reviewable.

### New roadmap document

#### `docs/PhosphorNet_MVP_implementation_roadmap.md`

Added a dedicated file-level implementation roadmap for the current MVP direction.

The new document records the agreed next-phase architecture decisions, including:

- support for both single-user and multiplayer doors
- a strict cross-interpreter door API
- Python-first implementation with future interpreter compatibility in mind
- canonical JSON stdin/stdout request-response runtime behavior
- first-class scoped state for:
  - `user`
  - `room`
  - `global`
- one implicit room per door for MVP
- sysop/admin-only global writes
- lifecycle targets:
  - `init`
  - `view`
  - `update`
  - `on_join`
  - `on_leave`
  - `tick`
- structured side effects including:
  - `state_ops`
  - `broadcasts`
  - `notifies`
  - `transitions`

It also maps the planned work onto concrete repository files, covering:

- `internal/protocol/`
- `internal/runtime/`
- `internal/storage/`
- `internal/node/`
- `internal/client/`
- `sdk/python/phosphornet/`
- `doors/`
- additional planned docs

### Purpose of the roadmap

This file was added so future implementation can follow a durable in-repo plan rather than relying on chat history.

It is intended to guide the next phases of:

- canonical runtime contract work
- scoped state and broadcast support
- multiplayer-capable door runtime primitives
- richer client rendering
- end-to-end proof doors for `lobby`, `chat`, and `strategy_demo`

## 2026-05-03 - Repository Git Ignore

This entry documents the addition of a repository-level ignore file so local development artifacts do not get tracked accidentally.

### New ignore file

#### `.gitignore`

Added a root `.gitignore` tuned to the artifacts this repository already produces during local development and verification.

The ignore rules now cover:

- local Codex metadata:
  - `.codex/`
  - `.agents/`
- Python bytecode and virtual environments:
  - `__pycache__/`
  - `*.py[cod]`
  - `*.pyo`
  - `.venv/`
  - `venv/`
  - `.hatch-pet-venv/`
- Go build and test outputs:
  - `bin/`
  - `dist/`
  - `*.test`
  - `coverage.out`
- local runtime data:
  - `*.db`
  - `*.db-shm`
  - `*.db-wal`
  - `*.log`
- generated project output:
  - `output/`
- OS/editor noise:
  - `.DS_Store`

### Purpose

This change keeps local-only state such as SQLite databases, Python cache files, generated pet output, and tool metadata from polluting version control as implementation continues.
