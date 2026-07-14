# PhosphorNet User Guide

## Purpose

This guide explains how to use the current `phosphor` client and what to expect when connecting to a station.

PhosphorNet is a terminal-native network of stations. A station hosts doors. A door is a hosted place, tool, game, room, or service. The trusted local client renders everything in your terminal.

## Connect To A Station

Install the client and create or reuse your local passport:

```bash
curl -fsSL https://aiyoyo.org/phosphornet/install.sh | sudo sh -s -- --client
phosphor init
```

For local development:

```bash
go run ./cmd/phosphor connect wss://127.0.0.1:7707/ws --quick
```

For a persistent identity:

```bash
phosphor connect wss://127.0.0.1:7707/ws
```

The address is optional. With no address, `phosphor connect` uses `wss://127.0.0.1:7707/ws`. Bare hosts are expanded for convenience: `phosphor connect localhost` becomes `wss://localhost:7707/ws`, and addresses that do not end in `/ws` get `/ws` appended.

The client creates a passport automatically if one does not already exist at the selected passport path.

## Passports

A passport is your local Ed25519 identity key.

Show the current default passport:

```bash
phosphor passport show
```

The public key and fingerprint identify you to nodes. The private key stays on your machine.

You can choose an explicit passport:

```bash
go run ./cmd/phosphor connect \
  wss://127.0.0.1:7707/ws \
  --passport ./dev/passport.toml
```

## Known Nodes

When you connect to a node, `phosphor` pins that node's public key for the address you used. This is similar to SSH known-host behavior.
The client accepts self-signed station certificates by default, but the actual station identity check is still the signed Ed25519 node challenge plus the pinned node key.

On a first connection to an address, `phosphor` shows a client-owned Bubble Tea trust screen before saving the pin. That screen separates the Ed25519-signed station name, transport encryption, certificate status, hostname verification, and the Ed25519 station identity fingerprint, then asks whether to trust and pin that station identity for the address.

Read the connection trust indicators as separate facts:

```text
Transport
  TLS encrypted or not encrypted.

Certificate
  Self-signed station certificate or domain-authenticated WebPKI certificate.

Hostname verification
  Whether the certificate proves the hostname through WebPKI.

Station identity
  Ed25519 node key, either new, pinned, or changed.
```

A self-signed certificate can still be acceptable because PhosphorNet pins the station's Ed25519 identity separately.

Default known-node file:

```text
~/.config/phosphornet/known_nodes.toml
```

If a node is regenerated and its key changes, the client refuses to connect to the same address until you update or remove the old pin.

## Station View

After connecting, the client shows:

- trusted local chrome with connection and identity information
- a door rail listing hosted doors
- the active remote door rendered as local TUI components
- a focus rail for interactive remote components

The node sends declarative JSON UI. The client renders it locally. Doors cannot execute code in your terminal.

The door rail only shows doors that are both accessible to your passport and marked with public door visibility. Private and hidden doors are filtered out of the normal rail.

## Controls

Station view:

| Key | Action |
|---|---|
| `tab` | Switch focus between the door rail and remote viewport. |
| `left` / `right` | Cycle doors or remote controls in the focused panel. |
| `up` / `down` | Scroll the focused panel. |
| `enter` | Open a door, activate a button/menu item, or submit input. |
| `pgup` / `pgdown` | Scroll the remote door viewport. |
| `ctrl+u` / `ctrl+d` | Scroll the remote door viewport. |
| `home` / `end` | Jump to the top or bottom of remote content. |
| `f` | Toggle fullscreen door mode. |
| `esc` | Leave fullscreen or return focus to the door rail. |
| `q` | Quit. |

When a remote input or textarea is focused, printable keys are typed into it instead of triggering global shortcuts. `ctrl+c` still quits from anywhere.

Fullscreen mode gives the active door more room while keeping slim trusted client chrome visible.

## Using Doors

A door may contain:

- text and status lines
- panels
- menus and lists
- buttons
- inputs and textareas
- logs
- grids

Interactive controls emit semantic events such as `action`, `select`, or `submit`. For example, pressing `enter` on a button sends the button's action to the node. Submitting an input sends the input value to the node.

Raw key events are not the default. They should only be used by doors that explicitly need key capture.

## Bundled Doors

`lobby`:

- default station landing door
- shows a simple station homepage with three panels: Station, Who Is Here, and Station Notice
- conditionally links to the `profile` door when you do not yet have a station display name
- records a per-user visit counter

`profile`:

- standalone station-identity door for display name, status line, and bio
- shows the passport fingerprint as the real identity anchor while letting you set the friendlier station-facing name

`chat`:

- IRC-like shared room message log
- opens at the newest messages and stays there while you are at the bottom
- scrolling up lets you read earlier channel events without being forced back down
- room-scoped state
- presence and join/leave messages
- uses your station display name for visible identity, falling back to a guest name based on your fingerprint
- slash commands: `/nickname <name>`, `/tell <display-name> <message>`, `/who`, `/help`
- broadcast re-rendering for connected sessions

Reconnects:

- reconnecting signs in as a new session
- brief drops may reopen your previous door when it is still available to you
- if the previous door is not safe to reopen, you return to `lobby`
- unsent input and scroll position are not restored
- join/leave messages are delayed briefly so short network drops do not create noisy leave/rejoin chatter

`forum`:

- keeps forum authors tied to station profile display names while still showing the passport fingerprint as the identity anchor in post cards

`strategy_demo` / Iron Orchard:

- shared tactics-room proof
- grid component
- player list and turn state
- room broadcasts after changes

`action_demo` / Action Workshop:

- demonstrates fixed semantic choices delegated to the optional `phosphor-actiond` host-action process
- shows structured success, failure, exit code, stdout, and stderr results without accepting command or argv text from users
- ships with its matching operator rules in `doors/action_demo/actiond.example.toml`
- starts disabled until the station operator configures the matching `demo-*` TOML rules and enables the door

`admin` / Station Admin:

- visible only to admin passports
- opens as a multi-page sysop console with Home, Doors, Door Settings, Users, Access Control, Moderation, Storage, Runtime, Logs, and Maintenance pages
- shows station metadata, hosted manifests, connected sessions, known users with display names and guest status, runtime settings, database path, scoped state summaries, and recent in-memory station events
- edits manifest-declared door settings such as lobby MOTD text, toggles, and taglines without rewriting door files
- can reload door manifests so newly added doors appear without restarting the node
- lets admins reorder enabled doors for the trusted navigation menu from the Doors page after selecting a door and using `=` or `-`
- records maintenance checkpoints in door-global state
- sends station-wide notices to all connected sessions
- enables and disables maintenance mode
- bans, unbans, mutes, unmutes, rate-limits, and records moderation notes for passport public keys
- clears admin notices and maintenance state
- requires a confirmation subview for maintenance reset

Public-station moderation is intentionally small for now. The admin surface supports key bans, key mutes, per-user event/open-door rate limits, door freezes, maintenance notices, recent station-event inspection, moderation notes, and forum post hide/delete controls. The local station-policy model is documented in `docs/PhosphorNet_public_station_moderation.md`.

## Notifications

Doors can request notifications, but the node and trusted client decide how they appear. Notifications are structured messages, not arbitrary terminal output.

Current notification levels are simple strings such as:

```text
info
warning
error
```

## Station Profile

Each passport can have a station-local display name, optional status line, and optional bio.

- The passport fingerprint remains the real identity anchor.
- The display name is the human-friendly layer used in lobby presence, chat, forum, and admin views.
- If you leave the display name blank, the station shows you as a guest derived from your fingerprint, such as `guest-RAVEN`.

## What Stations Cannot Do To The Client

Remote nodes and doors must not:

- execute code on the client
- print raw terminal escape sequences
- overwrite trusted client chrome
- access local files
- access the shell
- read the clipboard
- receive passport private keys

If a future feature asks for local authority, it should appear as trusted client UI and require an explicit local decision.

## Troubleshooting

Connection refused:

- Confirm `phosphord` is running.
- Confirm the address uses the right host and port. The client appends `/ws` when the address does not already end with it.
- Check `curl -k https://127.0.0.1:7707/healthz`.

Node identity changed:

- The node key for that address does not match the pinned key.
- For local development, remove the old pin from `known_nodes.toml`, use `--quick`, or reconnect with `--replace-known-node` to overwrite the pin for that address.

Station is invite-only:

- Ask the station operator to add your passport public key or fingerprint to the station allowlist.
- Use `phosphor passport show` to print both values.

Admin door is missing:

- Confirm your passport public key or fingerprint is listed in `access.admins` in the station config.
- Reconnect after the station operator restarts `phosphord`.

Door interaction does nothing:

- Press `tab` until the remote component focus rail is active.
- Use `left` / `right` to choose another remote component or door.
- Confirm the door exposes buttons, menus, lists, inputs, or textareas.

Text view is too tall:

- Use `pgup`, `pgdown`, `home`, or `end` to scroll.
- Use `f` for fullscreen door mode.
