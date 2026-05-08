# Trish Future Features Roadmap

Backlog des fonctionnalites a envisager pour les prochaines iterations.

## Lecture De La Roadmap

Chaque item indique si une reinstall ou mise a jour de l'executable agent est probablement necessaire.

- `Agent update: non`: implementation cote CLI, serveur, GUI ou plugin dynamique.
- `Agent update: possible`: faisable sans update agent en premiere version, mais mieux avec support natif plus tard.
- `Agent update: oui`: necessite un nouveau comportement dans l'agent.

## Plugins

- [x] `plugin enable <name>` / `plugin disable <name>` - Agent update: non
- [x] Gestion des versions de plugins - Agent update: non
- [x] Rollback plugin vers une version precedente - Agent update: non
- [x] `plugin test <path-or-git-url>` avant installation - Agent update: non
- [x] Verification checksum des plugins - Agent update: non
- [ ] Signature cryptographique des plugins - Agent update: non
- [x] Permissions declaratives dans le manifest plugin - Agent update: possible si enforcement cote agent
- [ ] Page GUI pour installer, mettre a jour, supprimer et auditer les plugins - Agent update: non

## Agents

- [ ] `agent update <agent|all>` pour mettre a jour l'agent a distance - Agent update: oui pour installer le premier updater fiable
- [x] Groupes d'agents: `group create`, `group list`, `group exec` - Agent update: non
- [x] Tags agents: `prod`, `hr`, `finance`, `laptop`, etc. - Agent update: non
- [x] Execution parallele: `exec --all` et `exec --group <name>` - Agent update: non
- [ ] Alertes quand un agent passe offline - Agent update: non
- [ ] Inventaire machine: OS, RAM, disque, utilisateurs, logiciels installes - Agent update: possible si on veut du natif, non si plugin dynamique
- [ ] Monitoring basique: CPU, RAM, disque, reseau - Agent update: possible, oui pour un monitoring push continu

## CLI

- [ ] Historique persistant des commandes CLI - Agent update: non
- [ ] Auto-completion dans le shell interactif - Agent update: non
- [ ] Upload/download de fichiers depuis le CLI, hors GUI - Agent update: possible si protocole natif optimise, non via plugin/superexec
- [ ] Planification: executer une commande a une heure donnee - Agent update: possible, non si serveur planifie
- [ ] Meilleurs messages d'erreur avec suggestions de commandes proches - Agent update: non

## GUI

- [ ] Vue agents avec filtres, tags, groupes et statut temps reel - Agent update: non
- [ ] Vue plugins avec versions, sources, commandes exposees et statut - Agent update: non
- [ ] Logs en temps reel dans la GUI - Agent update: possible si streaming agent natif
- [ ] Historique des commandes executees depuis la GUI - Agent update: non
- [ ] Actions multi-agents depuis la GUI - Agent update: non

## Audit Et Securite

- [ ] Audit log serveur: qui a lance quoi, quand, sur quel agent - Agent update: non
- [ ] Historique d'execution par agent - Agent update: possible si journal local agent, non si serveur seulement
- [ ] Export des logs d'audit - Agent update: non
- [ ] Validation stricte des permissions plugins avant execution - Agent update: possible si enforcement cote agent
- [ ] Politique d'autorisation par admin ou role - Agent update: non

## Remote Operations

- [ ] Screenshot distant - Agent update: possible, non via plugin PowerShell simple
- [ ] Capture de fenetre active - Agent update: possible, probablement oui pour une version fiable
- [ ] Collecte d'informations systeme detaillees - Agent update: non si plugin dynamique
- [ ] Execution de commandes longues avec suivi de progression - Agent update: oui pour une implementation propre
- [ ] Annulation de commande distante en cours - Agent update: oui

## Regle Pratique

- Tout ce qui est orchestration, affichage, stockage serveur, audit serveur, plugins dynamiques, groupes et tags ne devrait pas obliger a reinstaller les agents.
- Tout ce qui demande a l'agent de gerer un nouveau cycle de vie natif, par exemple job long, progression, annulation, streaming ou monitoring push, demandera une mise a jour agent.
- Priorite recommandee: implementer `agent update <agent|all>` assez tot pour eviter les reinstallations manuelles sur tous les postes employes.
