# PhosphorNet Public Station Moderation

## Purpose

Public stations need a small survival kit before they need reputation, federation-wide identity scores, public blocklists, or social governance systems.

This document defines the boring node-owned moderation primitives that let a station operator keep a public node usable without turning PhosphorNet into a social network platform.

Core rule:

```text
Moderation is local station policy.
Ed25519 keys identify subjects.
Doors may expose moderation UI.
phosphord owns enforcement.
```

## Boundaries

Basic public-station moderation is in scope:

- stop a known key from entering
- stop a known key from posting or acting noisily
- hide or delete abusive user content
- freeze a problematic door
- publish a maintenance or incident notice
- rate-limit abusive behavior
- inspect recent abuse-relevant activity
- support explicit unban/unmute actions and operator moderation notes

Advanced social systems are outside the local station moderation contract:

- global reputation
- cross-station ban federation
- public blocklist subscriptions
- moderator elections
- trust scores
- identity escrow
- automated content classification
- opaque anti-spam scoring

## Moderation Subjects

The primary moderation subject is the user's passport public key.

Station operators may also display or search by fingerprint, display name, and session metadata, but enforcement should resolve to a stable public key whenever possible.

Display names are not moderation identities. They are mutable local profile data.

## Primitive Set

| Primitive | Owner | Current behavior | Status |
|---|---|---|---|
| Ban key | `phosphord` station policy | Deny future auth/open access for a public key and disconnect active sessions when applied. | Implemented as node-owned station policy through `ban_key` / `unban_key`. |
| Mute key | `phosphord` station policy plus door cooperation | Keep the user admitted but reject generic write-like events while allowing navigation; expose `ctx.permissions.muted` to doors. | Implemented as node-owned station policy through `mute_key` / `unmute_key`. |
| Hide user content | Door state through admin/sysop moderation controls | Mark content hidden while preserving enough metadata for review and audit. | Implemented in the bundled forum door for posts. |
| Delete user content | Door state through admin/sysop moderation controls | Remove content when preservation is not needed or not safe. Deletion should be explicit and logged. | Implemented in the bundled forum door for posts. |
| Freeze door | `phosphord` station policy | Disable a door so non-admin users cannot see or open it while admins can still inspect policy. | Implemented as door enabled/disabled policy through Station Admin. |
| Maintenance notice | `phosphord` station policy and notifications | Broadcast or display station-level notice text and maintenance mode state. | Implemented through station notices, maintenance mode, and notifications. |
| Rate-limit user | `phosphord` session/event policy | Bound event and open-door abuse by public key and session. | Implemented as per-user event/open-door limits through `set_user_rate_limit`. |
| Inspect recent activity | `phosphord` event/audit stream | Show recent auth, admin, moderation, door-open, error, and rate-limit events with key/fingerprint context. | Implemented as in-memory station events in Station Admin. Durable audit policy remains separate work. |
| Review/unban path | Operator process | Record operator notes and explicitly unban or unmute a key after review. | Implemented through unban/unmute and moderation notes; any user contact path is station policy. |

## Policy Shape

Moderation policy should live with node-owned station policy, not with arbitrary door state.

Station-policy field shape:

```json
{
  "moderation": {
    "banned_keys": {
      "ed25519:...": {
        "reason": "spam",
        "created_by": "ed25519:admin...",
        "created_at": "2026-05-14T00:00:00Z",
        "expires_at": ""
      }
    },
    "muted_keys": {
      "ed25519:...": {
        "reason": "flooding chat",
        "created_by": "ed25519:admin...",
        "created_at": "2026-05-14T00:00:00Z",
        "expires_at": ""
      }
    },
    "rate_limits": {
      "ed25519:...": {
        "events_per_minute": 20,
        "opens_per_minute": 6
      }
    }
  }
}
```

Notes:

- Empty `expires_at` means indefinite.
- Reasons are operator-facing text, not trusted proof.
- The station should log moderation changes as admin actions.
- The station should avoid exposing public keys in ordinary user-facing door content unless an admin view specifically needs them.

## Admin Operations

Node-owned admin ops use explicit names rather than overloading roles:

| Admin op | Purpose |
|---|---|
| `ban_key` | Add or update a station deny entry for a public key. |
| `unban_key` | Remove a station deny entry. |
| `mute_key` | Add or update a station mute entry for a public key. |
| `unmute_key` | Remove a station mute entry. |
| `set_user_rate_limit` | Set or clear per-user station rate-limit overrides. |
| `record_moderation_note` | Add operator context without changing enforcement. |

These operations require an admin/sysop role and the dedicated `admin:moderate_users` capability.

The existing `set_user_role` operation is not a ban system. Enforcement remains an explicit denylist.

## Door Responsibilities

Doors may expose moderation controls, but they should not define station-wide moderation authority.

Doors are responsible for:

- preserving enough author metadata to identify abusive content
- supporting hide/delete where the door stores user-generated content
- checking `ctx.permissions.muted` before accepting post-like updates
- using typed effects and admin ops instead of direct policy mutation

Doors are not responsible for:

- deciding whether a banned user may authenticate
- editing station-wide deny/mute policy directly
- rate-limiting the WebSocket/session itself
- enforcing global appeal or unban process

## Inspection And Logging

The first useful inspection view should answer:

- Which key acted?
- Which display name was shown at the time?
- Which session and door were involved?
- What action was attempted?
- Was the action accepted, rejected, rate-limited, hidden, deleted, or escalated?
- Which admin changed moderation policy?
- When did the event happen?

The in-memory event log supports live operation. Security and operator mutations
also flow into the durable SQLite audit trail, with optional bounded JSONL
mirroring and configured retention. Audit history is node-owned rather than
ordinary door state.

## Appeal And Unban

Appeal and unban can stay lightweight:

- bans and mutes should be visible to admins with reason and timestamp
- unban/unmute should be explicit admin actions
- operator notes should be possible before building user-facing appeals
- public stations may document an out-of-band contact path

No station feature implies cross-station appeal rights or global account recovery.

## Current Implementation

`phosphord` owns ban, unban, mute, unmute, per-user rate-limit, and moderation-note
operations. Station Admin exposes the controls, enforcement occurs during auth,
door access, and event handling, and security/operator mutations are recorded in
the durable audit trail described by the runtime and database documentation.
