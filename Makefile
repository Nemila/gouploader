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
	ssh ultra "mkdir -p ~/scripts && systemctl --user stop gouploader.service"
	scp gouploader ultra:~/scripts
	ssh ultra "systemctl --user start gouploader.service"
	@echo "✅ Deployment successful! Binary copied to ~/scripts"
