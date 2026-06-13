# RepoManager

A self-hosted "bookshelf" for GitHub repositories — catalog, index, and maintain local copies of public GitHub repos with safeguards against deprecation and removal.

## Why

GitHub repositories increasingly serve as both development trackers and software distribution platforms. Developers point users to their GitHub repos for downloads, documentation, and releases. But GitHub is unreliable as a long-term storage platform — repos go private, get deleted, or have their contents stripped. There are few good solutions for maintaining local, up-to-date copies of these repositories with library-style indexing and navigation.

RepoManager solves this by providing a browsable catalog of locally stored GitHub repos, periodically updated with checks to detect and prevent destructive upstream changes.

## Features

- **Catalog with metadata** — browse repos by name, owner, date added, or last updated. Each repo shows tagged topics, license, latest release version, language breakdown, and a link to the live GitHub page
- **Local README rendering** — click into any repo to read its README.md rendered as HTML with syntax-highlighted code blocks
- **Full-text search** — search across repo names, descriptions, GitHub topics, user-defined tags, and README content
- **User-defined tags** — create, edit, and assign custom tags to repos for quick grouping and filtering
- **Safeguarded updates** — update repos with checks for deprecation and privatization:
  - **Remote check**: verifies the GitHub repo still exists before pulling. If it 404s (gone private/deleted), the local copy is preserved and the repo is flagged
  - **Deprecation check**: snapshots file count and size before pulling. If the pull removes >30% of files, the update is auto-reverted to the previous commit and the repo is flagged
  - **Safety tag**: each pull creates a local tag pinning the previous commit so Git can never garbage-collect it
- **Re-clone** — replace the local copy with a fresh clone from GitHub, with the same safeguard checks
- **Delete with cleanup** — removing a repo from the catalog also removes its local clone from disk
- **Automatic datastore scanning** — on startup and during Update All, scans the data store directory for repos added out-of-band and imports them into the catalog
- **Catalog metrics** — displays total repos tracked, repos needing attention, total storage size, and next scheduled update time
- **Light/dark theme** — toggle between themes, persisted to a cookie
- **Sorted persistence** — sort field and order persist across sessions via cookies

## Architecture

```
Browser (HTML/CSS/TypeScript SPA) ──HTTP──▶ Go HTTP Server (net/http)
                                               ├── REST API handlers
                                               ├── Git operations (clone, pull, diff, reset)
                                               ├── GitHub API client (topics, description, license)
                                               └── Datastore scanner
                                                  │
                                               PostgreSQL
                                               ├── repositories, languages, topics, releases
                                               ├── tags, repository_tags
                                               └── update_logs
```

## Stack

- **Backend**: Go (net/http), pgx (PostgreSQL driver), goldmark (markdown rendering)
- **Database**: PostgreSQL
- **Frontend**: Vanilla TypeScript, HTML, CSS (no framework)

## Setup

### Prerequisites

- Go 1.22+
- PostgreSQL
- Git (with git-lfs recommended)
- Node.js (for TypeScript compilation)

### Configuration

Create a `.env` file in the project root:

```env
# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=repoman
DB_PASSWORD=
DB_NAME=repoman

# Server
SERVER_PORT=8080
SERVER_ADDRESS=0.0.0.0

# Data store (where repos are cloned)
DATASTORE_DIR=./library

# Optional
GITHUB_TOKEN=           # GitHub API token for higher rate limits
UPDATE_INTERVAL=        # Custom update interval (e.g., "6h", "30m"). Empty = daily at 3am local time. "0" = disabled
```

### Build & Run

```bash
# Compile frontend
cd frontend && npm install && npx tsc

# Build and run
go build -o repomanager .
./repomanager
```

The server starts on the configured port, runs database migrations, scans the data store for existing repos, and begins the background update scheduler.

### API

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/health` | Health check |
| `GET` | `/api/repos` | List repos (`?sort=`, `&order=`, `&search=`) |
| `POST` | `/api/repos` | Add a new repo |
| `GET` | `/api/repos/{id}` | Get repo details |
| `DELETE` | `/api/repos/{id}` | Remove repo (deletes local clone + DB record) |
| `GET` | `/api/repos/{id}/readme` | Get rendered README (HTML) |
| `POST` | `/api/repos/{id}/update` | Update single repo with safeguards |
| `POST` | `/api/repos/{id}/reclone` | Delete local clone and re-clone from GitHub |
| `POST` | `/api/repos/update-all` | Scan data store + update all repos |
| `GET` | `/api/tags` | List all user-defined tags |
| `POST` | `/api/tags` | Create a tag |
| `PUT` | `/api/tags/{id}` | Update a tag |
| `DELETE` | `/api/tags/{id}` | Delete a tag |
| `GET` | `/api/repos/{id}/tags` | Get tags assigned to a repo |
| `PUT` | `/api/repos/{id}/tags` | Set tags for a repo |
| `GET` | `/api/metrics` | Catalog statistics + next update time |
