CREATE TABLE files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_path TEXT UNIQUE NOT NULL,
    discovered_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    status TEXT DEFAULT 'PENDING', -- PENDING | PROCESSING | DONE | MISSING
    error TEXT
);

CREATE TABLE upload_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_id INTEGER NOT NULL REFERENCES files(id),
    host_name TEXT NOT NULL,
    status TEXT DEFAULT 'PENDING', -- FAILED | DONE | PENDING 
    retry_count INTEGER DEFAULT 0,
    last_error TEXT,
    embed_id TEXT,
    updated_at DATETIME,
    UNIQUE(file_id, host_name)
);