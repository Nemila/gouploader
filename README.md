# gouploader

A self-hosted media upload pipeline written in Go. It watches local folders for video files, uploads them in parallel to multiple video hosting providers, registers the resulting links with your website, archives the originals to a Telegram channel, and cleans up local disk.

The whole pipeline runs in a loop on a fixed cooldown (5 minutes), making it suitable to run as a long-lived background service (e.g. a `systemd` user unit).

## How it works

Each cycle runs four stages:

1. **Scan** — Recursively walks the configured media folders and inserts any matching files into the SQLite database as `pending`.
2. **Process** — For every `pending` file, uploads it concurrently to all configured hosts (skipping hosts where it already exists), stores the returned slug, and imports the link into your website via its API.
3. **Archive** — Sends files that haven't been archived yet to a Telegram channel as a backup.
4. **Clean up** — Deletes local files that are both uploaded (`done`) and archived, marking them `saved`.

State is tracked in a local SQLite database so the pipeline is resumable and idempotent across restarts.

### File naming convention

Only files matching the following pattern are picked up by the scanner:

```
<name>-<tv|movie>-<tmdbId>-S<season>-E<episode>[-<vf|vo|vostfr|multi>].<ext>
```

Example: `the-office-tv-2316-S01-E01-vostfr.mp4`

## Supported hosts

| Host    | Adapter                |
| ------- | ---------------------- |
| Hydrax / Abyss | `adapters/abyss.go`   |
| Uqload  | `adapters/uqload.go`   |
| Vidhide | `adapters/vidhide.go`  |
| Sendvid | `adapters/sendvid.go`  |

Adapters implement a single `Upload(filePath string) (string, error)` interface, so adding a new host is a matter of implementing that interface and registering it in `adapters/config.go`.

## Tech stack

- **Go** 1.26
- **SQLite** via [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite) (pure Go, no CGO)
- [`sqlc`](https://sqlc.dev/) for type-safe query generation
- [`go-telegram-bot-api`](https://github.com/go-telegram-bot-api/telegram-bot-api) for Telegram archiving
- [`goquery`](https://github.com/PuerkitoBio/goquery) for HTML scraping in adapters

## Getting started

### Prerequisites

- Go 1.26+
- API credentials for the hosts you want to use
- A Telegram bot token and a target channel

### Configuration

Copy the example environment file and fill in your values:

```bash
cp .env.example .env
```

| Variable         | Description                                              |
| ---------------- | -------------------------------------------------------- |
| `ABYSS_KEY`      | Hydrax/Abyss API key                                     |
| `VIDHIDE_KEY`    | Vidhide API key                                          |
| `VIDHIDE_SESSID` | Vidhide session ID                                       |
| `UQLOAD_KEY`     | Uqload API key                                           |
| `UQLOAD_SESSID`  | Uqload session ID                                        |
| `SENDVID_KEY`    | Sendvid credentials in `username:password` form          |
| `WEBSITE_URL`    | Base URL of your website's import API                    |
| `MEDIA_PATH`     | Folder to scan for media files                           |
| `TG_TOKEN`       | Telegram bot token                                       |
| `TG_ENDPOINT`    | Telegram Bot API endpoint                                |
| `ENV`            | `DEV` or `PROD`                                          |

All variables are required; the app fails fast on startup if any are missing.

> The Telegram channel ID and a secondary scan path are currently hardcoded in `uploader/backup.go` and `main.go` respectively.

### Run

```bash
make run      # go run .
```

### Build

Produces a static binary (no GLIBC dependency, good for older Linux):

```bash
make build    # CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o gouploader .
```

### Deploy

Builds and deploys to a remote host over SSH, restarting its `systemd` user service:

```bash
make deploy
```

## Database

The schema (`database/schema.sql`) is embedded into the binary and applied automatically on first run if the `files` table doesn't exist. Queries are defined in `database/query.sql` and generated into the `sqlc` package.

To regenerate the query code after editing `database/query.sql` or the schema:

```bash
sqlc generate
```

## Project structure

```
adapters/   Per-host upload implementations + the Adapter interface
config/     Environment loading and logger setup
database/   SQLite connection, schema, and ORM wrapper
sqlc/       Generated type-safe query code
uploader/   Pipeline stages: scan, process, archive, clean up
website/    HTTP client for the website import API
main.go     Entry point and the main loop
```
