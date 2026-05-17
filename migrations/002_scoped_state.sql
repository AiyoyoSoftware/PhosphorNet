CREATE TABLE IF NOT EXISTS scoped_door_state (
  door_id TEXT NOT NULL,
  scope TEXT NOT NULL,
  scope_id TEXT NOT NULL,
  value_json TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (door_id, scope, scope_id)
);
