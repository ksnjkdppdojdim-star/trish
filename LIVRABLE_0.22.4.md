# Trish 0.22.4

Livrable fonctionnel minimal.

## Build

Depuis `M:\trish\trish` :

```powershell
.\build.ps1
```

## Binaries

- `trish-server.exe` : serveur central
- `trish-agent.exe` : installateur / agent Windows
- `trish-cli.exe` : client admin
- `trish.exe` : alias CLI principal

## Usage simple

### Serveur

Lancer :

```powershell
.\trish-server.exe
```

Notes :

- nettoie automatiquement un lock stale
- garde la fenetre ouverte 10 secondes si un vrai conflit serveur existe

### Agent

Lancer sur la machine cible :

```powershell
.\trish-agent.exe --server=<IP_DU_SERVEUR> --port=9999
```

Au double-clic :

- demande elevation admin
- installe le service Windows
- copie les fichiers dans `C:\ProgramData\Trish\agent`

### CLI

Lancer :

```powershell
.\trish-cli.exe
```

Sans argument, ouvre un shell interactif.

## Commandes utiles

```powershell
.\trish.exe --server=<IP_DU_SERVEUR> --port=9999 list
.\trish.exe --server=<IP_DU_SERVEUR> --port=9999 info <agent-id>
.\trish.exe --server=<IP_DU_SERVEUR> --port=9999 ping <agent-id>
.\trish.exe --server=<IP_DU_SERVEUR> --port=9999 exec <agent-id> ipconfig
.\trish.exe --server=<IP_DU_SERVEUR> --port=9999 agent freeze <agent-id>
.\trish.exe --server=<IP_DU_SERVEUR> --port=9999 agent unfreeze <agent-id>
.\trish.exe --server=<IP_DU_SERVEUR> --port=9999 agent stop <agent-id>
```

## Logs agent

- config : `C:\ProgramData\Trish\agent\agent-config.json`
- logs : `C:\ProgramData\Trish\agent\logs\agent.log`
