# Trish Future Features Roadmap

Backlog des fonctionnalites a envisager pour les prochaines iterations.

## Plugins

- [ ] `plugin enable <name>` / `plugin disable <name>`
- [ ] Gestion des versions de plugins
- [ ] Rollback plugin vers une version precedente
- [ ] `plugin test <path-or-git-url>` avant installation
- [ ] Verification checksum/signature des plugins
- [ ] Permissions declaratives dans le manifest plugin
- [ ] Page GUI pour installer, mettre a jour, supprimer et auditer les plugins

## Agents

- [ ] `agent update <agent|all>` pour mettre a jour l'agent a distance
- [ ] Groupes d'agents: `group create`, `group list`, `group exec`
- [ ] Tags agents: `prod`, `hr`, `finance`, `laptop`, etc.
- [ ] Execution parallele: `exec --all` et `exec --group <name>`
- [ ] Alertes quand un agent passe offline
- [ ] Inventaire machine: OS, RAM, disque, utilisateurs, logiciels installes
- [ ] Monitoring basique: CPU, RAM, disque, reseau

## CLI

- [ ] Historique persistant des commandes CLI
- [ ] Auto-completion dans le shell interactif
- [ ] Upload/download de fichiers depuis le CLI, hors GUI
- [ ] Planification: executer une commande a une heure donnee
- [ ] Meilleurs messages d'erreur avec suggestions de commandes proches

## GUI

- [ ] Vue agents avec filtres, tags, groupes et statut temps reel
- [ ] Vue plugins avec versions, sources, commandes exposees et statut
- [ ] Logs en temps reel dans la GUI
- [ ] Historique des commandes executees depuis la GUI
- [ ] Actions multi-agents depuis la GUI

## Audit Et Securite

- [ ] Audit log serveur: qui a lance quoi, quand, sur quel agent
- [ ] Historique d'execution par agent
- [ ] Export des logs d'audit
- [ ] Validation stricte des permissions plugins avant execution
- [ ] Politique d'autorisation par admin ou role

## Remote Operations

- [ ] Screenshot distant
- [ ] Capture de fenetre active
- [ ] Collecte d'informations systeme detaillees
- [ ] Execution de commandes longues avec suivi de progression
- [ ] Annulation de commande distante en cours
