# PhosphorNet Database Lifecycle

## Purpose

This is the boring operator document for station memory.

SQLite is the station's durable memory. Losing the database means losing known users, user profiles, roles, door state, station settings, moderation state, station policy, durable audit events, and admin-door configuration.

Backups should treat these files as one station bundle:

- `node.toml`
- `phosphornet.db`, or the configured `database` path
- node private key material, currently the `private_key` field in `node.toml`
- door manifests and bundled custom doors under `doors_dir`

If a station has custom stdio images, also preserve the image source or registry tags needed to recreate them. The database stores state, not container images.

## Startup Check

On `phosphord serve`, the node logs the absolute SQLite path and current schema version:

```text
phosphord database path=/srv/phosphornet/phosphornet.db schema_version=5
```

The version is SQLite `PRAGMA user_version`. It should match the latest schema version understood by the running binary.

## Audit Events

`phosphord` appends operator/security audit records to the `audit_events` table. The table is append-only-ish: the shipped storage API appends new rows, SQLite rejects direct `UPDATE`, and size retention deletes only the oldest rows when an operator sets `--audit-log-max-bytes`.

Current audit columns are:

- `timestamp`
- `actor_public_key`
- `actor_fingerprint`
- `action`
- `target`
- `result`
- `metadata_json`

The audit stream is separate from the in-memory Station Admin event log. Clearing the in-memory event log does not clear `audit_events`, and the clear action is itself audited. Operators may also pass `--audit-log-file` to `phosphord serve` to mirror the same events to a JSON Lines file. `--audit-log-max-bytes` applies to both SQLite retention and optional JSONL file rotation; `--audit-log-file-max-backups` controls how many rotated files are kept.

## Backup

The safest backup is a stopped-daemon file backup:

1. Stop `phosphord`.
2. Copy `node.toml`.
3. Copy the configured SQLite database file.
4. Copy `doors_dir`, including every `manifest.toml` and custom door source.
5. Copy any external artifacts required by custom doors, such as container build files or pinned image tags.
6. Start `phosphord`.

If SQLite WAL files exist beside the database, back up the database with SQLite's online backup API or stop the daemon before copying. Do not copy only `phosphornet.db` while `phosphornet.db-wal` is active.

For a simple stopped backup:

```bash
systemctl stop phosphord
tar -czf phosphornet-station-backup.tgz node.toml phosphornet.db doors/
systemctl start phosphord
```

For a manual local development backup:

```bash
cp node.toml node.toml.backup
cp phosphornet.db phosphornet.db.backup
cp -a doors doors.backup
```

Keep backups encrypted or access-controlled. `node.toml` contains the station private key.

## Restore

Restore the station bundle together:

1. Stop `phosphord`.
2. Move the damaged or replacement files aside.
3. Restore `node.toml`.
4. Restore the SQLite database to the configured `database` path.
5. Restore `doors_dir` with matching manifests and custom door source.
6. Ensure file ownership and permissions match the `phosphord` service user.
7. Start `phosphord`.
8. Check startup logs for the database path and schema version.

If the node key in `node.toml` changes during restore, clients will treat the station as a different identity and will refuse the old address until they replace their known-node pin.

## Migration Expectations

Current code bootstraps and upgrades the SQLite schema in-process on open, then sets `PRAGMA user_version` to the current schema version.

Migration expectations:

- New binaries may add tables or columns during startup.
- Schema changes must be backward-readable only when the code explicitly keeps that compatibility.
- Back up before starting a newer binary against an important station database.
- Do not downgrade a station database unless the release notes explicitly say it is supported.
- The `migrations/` directory documents the intended schema history; runtime bootstrap is the current enforcement path.

## If The Database Is Deleted

If `phosphornet.db` is deleted, `phosphord` creates a fresh empty database on startup.

The station keeps its network identity only if `node.toml` is still present, but the station memory is gone:

- known users are gone
- display names, bios, and status lines are gone
- roles and station policy stored in SQLite are gone
- moderation notes, bans, mutes, and rate limits are gone
- durable audit events are gone unless mirrored or backed up separately
- door settings and door state are gone
- admin-door storage summaries start empty

The default station policy is seeded again, including disabling `strategy_demo` by default. Anything operator-edited in SQLite must be restored from backup.

## If The Node Key Is Deleted

If the node private key is deleted or `node.toml` is regenerated, the station becomes a new Ed25519 identity.

The database may still contain users, roles, and door state, but clients pinned to the old station key will reject the connection for the same address. Operators should either restore the old `node.toml` from backup or deliberately announce a station identity replacement.

Do not treat a regenerated key as a normal restart. It is closer to replacing the station.

## What Is Safe To Delete

Safe to delete for local development only:

- disposable test databases
- quick-mode client state under `/tmp/phosphornet-quick/`
- generated logs and build artifacts

Not safe to delete on a real station without a backup:

- the configured SQLite database
- `node.toml`
- custom door manifests or source
- files needed to rebuild custom stdio images

## Repair And Export

Database repair and export use standard SQLite tools; PhosphorNet does not wrap them in a dedicated command.

For serious SQLite recovery, stop `phosphord` and use standard SQLite tooling on a copy of the database. Keep the original untouched until the recovered copy has been verified.

For compaction, run SQLite `VACUUM` only while `phosphord` is stopped. Do not mutate the live database behind a running node.
