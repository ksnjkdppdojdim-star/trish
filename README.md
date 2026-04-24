# Trish - Remote PC Management System

Trish is a lightweight backend for discovering office PCs on a LAN, registering them on a central server, and running a small set of remote audit commands.

## Current Status

Implemented:

- Central TCP server with persistent agent registry
- Remote agent service with heartbeat/refresh registration
- Admin CLI connected to the server
- Remote commands: `ipconfig`, `dir`, `cd`
- Interactive shell mode: `trish shell`

Not implemented yet:

- Authentication / encryption
- Multi-user access control
- Rich audit history
- Auto-discovery beyond explicit agent registration

## Architecture

- `cmd/server`: central registry and command relay
- `cmd/agent`: service running on managed PCs
- `cmd/cli`: admin CLI
- `core/`: protocol, registry, plugin system
- `plugins/`: remote commands
- `main.go`: shortcut entrypoint for the CLI

## Build

```bash
go build -o trish .
go build -o trish-server ./cmd/server
go build -o trish-agent ./cmd/agent
```

Version courante: `1.23.00406`

## .env

Le fichier `.env` sert au build pour injecter les valeurs par defaut dans les
binaires `.exe`. Une fois compiles, `trish.exe`, `trish-server.exe` et
`trish-agent.exe` utilisent ces valeurs embarquees. Les flags en ligne de
commande restent prioritaires si tu veux surcharger ponctuellement.

Exemple:

```env
TRISH_SERVER_ADDR=192.168.100.209
TRISH_SERVER_PORT=9999
TRISH_AGENT_LISTEN_PORT=2222
```

Variables supportees:

- `TRISH_SERVER_ADDR`: adresse IP ou nom DNS du serveur central
- `TRISH_SERVER_PORT`: port TCP du serveur central
- `TRISH_AGENT_LISTEN_PORT`: port annonce par l'agent
- `TRISH_SERVER_REGISTRY_PATH`: chemin du registre serveur
- `TRISH_SERVER_LOCK_PATH`: chemin du lock serveur
- `TRISH_ALLOW_LOOPBACK_SERVER`: autorise `127.0.0.1` pour un install agent local de test

## Run Locally

Start the server:

```bash
./trish-server
```

Start an agent:

```bash
./trish-agent -server=127.0.0.1 -port=9999 -listen=2222
```

For a managed employee PC, do not leave the default `127.0.0.1`.
Point the agent to the central server instead:

```bash
trish-agent.exe install -server=<admin-pc-ip> -port=9999
```

If the binary was built with the right embedded server values, the install can simply be:

```bash
trish-agent.exe install
```

If you really want a local-only installation for testing on the same machine,
you must opt in explicitly:

```bash
trish-agent.exe install -server=127.0.0.1 --allow-loopback-server
```

Use the CLI:

```bash
./trish list
./trish info <agent-id>
./trish exec <agent-id> ipconfig
./trish exec <agent-id> dir .
./trish exec <agent-id> cd ..
./trish shell
```

## Docker

Build the image:

```bash
docker build -t trish:latest .
```

Run the CLI container against a server reachable from the container:

```bash
docker run --rm trish:latest -server=<server-host> -port=9999 list
docker run --rm trish:latest -server=<server-host> -port=9999 exec <agent-id> ipconfig
```

## Notes

- `cd` changes the working directory inside the running agent session.
- The registry file is stored by default in `~/.trish/registry.json`.
- For LAN usage, launch one `trish-agent` per managed machine.
