CREATE TABLE IF NOT EXISTS users (
  public_key TEXT PRIMARY KEY,
  name TEXT,
  role TEXT,
  first_seen TEXT
);
