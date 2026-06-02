# Run the local development server
run:
	go run .

# Build for older Linux systems
# GOOS=linux targets Linux.
# GOARCH=amd64 targets standard 64-bit Intel/AMD processors.
# CGO_ENABLED=0 creates a static binary that doesn't depend on system GLIBC versions (perfect for old Linux).
build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o gouploader .

# Build and safely deploy to the VPS
deploy: build
	ssh ultra "mkdir -p ~/scripts"
	scp gouploader ultra:~/scripts
	@echo "✅ Deployment successful! Binary copied to ~/scripts"

# Wipe SQLite database clean and re-apply schema without deleting the file
db-reset:
	sqlite3 database.db "PRAGMA writable_schema = 1; delete from sqlite_master where type in ('table', 'index', 'trigger'); PRAGMA writable_schema = 0; VACUUM;"
	sqlite3 database.db < schema.sql
	@echo "✅ Database wiped and schema.sql applied successfully!"