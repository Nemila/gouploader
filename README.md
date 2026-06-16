# gouploader — Code Audit & Cleanup Plan

## Confirmed Bugs

---

### Bug 1 — `WalkDir` callback doesn't guard against nil `d` (`scanner.go:38-40`)

When `WalkDir` encounters a permission error on a subdirectory, it calls your callback with `err != nil` and `d == nil`. Your callback jumps straight to `d.IsDir()` → **nil pointer dereference, crash**.

```go
// current
err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
    if d.IsDir() { // panics if d == nil

// fix
    if err != nil {
        return nil // skip unreadable entries
    }
    if d.IsDir() {
```

---

### Bug 2 — File descriptor leak in scanner (`scanner.go:43-48`)

`os.Open(p)` is called only to call `.Name()` on the result, which returns the same path `p`. The file is never closed. On a large media folder this exhausts file descriptors.

```go
// current
file, err := os.Open(p)
...
fileName := filepath.Base(file.Name())  // same as filepath.Base(p)

// fix — delete the os.Open entirely
fileName := filepath.Base(p)
```

---

### Bug 3 — `errors.Is` arguments are inverted (`processor.go:64`)

```go
// current — checks if sql.ErrNoRows wraps err, which is never true
if err != nil && !errors.Is(sql.ErrNoRows, err) {

// fix
if err != nil && !errors.Is(err, sql.ErrNoRows) {
```

As written, a fresh file with no upload rows hits the error branch and `uploads` is nil, but the code continues to use it — silent misbehavior on every first-time file.

---

### Bug 4 — Sendvid `getContentLength` always fails (`sendvid.go:31-58`)

`w.Close()` is called on line 34 before any fields are written. Every subsequent `WriteField`/`CreateFormFile` call returns an error. `getContentLength` always returns `(0, error)`. `Upload()` then returns `""` for every file. **Sendvid is completely broken.**

```go
// fix: remove the premature Close
func (s *Sendvid) getContentLength(filePath, token string) (int64, error) {
    buf := &bytes.Buffer{}
    w := multipart.NewWriter(buf)
    // NO w.Close() here

    if err := w.WriteField("authenticity_token", token); err != nil {
    ...
    w.Close() // close AFTER writing all fields
    return int64(buf.Len()) + i.Size(), nil
}
```

---

### Bug 5 — `backup.go` reads entire video file into RAM (`backup.go:25-28`)

```go
b, err := io.ReadAll(file)  // could be gigabytes
video := tgbotapi.NewVideo(chatId, tgbotapi.FileBytes{Bytes: b, ...})
```

For a 2 GB video, this allocates 2 GB on the heap. Fix: use `tgbotapi.NewVideo` with a `FileReader` to stream the file.

```go
video := tgbotapi.NewVideo(chatId, tgbotapi.FileReader{
    Name:   filepath.Base(filePath),
    Reader: file,
})
```

---

### Bug 6 — `panic` inside `ScanFolder` loop (`scanner.go:27`)

```go
if err := orm.Queries.InsertFile(ctx, filePath); err != nil {
    panic(err.Error())
}
```

`InsertFile` uses `INSERT OR IGNORE`, so duplicate files don't error. But any real DB error (locked, disk full) panics the entire daemon inside the loop. Should return the error to the caller.

---

### Bug 7 — Cooldown sleep ignores context cancellation (`processor.go:115`, `main.go:64`)

```go
time.Sleep(uploadCooldown)  // blocks 2 minutes after a graceful shutdown signal
```

Fix with a select:

```go
select {
case <-time.After(uploadCooldown):
case <-ctx.Done():
    return ctx.Err()
}
```

Same problem in `main.go:64` (5-minute sleep).

---

### Bug 8 — Telegram failure stops all cleanup (`cleaner.go:23-26`)

```go
uploaded, err := UploadToChannel(...)
if err != nil {
    return err  // drops all remaining files
}
```

One flaky Telegram upload abandons the rest of the batch. Should log the error and `continue`.

---

### Bug 9 — `os.Remove` and DB update errors silently swallowed (`cleaner.go:29-33`)

```go
orm.Queries.UpsertFile(...)  // error ignored
os.Remove(file.FilePath)     // error ignored
```

A file could fail to delete (permission issue, already gone) but be marked `saved` anyway, disappearing from all future retries with no trace.

---

### Bug 10 — `os.Exit(0)` on signal aborts in-flight uploads (`main.go:38-47`)

The goroutine watching `ctx.Done()` calls `os.Exit(0)` immediately after the DB reset. The main goroutine may be mid-upload or mid-DB write. The fix is to let the main loop exit naturally by checking `ctx.Err()` after the sleep, not by calling `os.Exit`.

---

### Bug 11 — `panic` in main loop instead of error handling (`main.go:51-61`)

Any error from `ScanFolder`, `ProcessFiles`, or `CleanUp` crashes the daemon. These should be logged and the loop should continue or back off, not die.

---

## Design Problems

---

### Design 1 — `Adapter` interface has no context (`adapters/config.go:12-14`)

```go
type Adapter interface {
    Upload(filePath string) (string, error)
}
```

Uploads cannot be cancelled. The parent `errgroup` context (`groupCtx`) is already wired up but the adapters all use `context.Background()` internally. Fix: `Upload(ctx context.Context, filePath string) (string, error)`.

---

### Design 2 — Shared cookie jar across all adapters (`adapters/config.go:17`)

```go
var jar, _ = cookiejar.New(nil)
// used by hydrax, uqload, vidhide, sendvid adapters
```

All four services share one jar. Cookies from uqload are sent to vidhide and vice versa. Each adapter needs its own `cookiejar`.

---

### Design 3 — Adapters initialized at package init time (`adapters/config.go:19-50`)

Package-level `var Adapters = map[string]Adapter{...}` with `os.Getenv` calls at declaration time means:

- Adapters are initialized before `godotenv.Load()` in `config.Load()` runs if import order changes
- Impossible to unit-test adapters without real env vars
- `godotenv.Load()` is called a second time here (also called in `config.Load()`)

Fix: accept a `Config` struct and build the adapters map in `main()` after config is loaded.

---

### Design 4 — Config struct carries adapter credentials it never uses

`config.Config` has `AbyssKey`, `VidhideKey`, etc., but adapters read directly from `os.Getenv`. The config fields are populated but never passed anywhere — dead weight.

---

### Design 5 — No retry limit despite `retry_count` column

`upload_jobs.retry_count` is incremented on every upsert but never read. A permanently broken adapter (wrong API key, banned account) will retry every 5 minutes forever, burning rate limits. Fix: skip files where `retry_count > N` and mark them `failed_permanent`.

---

### Design 6 — No structured logging

`slog.Logger` is created in `main()` but immediately discarded — none of the uploader/adapter/cleaner packages use it. All output is `fmt.Printf`. This makes log filtering, log levels, and machine-parseable output impossible. Pass the logger down (or use `slog.Default()`).

---

### Design 7 — Telegram bot instantiated per file (`backup.go:14`)

```go
bot, err := tgbotapi.NewBotAPIWithAPIEndpoint(token, endpoint)
```

A new API client (with its own HTTP client, handshake, `getMe` call) is created for every single file. Initialize it once and reuse it.

---

## Code Quality

---

### Quality 1 — Regex compiled on every filename check (`scanner.go:13-16`)

```go
func isValidName(fileName string) bool {
    re := regexp.MustCompile(`...`)  // compiled per file
```

Move to a package-level `var`:

```go
var fileNameRe = regexp.MustCompile(`(?i)^(.+?)-(tv|movie)-...`)
```

---

### Quality 2 — `importResponse` type is unused (`main.go:19-21`)

Dead code. Delete it.

---

### Quality 3 — DB queries missing indexes (`schema.sql`)

`GetFilesByStatus` (`WHERE status = ?`) and `GetFileUploads` (`WHERE file_id = ?`) do full scans. Add:

```sql
CREATE INDEX idx_files_status ON files(status);
CREATE INDEX idx_upload_jobs_file_id ON upload_jobs(file_id);
```

---

### Quality 4 — SQLite foreign keys not enforced (`database/orm.go`)

SQLite foreign keys default to OFF. The `files` → `upload_jobs` FK does nothing without:

```
?_foreign_keys=on
```

added to the DSN.

---

### Quality 5 — `folderExists` has redundant branches (`config/config.go:27-37`)

Both error branches return `false`. The inner `if` does nothing.

---

## Cleanup Plan (Priority Order)

### Phase 1 — Crash & data-loss fixes

1. Fix `WalkDir` nil `d` guard (`scanner.go:38`)
2. Fix `errors.Is` inversion (`processor.go:64`)
3. Fix Sendvid `w.Close()` order (`sendvid.go:34`)
4. Replace `io.ReadAll` with `FileReader` in backup (`backup.go`)
5. Fix file descriptor leak — remove `os.Open` in scanner (`scanner.go:43`)
6. Handle `os.Remove`/`UpsertFile` errors in cleaner; `continue` on telegram errors
7. Remove `os.Exit(0)` — let main loop drain naturally
8. Replace `panic` in main loop and `ScanFolder` with `log.Error` + `continue`

### Phase 2 — Correctness & reliability

9. Add context-aware sleep (main loop + processor cooldown)
10. Fix `Adapter` interface to accept `context.Context`; propagate through all adapters
11. Give each adapter its own cookie jar
12. Move adapter initialization out of package-level vars into a constructor called from `main()`
13. Implement `retry_count` enforcement — skip after N failures, log it

### Phase 3 — Scalability & performance

14. Compile regex once at package level
15. Add DB indexes on `status` and `file_id`
16. Enable SQLite foreign key enforcement in DSN
17. Initialize Telegram bot once and reuse
18. Remove the `Config` fields for adapter credentials (they're unused)

### Phase 4 — Dev experience

19. Pass `slog.Logger` down to all packages — remove all `fmt.Printf` log lines
20. Delete dead code (`importResponse`)
21. Simplify `folderExists`
22. Add a `Makefile` with `build`, `run`, `test`, `lint` targets
23. Add at minimum one integration test for the scan→process→cleanup cycle using a temp SQLite DB
