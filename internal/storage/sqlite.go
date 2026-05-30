package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"phosphornet/internal/protocol"

	_ "modernc.org/sqlite"
)

type Store struct {
	db      *sql.DB
	path    string
	created bool
}

type StateScopeIDs struct {
	User   string
	Room   string
	Global string
}

type UserRecord struct {
	PublicKey  string
	Name       string
	Role       string
	FirstSeen  string
	LastSeen   string
	Bio        string
	StatusLine string
}

type UserProfile struct {
	PublicKey   string
	DisplayName string
	Bio         string
	StatusLine  string
}

type StateRecordSummary struct {
	DoorID    string
	Scope     string
	ScopeID   string
	Bytes     int
	UpdatedAt string
}

type AuditEvent struct {
	ID               int64          `json:"id,omitempty"`
	Timestamp        string         `json:"timestamp"`
	ActorPublicKey   string         `json:"actor_public_key,omitempty"`
	ActorFingerprint string         `json:"actor_fingerprint,omitempty"`
	Action           string         `json:"action"`
	Target           string         `json:"target,omitempty"`
	Result           string         `json:"result"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

var stateKeyPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

const CurrentSchemaVersion = 5

func OpenSQLite(path string) (*Store, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve sqlite path: %w", err)
	}
	created := false
	if info, err := os.Stat(absPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("stat sqlite path: %w", err)
		}
		created = true
	} else if info.Size() == 0 {
		created = true
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	store := &Store{db: db, path: absPath, created: created}
	if err := store.ensureSchema(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) Created() bool {
	return s.created
}

func (s *Store) SchemaVersion(ctx context.Context) (int, error) {
	var version int
	if err := s.db.QueryRowContext(ctx, `PRAGMA user_version;`).Scan(&version); err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	return version, nil
}

func (s *Store) ensureSchema(ctx context.Context) error {
	schema := `
CREATE TABLE IF NOT EXISTS users (
  public_key TEXT PRIMARY KEY,
  name TEXT,
  bio TEXT,
  status_line TEXT,
  role TEXT,
  first_seen TEXT,
  last_seen TEXT
);

CREATE TABLE IF NOT EXISTS door_state (
  door_id TEXT NOT NULL,
  scope_key TEXT NOT NULL,
  value_json TEXT NOT NULL,
  PRIMARY KEY (door_id, scope_key)
);

CREATE TABLE IF NOT EXISTS scoped_door_state (
  door_id TEXT NOT NULL,
  scope TEXT NOT NULL,
  scope_id TEXT NOT NULL,
  value_json TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (door_id, scope, scope_id)
);

CREATE TABLE IF NOT EXISTS node_state (
  key TEXT PRIMARY KEY,
  value_json TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS audit_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  timestamp TEXT NOT NULL,
  actor_public_key TEXT,
  actor_fingerprint TEXT,
  action TEXT NOT NULL,
  target TEXT,
  result TEXT NOT NULL,
  metadata_json TEXT NOT NULL
);

DROP TRIGGER IF EXISTS audit_events_no_delete;

CREATE TRIGGER IF NOT EXISTS audit_events_no_update
BEFORE UPDATE ON audit_events
BEGIN
  SELECT RAISE(ABORT, 'audit_events are append-only');
END;`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("ensure schema: %w", err)
	}
	for _, column := range []struct {
		name       string
		definition string
	}{
		{name: "last_seen", definition: "TEXT"},
		{name: "bio", definition: "TEXT"},
		{name: "status_line", definition: "TEXT"},
	} {
		if err := s.ensureUsersColumn(ctx, column.name, column.definition); err != nil {
			return err
		}
	}
	if _, err := s.db.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d;`, CurrentSchemaVersion)); err != nil {
		return fmt.Errorf("set schema version: %w", err)
	}
	return nil
}

func (s *Store) ensureUsersColumn(ctx context.Context, columnName, definition string) error {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(users);`)
	if err != nil {
		return fmt.Errorf("inspect users schema: %w", err)
	}
	defer rows.Close()

	hasColumn := false
	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return fmt.Errorf("scan users schema: %w", err)
		}
		if name == columnName {
			hasColumn = true
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate users schema: %w", err)
	}
	if hasColumn {
		return nil
	}
	query := fmt.Sprintf(`ALTER TABLE users ADD COLUMN %s %s;`, columnName, definition)
	if _, err := s.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("add users.%s: %w", columnName, err)
	}
	return nil
}

func (s *Store) RecordUser(ctx context.Context, publicKey string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	query := `
INSERT INTO users (public_key, name, bio, status_line, role, first_seen, last_seen)
VALUES (?, '', '', '', 'member', ?, ?)
ON CONFLICT(public_key) DO UPDATE SET
  last_seen = excluded.last_seen;`
	_, err := s.db.ExecContext(ctx, query, publicKey, now, now)
	if err != nil {
		return fmt.Errorf("record user: %w", err)
	}
	return nil
}

func (s *Store) ListUsers(ctx context.Context) ([]UserRecord, error) {
	query := `SELECT public_key, COALESCE(name, ''), role, first_seen, COALESCE(last_seen, first_seen, ''), COALESCE(bio, ''), COALESCE(status_line, '') FROM users ORDER BY first_seen ASC;`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	users := []UserRecord{}
	for rows.Next() {
		var user UserRecord
		if err := rows.Scan(&user.PublicKey, &user.Name, &user.Role, &user.FirstSeen, &user.LastSeen, &user.Bio, &user.StatusLine); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}
	return users, nil
}

func (s *Store) LoadUserProfile(ctx context.Context, publicKey string) (UserProfile, error) {
	query := `SELECT public_key, COALESCE(name, ''), COALESCE(bio, ''), COALESCE(status_line, '') FROM users WHERE public_key = ?;`
	var profile UserProfile
	err := s.db.QueryRowContext(ctx, query, publicKey).Scan(&profile.PublicKey, &profile.DisplayName, &profile.Bio, &profile.StatusLine)
	if err != nil {
		if err == sql.ErrNoRows {
			return UserProfile{PublicKey: publicKey}, nil
		}
		return UserProfile{}, fmt.Errorf("load user profile: %w", err)
	}
	return profile, nil
}

func (s *Store) LoadUserProfiles(ctx context.Context, publicKeys []string) (map[string]UserProfile, error) {
	profiles := map[string]UserProfile{}
	seen := map[string]bool{}
	for _, publicKey := range publicKeys {
		if publicKey == "" || seen[publicKey] {
			continue
		}
		seen[publicKey] = true
		profile, err := s.LoadUserProfile(ctx, publicKey)
		if err != nil {
			return nil, err
		}
		profiles[publicKey] = profile
	}
	return profiles, nil
}

func (s *Store) SaveUserProfile(ctx context.Context, publicKey string, update protocol.ProfileUpdateEffect) (UserProfile, error) {
	if publicKey == "" {
		return UserProfile{}, fmt.Errorf("save user profile: public key is required")
	}
	if err := s.RecordUser(ctx, publicKey); err != nil {
		return UserProfile{}, err
	}
	profile, err := s.LoadUserProfile(ctx, publicKey)
	if err != nil {
		return UserProfile{}, err
	}
	if update.Reset {
		profile.DisplayName = ""
		profile.Bio = ""
		profile.StatusLine = ""
	} else {
		if update.DisplayName != nil {
			displayName, err := validateDisplayName(*update.DisplayName)
			if err != nil {
				return UserProfile{}, err
			}
			profile.DisplayName = displayName
		}
		if update.Bio != nil {
			bio, err := validateProfileText("bio", *update.Bio, 280)
			if err != nil {
				return UserProfile{}, err
			}
			profile.Bio = bio
		}
		if update.StatusLine != nil {
			statusLine, err := validateProfileText("status line", *update.StatusLine, 80)
			if err != nil {
				return UserProfile{}, err
			}
			profile.StatusLine = statusLine
		}
	}

	query := `
UPDATE users
SET name = ?, bio = ?, status_line = ?
WHERE public_key = ?;`
	if _, err := s.db.ExecContext(ctx, query, profile.DisplayName, profile.Bio, profile.StatusLine, publicKey); err != nil {
		return UserProfile{}, fmt.Errorf("save user profile: %w", err)
	}
	return profile, nil
}

func (s *Store) ListStateRecords(ctx context.Context) ([]StateRecordSummary, error) {
	query := `
SELECT door_id, scope, scope_id, length(value_json), updated_at
FROM scoped_door_state
ORDER BY door_id ASC, scope ASC, scope_id ASC;`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list scoped state records: %w", err)
	}
	defer rows.Close()

	records := []StateRecordSummary{}
	for rows.Next() {
		var record StateRecordSummary
		if err := rows.Scan(&record.DoorID, &record.Scope, &record.ScopeID, &record.Bytes, &record.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan scoped state record: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate scoped state records: %w", err)
	}
	return records, nil
}

func (s *Store) LoadNodeState(ctx context.Context, key string) (map[string]any, error) {
	if err := validateStateKey(key); err != nil {
		return nil, fmt.Errorf("load node state: %w", err)
	}
	query := `SELECT value_json FROM node_state WHERE key = ?;`
	var raw string
	err := s.db.QueryRowContext(ctx, query, key).Scan(&raw)
	if err != nil {
		if err == sql.ErrNoRows {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("load node state: %w", err)
	}
	return decodeState(raw)
}

func (s *Store) SaveNodeState(ctx context.Context, key string, value map[string]any) error {
	if err := validateStateKey(key); err != nil {
		return fmt.Errorf("save node state: %w", err)
	}
	if value == nil {
		value = map[string]any{}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode node state: %w", err)
	}
	if len(raw) > protocol.MaxScopedStateJSONBytes {
		return fmt.Errorf("save node state: state exceeds %d bytes", protocol.MaxScopedStateJSONBytes)
	}
	query := `
INSERT INTO node_state (key, value_json, updated_at)
VALUES (?, ?, ?)
ON CONFLICT(key) DO UPDATE SET
  value_json = excluded.value_json,
  updated_at = excluded.updated_at;`
	if _, err := s.db.ExecContext(ctx, query, key, string(raw), time.Now().UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("save node state: %w", err)
	}
	return nil
}

func (s *Store) AppendAuditEvent(ctx context.Context, event AuditEvent) (AuditEvent, error) {
	if strings.TrimSpace(event.Action) == "" {
		return AuditEvent{}, fmt.Errorf("append audit event: action is required")
	}
	if strings.TrimSpace(event.Result) == "" {
		return AuditEvent{}, fmt.Errorf("append audit event: result is required")
	}
	if event.Timestamp == "" {
		event.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	if event.Metadata == nil {
		event.Metadata = map[string]any{}
	}
	raw, err := json.Marshal(event.Metadata)
	if err != nil {
		return AuditEvent{}, fmt.Errorf("encode audit metadata: %w", err)
	}
	query := `
INSERT INTO audit_events (timestamp, actor_public_key, actor_fingerprint, action, target, result, metadata_json)
VALUES (?, ?, ?, ?, ?, ?, ?);`
	result, err := s.db.ExecContext(ctx, query, event.Timestamp, event.ActorPublicKey, event.ActorFingerprint, event.Action, event.Target, event.Result, string(raw))
	if err != nil {
		return AuditEvent{}, fmt.Errorf("append audit event: %w", err)
	}
	id, err := result.LastInsertId()
	if err == nil {
		event.ID = id
	}
	return event, nil
}

func (s *Store) ListAuditEvents(ctx context.Context, limit int) ([]AuditEvent, error) {
	query := `
SELECT id, timestamp, COALESCE(actor_public_key, ''), COALESCE(actor_fingerprint, ''), action, COALESCE(target, ''), result, metadata_json
FROM audit_events
ORDER BY id ASC;`
	args := []any{}
	if limit > 0 {
		query = `
SELECT id, timestamp, COALESCE(actor_public_key, ''), COALESCE(actor_fingerprint, ''), action, COALESCE(target, ''), result, metadata_json
FROM audit_events
ORDER BY id DESC
LIMIT ?;`
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list audit events: %w", err)
	}
	defer rows.Close()

	events := []AuditEvent{}
	for rows.Next() {
		var event AuditEvent
		var rawMetadata string
		if err := rows.Scan(&event.ID, &event.Timestamp, &event.ActorPublicKey, &event.ActorFingerprint, &event.Action, &event.Target, &event.Result, &rawMetadata); err != nil {
			return nil, fmt.Errorf("scan audit event: %w", err)
		}
		if rawMetadata != "" {
			if err := json.Unmarshal([]byte(rawMetadata), &event.Metadata); err != nil {
				return nil, fmt.Errorf("decode audit metadata: %w", err)
			}
		}
		if event.Metadata == nil {
			event.Metadata = map[string]any{}
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit events: %w", err)
	}
	if limit > 0 {
		for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
			events[i], events[j] = events[j], events[i]
		}
	}
	return events, nil
}

func (s *Store) TrimAuditEventsToMaxBytes(ctx context.Context, maxBytes int64) error {
	if maxBytes <= 0 {
		return nil
	}
	query := `
SELECT id,
       length(timestamp) +
       length(COALESCE(actor_public_key, '')) +
       length(COALESCE(actor_fingerprint, '')) +
       length(action) +
       length(COALESCE(target, '')) +
       length(result) +
       length(metadata_json)
FROM audit_events
ORDER BY id ASC;`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("trim audit events: %w", err)
	}
	defer rows.Close()

	type auditRowSize struct {
		id    int64
		bytes int64
	}
	rowsToTrim := []auditRowSize{}
	var total int64
	for rows.Next() {
		var row auditRowSize
		if err := rows.Scan(&row.id, &row.bytes); err != nil {
			return fmt.Errorf("scan audit event size: %w", err)
		}
		rowsToTrim = append(rowsToTrim, row)
		total += row.bytes
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate audit event sizes: %w", err)
	}

	for total > maxBytes && len(rowsToTrim) > 1 {
		row := rowsToTrim[0]
		if _, err := s.db.ExecContext(ctx, `DELETE FROM audit_events WHERE id = ?;`, row.id); err != nil {
			return fmt.Errorf("delete old audit event: %w", err)
		}
		total -= row.bytes
		rowsToTrim = rowsToTrim[1:]
	}
	return nil
}

func (s *Store) LoadDoorState(ctx context.Context, doorID, scopeKey string) (map[string]any, error) {
	state, err := s.loadScopedState(ctx, doorID, protocol.StateScopeUser, scopeKey)
	if err != nil {
		return nil, err
	}
	if len(state) > 0 {
		return state, nil
	}

	query := `SELECT value_json FROM door_state WHERE door_id = ? AND scope_key = ?;`
	var raw string
	err = s.db.QueryRowContext(ctx, query, doorID, scopeKey).Scan(&raw)
	if err != nil {
		if err == sql.ErrNoRows {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("load door state: %w", err)
	}

	return decodeState(raw)
}

func (s *Store) SaveDoorState(ctx context.Context, doorID, scopeKey string, state map[string]any) error {
	return s.saveScopedState(ctx, doorID, protocol.StateScopeUser, scopeKey, state)
}

func (s *Store) LoadScopedState(ctx context.Context, doorID string, ids StateScopeIDs) (protocol.RuntimeStateSnapshot, error) {
	user, err := s.loadScopedState(ctx, doorID, protocol.StateScopeUser, ids.User)
	if err != nil {
		return protocol.RuntimeStateSnapshot{}, err
	}
	if len(user) == 0 {
		legacy, err := s.loadLegacyDoorState(ctx, doorID, ids.User)
		if err != nil {
			return protocol.RuntimeStateSnapshot{}, err
		}
		user = legacy
	}
	room, err := s.loadScopedState(ctx, doorID, protocol.StateScopeRoom, ids.Room)
	if err != nil {
		return protocol.RuntimeStateSnapshot{}, err
	}
	global, err := s.loadScopedState(ctx, doorID, protocol.StateScopeGlobal, ids.Global)
	if err != nil {
		return protocol.RuntimeStateSnapshot{}, err
	}
	return protocol.RuntimeStateSnapshot{
		User:   user,
		Room:   room,
		Global: global,
	}, nil
}

func (s *Store) ApplyStateOps(ctx context.Context, doorID string, ids StateScopeIDs, role string, ops []protocol.StateOp) error {
	if len(ops) == 0 {
		return nil
	}
	if err := validateStateOps(ops); err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin state transaction: %w", err)
	}
	defer tx.Rollback()

	for _, op := range ops {
		if op.Scope == protocol.StateScopeGlobal && !canWriteGlobal(role) {
			return fmt.Errorf("apply state op: %w", errors.New("global state writes require admin role"))
		}

		scopeID, err := stateScopeID(op.Scope, ids)
		if err != nil {
			return err
		}

		state, err := loadScopedStateTx(ctx, tx, doorID, op.Scope, scopeID)
		if err != nil {
			return err
		}
		next, err := applyStateOp(state, op)
		if err != nil {
			return err
		}
		if err := saveScopedStateTx(ctx, tx, doorID, op.Scope, scopeID, next); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit state transaction: %w", err)
	}
	return nil
}

func (s *Store) loadLegacyDoorState(ctx context.Context, doorID, scopeKey string) (map[string]any, error) {
	query := `SELECT value_json FROM door_state WHERE door_id = ? AND scope_key = ?;`
	var raw string
	err := s.db.QueryRowContext(ctx, query, doorID, scopeKey).Scan(&raw)
	if err != nil {
		if err == sql.ErrNoRows {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("load legacy door state: %w", err)
	}
	return decodeState(raw)
}

func (s *Store) loadScopedState(ctx context.Context, doorID string, scope protocol.StateScope, scopeID string) (map[string]any, error) {
	return loadScopedState(ctx, s.db, doorID, scope, scopeID)
}

func (s *Store) saveScopedState(ctx context.Context, doorID string, scope protocol.StateScope, scopeID string, state map[string]any) error {
	return saveScopedState(ctx, s.db, doorID, scope, scopeID, state)
}

func loadScopedStateTx(ctx context.Context, tx *sql.Tx, doorID string, scope protocol.StateScope, scopeID string) (map[string]any, error) {
	return loadScopedState(ctx, tx, doorID, scope, scopeID)
}

func saveScopedStateTx(ctx context.Context, tx *sql.Tx, doorID string, scope protocol.StateScope, scopeID string, state map[string]any) error {
	return saveScopedState(ctx, tx, doorID, scope, scopeID, state)
}

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type execer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func loadScopedState(ctx context.Context, q queryer, doorID string, scope protocol.StateScope, scopeID string) (map[string]any, error) {
	if scopeID == "" {
		return map[string]any{}, nil
	}
	query := `SELECT value_json FROM scoped_door_state WHERE door_id = ? AND scope = ? AND scope_id = ?;`
	var raw string
	err := q.QueryRowContext(ctx, query, doorID, string(scope), scopeID).Scan(&raw)
	if err != nil {
		if err == sql.ErrNoRows {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("load scoped door state: %w", err)
	}
	return decodeState(raw)
}

func saveScopedState(ctx context.Context, e execer, doorID string, scope protocol.StateScope, scopeID string, state map[string]any) error {
	if state == nil {
		state = map[string]any{}
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode door state: %w", err)
	}
	if len(raw) > protocol.MaxScopedStateJSONBytes {
		return fmt.Errorf("save scoped door state: state exceeds %d bytes", protocol.MaxScopedStateJSONBytes)
	}

	query := `
INSERT INTO scoped_door_state (door_id, scope, scope_id, value_json, updated_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(door_id, scope, scope_id) DO UPDATE SET
  value_json = excluded.value_json,
  updated_at = excluded.updated_at;`
	if _, err := e.ExecContext(ctx, query, doorID, string(scope), scopeID, string(raw), time.Now().UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("save scoped door state: %w", err)
	}
	return nil
}

func decodeState(raw string) (map[string]any, error) {
	var state map[string]any
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return nil, fmt.Errorf("decode door state: %w", err)
	}
	if state == nil {
		state = map[string]any{}
	}
	return state, nil
}

func stateScopeID(scope protocol.StateScope, ids StateScopeIDs) (string, error) {
	switch scope {
	case protocol.StateScopeUser:
		return ids.User, nil
	case protocol.StateScopeRoom:
		return ids.Room, nil
	case protocol.StateScopeGlobal:
		return ids.Global, nil
	default:
		return "", fmt.Errorf("unknown state scope %q", scope)
	}
}

func applyStateOp(state map[string]any, op protocol.StateOp) (map[string]any, error) {
	if state == nil {
		state = map[string]any{}
	}
	switch op.Op {
	case protocol.StateOpSet:
		if op.Key == "" {
			return nil, fmt.Errorf("state set op requires key")
		}
		state[op.Key] = op.Value
	case protocol.StateOpDelete:
		if op.Key == "" {
			return nil, fmt.Errorf("state delete op requires key")
		}
		delete(state, op.Key)
	case protocol.StateOpClear:
		state = map[string]any{}
	case protocol.StateOpReplace:
		next, ok := op.Value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("state replace op requires object value")
		}
		state = next
	default:
		return nil, fmt.Errorf("unknown state op %q", op.Op)
	}
	return state, nil
}

func canWriteGlobal(role string) bool {
	return role == "admin" || role == "sysop"
}

func validateStateOps(ops []protocol.StateOp) error {
	if len(ops) > protocol.MaxStateOpsPerBatch {
		return fmt.Errorf("apply state op: too many operations in one batch (%d > %d)", len(ops), protocol.MaxStateOpsPerBatch)
	}

	totalJSONBytes := 0
	for _, op := range ops {
		opBytes, err := json.Marshal(op)
		if err != nil {
			return fmt.Errorf("apply state op: encode op: %w", err)
		}
		totalJSONBytes += len(opBytes)
		if totalJSONBytes > protocol.MaxStateBatchJSONBytes {
			return fmt.Errorf("apply state op: batch exceeds %d bytes", protocol.MaxStateBatchJSONBytes)
		}

		switch op.Op {
		case protocol.StateOpSet, protocol.StateOpDelete:
			if err := validateStateKey(op.Key); err != nil {
				return fmt.Errorf("apply state op: %w", err)
			}
		case protocol.StateOpClear:
		case protocol.StateOpReplace:
			valueBytes, err := json.Marshal(op.Value)
			if err != nil {
				return fmt.Errorf("apply state op: encode replace value: %w", err)
			}
			if len(valueBytes) > protocol.MaxStateValueJSONBytes {
				return fmt.Errorf("apply state op: replace value exceeds %d bytes", protocol.MaxStateValueJSONBytes)
			}
		default:
			return fmt.Errorf("apply state op: unknown state op %q", op.Op)
		}

		if op.Op == protocol.StateOpSet {
			valueBytes, err := json.Marshal(op.Value)
			if err != nil {
				return fmt.Errorf("apply state op: encode set value: %w", err)
			}
			if len(valueBytes) > protocol.MaxStateValueJSONBytes {
				return fmt.Errorf("apply state op: set value exceeds %d bytes", protocol.MaxStateValueJSONBytes)
			}
		}
	}
	return nil
}

func validateStateKey(key string) error {
	if len(key) > protocol.MaxStateKeyBytes {
		return fmt.Errorf("state key exceeds %d bytes", protocol.MaxStateKeyBytes)
	}
	if !stateKeyPattern.MatchString(key) {
		return fmt.Errorf("invalid state key %q", key)
	}
	return nil
}

var reservedDisplayNames = map[string]bool{
	"admin":   true,
	"sysop":   true,
	"root":    true,
	"mod":     true,
	"system":  true,
	"station": true,
}

func validateDisplayName(value string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(value, "\r", " "), "\n", " "))
	if value == "" {
		return "", nil
	}
	if len(value) < 2 || len(value) > 32 {
		return "", fmt.Errorf("display name must be 2-32 characters")
	}
	for _, ch := range value {
		if ch < 32 || ch == 127 {
			return "", fmt.Errorf("display name cannot include control characters")
		}
	}
	if reservedDisplayNames[strings.ToLower(value)] {
		return "", fmt.Errorf("display name %q is reserved", value)
	}
	return value, nil
}

func validateProfileText(label, value string, maxLen int) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(value, "\r", " "), "\n", " "))
	if len(value) > maxLen {
		return "", fmt.Errorf("%s must be %d characters or less", label, maxLen)
	}
	return value, nil
}
