# PhosphorNet Test Index

## Purpose

This document maps PhosphorNet's current release-confidence checklist to the tests that protect it.

Use it before adding broad new integration suites. If an item is already covered here, prefer filling a specific gap over duplicating the same behavior in a new directory.

## How To Read This

- **Websocket integration** means a test drives a live in-process node websocket session with real protocol messages, temporary doors, Ed25519 auth, and SQLite-backed state.
- **Package coverage** means a focused package test covers the invariant at the owning layer.
- **Gap** means the behavior has coverage, but not yet at the most useful end-to-end layer or not for the most important negative path.

There is currently no top-level `tests/` directory. The broad integration suite lives in `internal/node/integration_test.go`.

## Release Confidence Matrix

| Promise | Current coverage | Status |
|---|---|---|
| Auth handshake | `internal/node/integration_test.go`: `TestIntegrationCompatibleClientAuthRendersLobbyAndPersistsState`; `internal/client/cli_test.go`: `TestVerifyServerChallengeRequiresValidNodeSignature` | Covered |
| Incompatible-client handling | `internal/node/integration_test.go`: `TestIntegrationCompatibleClientAuthRendersLobbyAndPersistsState`; `internal/protocol/compatibility_test.go` | Covered |
| Node key pinning | `internal/client/cli_test.go`: `TestPinNodeRejectsChangedKnownNodeKey`, `TestPinNodeCanReplaceChangedKnownNodeKey`, `TestBuildTrustSummaryRejectsChangedKnownStationIdentity` | Covered at client layer |
| First-connect trust behavior | `internal/client/cli_test.go`: `TestBuildTrustSummarySeparatesSelfSignedTLSFromStationIdentity`, `TestTrustPromptTUIRequiresExplicitAcceptance` | Covered at client layer |
| Manifest rejection | `internal/runtime/manifest_test.go`: invalid runtime, missing stdio isolation image, symlink escape, duplicate IDs, invalid settings, bundled manifest loading | Covered at manifest layer |
| UI contract validation | `internal/protocol/ui_contract_test.go`, `internal/runtime/ui_contract_test.go`, `internal/client/ui_contract_test.go`; `internal/node/integration_test.go`: `TestIntegrationBadRuntimeOutputReturnsTypedError` | Covered |
| UI sanitization | `internal/client/render_test.go`: hostile string sanitization, unknown/deep tree handling, style stripping from leaf components | Covered at client layer |
| Door effect authorization | `internal/node/capabilities_test.go`; `internal/node/integration_test.go`: `TestIntegrationStateWriteRequiresManifestCapability`, `TestIntegrationCaptureKeysDoorReceivesKeyEvents`, admin and moderation flows | Covered, with some negative paths at package layer |
| Scoped state atomicity | `internal/storage/sqlite_test.go`: `TestScopedStateOpsAreAtomicAcrossScopes` plus invalid-key, oversized-value, and oversized-batch rejection | Covered at storage layer |
| Malformed stdio output | `internal/runtime/invoke_test.go`: `TestStdioInvokerRejectsMalformedJSON`, stdout cap and timeout tests; `internal/node/integration_test.go`: `TestIntegrationBadRuntimeOutputReturnsTypedError` | Covered |
| Lua sandbox escape attempts | `internal/runtime/invoke_test.go`: `TestLuaSandboxDoesNotOpenOSByDefault`, `TestLuaSandboxIgnoresUnsafeProfileLibraries` | Covered at runtime layer |
| Podman isolation policy | `internal/runtime/manifest_test.go`; `internal/runtime/invoke_test.go`: Podman defaulting, missing image rejection, hardened argv, malformed JSON, timeout, stderr cap | Covered at runtime layer |
| Broadcast fanout | `internal/node/integration_test.go`: `TestIntegrationRoomNotificationFanout`, `TestIntegrationBroadcastRerendersRoomStateForPeers`; `internal/node/session_test.go` | Covered |
| Admin op authorization | `internal/node/capabilities_test.go`: missing admin capability and moderation capability rejection; `internal/node/integration_test.go`: successful admin policy, settings, maintenance, reload, rate-limit flows | Covered, with negative authorization mostly at package layer |
| Audit logs | `internal/node/audit_test.go`; `internal/storage/sqlite_test.go`; `internal/node/integration_test.go`: `TestIntegrationAuditEventsCaptureAuthDenialAndAdminChange`, `TestIntegrationAuditMaxBytesTrimsSQLiteEvents` | Covered |
| Moderation policy | `internal/node/access_test.go`; `internal/node/integration_test.go`: ban disconnect and reconnect denial, mute enforcement with navigation carve-out, per-user event/open-door rate limits | Covered |

## Useful Next Gaps

These are the highest-signal additions if the project needs more release confidence.

- Add a websocket negative test for an admin door that has `access = "admin"` but lacks the specific `admin:*` capability, proving privileged admin effects are denied end to end.
- Add a websocket or command-level test for manifest reload rejection when a newly added manifest is invalid, proving live nodes do not partially accept bad door inventory.
- Add an end-to-end client connect test around first-connect known-node pinning if the client connection code becomes easier to drive without a full terminal UI.
- Add a websocket scoped-state atomicity test only if node-level lifecycle scheduling semantics become part of the compatibility promise. The storage transaction invariant is already covered.

## Test Commands

For the current workspace, use the sandbox-friendly Go cache path:

```sh
GOCACHE=/home/nand/phosphornet/.gocache go test ./...
```

For just the high-impact websocket integration suite:

```sh
GOCACHE=/home/nand/phosphornet/.gocache go test ./internal/node -run Integration -count=1
```
