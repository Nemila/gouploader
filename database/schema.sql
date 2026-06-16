CREATE TABLE files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_path TEXT UNIQUE NOT NULL,
    discovered_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    status TEXT DEFAULT 'pending' NOT NULL 
    -- PENDING | PROCESSING | DONE | SAVED
);

CREATE TABLE upload_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_id INTEGER NOT NULL REFERENCES files(id),
    host_name TEXT NOT NULL,
    status TEXT DEFAULT 'pending' NOT NULL, -- FAILED | DONE | PENDING 
    retry_count INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    slug TEXT,
    UNIQUE(file_id, host_name)
);