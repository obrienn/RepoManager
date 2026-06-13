CREATE TABLE IF NOT EXISTS repositories (
    id            SERIAL PRIMARY KEY,
    name          TEXT NOT NULL,
    owner         TEXT NOT NULL,
    github_url    TEXT NOT NULL UNIQUE,
    local_path    TEXT NOT NULL UNIQUE,
    description   TEXT,
    license       TEXT,
    default_branch TEXT DEFAULT 'main',
    last_updated  TIMESTAMPTZ,
    added_date    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    total_size_bytes BIGINT DEFAULT 0,
    needs_attention BOOLEAN DEFAULT FALSE,
    attention_reason TEXT
);

CREATE TABLE IF NOT EXISTS languages (
    id            SERIAL PRIMARY KEY,
    repo_id       INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    language_name TEXT NOT NULL,
    percentage    REAL DEFAULT 0,
    bytes         BIGINT DEFAULT 0
);

CREATE TABLE IF NOT EXISTS topics (
    id       SERIAL PRIMARY KEY,
    repo_id  INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    topic_name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS releases (
    id           SERIAL PRIMARY KEY,
    repo_id      INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    tag_name     TEXT NOT NULL,
    published_date TIMESTAMPTZ,
    is_prerelease BOOLEAN DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS tags (
    id    SERIAL PRIMARY KEY,
    name  TEXT NOT NULL UNIQUE,
    color TEXT DEFAULT '#6b7280'
);

CREATE TABLE IF NOT EXISTS repository_tags (
    repo_id INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    tag_id  INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (repo_id, tag_id)
);

CREATE TABLE IF NOT EXISTS update_logs (
    id              SERIAL PRIMARY KEY,
    repo_id         INTEGER REFERENCES repositories(id) ON DELETE SET NULL,
    checked_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status          TEXT NOT NULL DEFAULT 'ok',
    file_count_old  INTEGER,
    file_count_new  INTEGER,
    size_bytes_old  BIGINT,
    size_bytes_new  BIGINT,
    delta_percent   REAL,
    flagged         BOOLEAN DEFAULT FALSE
);
