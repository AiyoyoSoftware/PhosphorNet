# PhosphorNet Roadmap

PhosphorNet is a working terminal-native platform for authenticated homelab
interfaces, server-hosted doors, persistent state, and explicitly authorized
host actions.

This file is the single authoritative source for planned project work. The
README and subsystem documentation describe behavior available in the current
release. Completed work is recorded in `changelog.md`; historical planning
remains available through version-control history.

## Current Focus

Make PhosphorNet immediately useful as a safe, terminal-native homelab control
surface.

- Define a versioned door package format with compatibility metadata,
  checksums, and explicit capability declarations.
- Add `phosphord doors` commands to inspect, install, update, disable, and
  remove packaged doors without granting host authority implicitly.
- Establish a separately released official door collection for homelab status,
  service inspection, bounded logs, containers, storage, and backups.
- Build a polished flagship homelab console that combines read-only status
  with a small set of explicitly allowlisted maintenance actions.

## Planned Next

- Package service-manager units and service-account/socket setup for
  `phosphord` and `phosphor-actiond`.
- Add operator-facing diagnostics for station health, door compatibility,
  runtime availability, action rules, and socket permissions.
- Add publisher provenance, signatures, atomic upgrades, and rollback to the
  door installation flow.
- Refine the generic stdio containment profile with explicit environment
  allowlists and reusable named profiles.

## Longer-Term Direction

- Native `switchboard` relay and rendezvous support for operators who do not
  use direct networking or a private VPN.
- Additional transition types beyond the currently supported `open_door`
  transition.
- Richer interaction scopes when a concrete door requires more than the
  current one-scope-per-door model.
- Encrypted-at-rest passport storage and clearer identity backup, migration,
  and key-rotation workflows.

These directions are not requirements for direct LAN, VPN, or reverse-proxy
homelab deployments.

## Product Boundaries

PhosphorNet intentionally does not:

- provide shell accounts to station users
- execute downloaded door code on the client
- pass node-provided terminal escape sequences through to the terminal
- grant doors implicit clipboard, filesystem, network, or host-command access
- allow a door to invoke a host action unless both its manifest and the local
  `phosphor-actiond` rule authorize the exact rule ID and door ID

## Roadmap Maintenance

Keep only committed or intentionally tracked work in this file. Do not add
resolved implementation history, speculative design questionnaires, or a
second backlog to other documentation. When work ships, move its outcome to
`changelog.md` and update the owning current-behavior document.
