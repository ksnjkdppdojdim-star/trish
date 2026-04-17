# Trish - Remote PC Management System

Backend-only system for managing and auditing office PCs over LAN.

## Architecture

- **core/**: Plugin registry, event bus, agent/client interface
- **plugins/**: Extensible command plugins (ipconfig, cd, dir)
- **main.go**: CLI terminal interface

## Build & Run

```bash
go mod tidy
go build -o trish .
./trish list
./trish info <agent-id>
./trish exec <agent-id> ipconfig
./trish exec <agent-id> dir /path
./trish exec <agent-id> cd /path
```

## Docker

```bash
docker build -t trish:latest .
docker run --rm trish list
docker run --rm trish exec <agent-id> ipconfig
```
