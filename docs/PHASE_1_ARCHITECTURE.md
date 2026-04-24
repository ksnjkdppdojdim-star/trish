# Phase 1 - Refonte Architecture

Ce document transforme les decisions produit de la phase 0 en design technique exploitable.

## Objectif

Passer d'une implementation prototype a une architecture reseau claire, centralisee et extensible, ou :

- `trish-cli` ne parle qu'au serveur
- `trish-agent` maintient une connexion sortante vers le serveur
- `trish-server` orchestre les commandes, l'etat et les reponses

## Probleme de l'Implementation Actuelle

L'implementation actuelle fonctionne comme preuve de concept, mais elle n'est pas encore la bonne architecture cible :

- l'agent ecoute localement sur un port entrant
- le serveur recontacte l'agent en direct pour executer une commande
- le transport est base sur des connexions courtes
- il n'y a pas de notion formelle de session agent
- les messages sont encore minimaux

Ce modele est acceptable pour tester, mais pas ideal en environnement entreprise.

## Architecture Cible Retenue

Architecture cible :

1. `trish-agent` ouvre une connexion persistante sortante vers `trish-server`
2. `trish-server` conserve cette session en memoire
3. `trish-cli` envoie les commandes au serveur
4. `trish-server` route les commandes sur la session active de l'agent
5. `trish-agent` execute et renvoie la reponse sur la meme session

Avantages :

- pas de port entrant a exposer sur chaque PC employe
- plus simple avec firewall d'entreprise
- meilleure gestion des deconnexions/reconnexions
- base propre pour heartbeat, auth, audit et timeout

## Topologie Officielle

Topologie retenue :

```text
Admin PC(s) ----> trish-cli ---->
                               trish-server <---- connexion persistante ---- trish-agent ---- PC employe
Admin PC(s) ----> trish-cli ---->
```

Regles :

- tout passe par `trish-server`
- aucun acces direct `cli -> agent`
- l'agent est initie par la machine distante, pas par le serveur

## Flux Techniques

### Flux 1 - Connexion d'un agent

1. le service `trish-agent` demarre
2. il charge sa configuration locale
3. il ouvre une connexion TCP vers `trish-server`
4. il envoie `agent.register`
5. le serveur cree ou met a jour la session
6. le serveur repond `register.ack`
7. l'agent reste connecte

### Flux 2 - Heartbeat

1. l'agent envoie periodiquement `agent.heartbeat`
2. le serveur met a jour `last_seen`
3. si la connexion tombe ou si le heartbeat expire, l'agent passe offline

### Flux 3 - Execution de commande

1. le CLI envoie `cli.exec`
2. le serveur valide la cible et cree une commande en attente
3. le serveur pousse `server.exec.dispatch` a l'agent connecte
4. l'agent execute le plugin
5. l'agent renvoie `agent.exec.result`
6. le serveur persiste l'evenement
7. le serveur renvoie le resultat au CLI

### Flux 4 - Reconnexion agent

1. la connexion agent tombe
2. le serveur marque la session comme fermee
3. l'agent tente de se reconnecter
4. une nouvelle session remplace l'ancienne
5. l'etat agent est remis a jour

## Etats d'un Agent

Les etats officiels de l'agent seront :

- `installing`
- `online`
- `offline`
- `degraded`
- `removed`

### Definition des etats

`installing`

- agent en cours d'installation ou d'enrolement initial

`online`

- agent connu
- session active ou heartbeat valide
- commandes possibles

`offline`

- agent connu mais sans session valide
- commandes impossibles

`degraded`

- agent joignable ou partiellement enregistre
- problemes de compatibilite, plugin, version ou sante

`removed`

- agent retire du parc
- ne doit plus etre ciblable

## Session Agent

La session agent est une notion technique differente de la fiche d'inventaire persistante.

### Donnees de session

- `session_id`
- `agent_id`
- `connected_at`
- `last_heartbeat_at`
- `remote_addr`
- `protocol_version`
- `authenticated`

### Donnees persistantes agent

- `agent_id`
- `machine_id`
- `hostname`
- `ip`
- `version`
- `commands`
- `status`
- `last_seen`

## Protocole de Messages

Le protocole V1 reste base sur JSON lignes, avec un envelope plus explicite.

## Enveloppe Cible

```json
{
  "type": "agent.register",
  "request_id": "req-123",
  "agent_id": "pc-001",
  "timestamp": "2026-04-21T15:00:00Z",
  "payload": {}
}
```

Champs cibles :

- `type`
- `request_id`
- `agent_id`
- `timestamp`
- `payload`

Principes :

- `request_id` sert a corriger le couplage requete/reponse
- `type` remplace progressivement la notion trop generique de `Action`
- `payload` permet d'evoluer sans casser la structure globale

## Types de Messages V1

### Messages agent -> server

- `agent.register`
- `agent.heartbeat`
- `agent.exec.result`
- `agent.log`
- `agent.health`

### Messages server -> agent

- `server.register.ack`
- `server.exec.dispatch`
- `server.ping`
- `server.shutdown.notice`

### Messages cli -> server

- `cli.list`
- `cli.info`
- `cli.exec`
- `cli.ping`
- `cli.version`

### Messages server -> cli

- `server.list.result`
- `server.info.result`
- `server.exec.accepted`
- `server.exec.result`
- `server.error`

## Charges Utiles Initiales

### `agent.register`

Payload minimal :

- `machine_id`
- `hostname`
- `ip`
- `version`
- `commands`
- `os`

### `agent.heartbeat`

Payload minimal :

- `status`
- `uptime`
- `version`
- `health`

### `server.exec.dispatch`

Payload minimal :

- `command_id`
- `plugin`
- `args`
- `timeout_seconds`

### `agent.exec.result`

Payload minimal :

- `command_id`
- `success`
- `stdout`
- `stderr`
- `exit_code`
- `duration_ms`

### `cli.exec`

Payload minimal :

- `agent_id`
- `plugin`
- `args`

## Schema de Correlation

Deux identifiants sont necessaires :

- `request_id` pour tracer l'echange technique
- `command_id` pour tracer une commande metier de bout en bout

Regle :

- une execution distante a un `command_id` unique de la demande CLI jusqu'au resultat final

## Timeouts et Comportement

Time-box V1 recommandee :

- connexion agent -> serveur : retry infini avec backoff
- heartbeat : toutes les 30 secondes
- offline timeout : 90 secondes sans heartbeat
- timeout exec simple : 30 secondes par defaut
- timeout de requete CLI : configurable

## Convergence depuis l'Implementation Actuelle

Etat actuel :

- `ActionRegister`, `ActionExec`, `ActionRun`, `ActionList`, `ActionInfo`, `ActionHealth`
- connexions courtes
- serveur qui rappelle l'agent

Convergence cible :

1. introduire une enveloppe de message plus explicite
2. conserver temporairement la compatibilite interne
3. introduire une session agent persistante cote serveur
4. migrer `exec` vers un dispatch sur session existante
5. supprimer ensuite le listener entrant permanent de l'agent

## Plan de Migration Technique

### Etape 1

- garder le JSON ligne
- enrichir le protocole avec types et IDs
- ajouter `machine_id`
- formaliser les structures de message

### Etape 2

- ajouter un gestionnaire de sessions agent cote serveur
- memoriser les connexions actives
- separer inventaire agent et session agent

### Etape 3

- faire porter la commande via la session agent persistante
- ne plus faire `net.Dial()` du serveur vers l'agent pour chaque exec

### Etape 4

- retirer l'obligation d'un port entrant sur l'agent
- reserver le listener entrant a du debug temporaire si besoin

## Compatibilite V1

Pour avancer sans tout casser d'un coup :

- le transport actuel peut etre maintenu temporairement tant que la refonte session n'est pas faite
- toute nouvelle structure doit aller dans le sens de `agent -> server` persistant
- on n'ajoute plus de nouvelles fonctions dependantes du modele `server -> dial agent`

## Decisions Figees en Fin de Phase 1

- le modele cible officiel est la connexion persistante sortante `agent -> server`
- le serveur garde des sessions agents actives
- le CLI reste totalement decouple des agents
- le protocole doit evoluer vers un envelope de message typee
- les etats agents `installing / online / offline / degraded / removed` sont retenus
- l'implementation actuelle de rappel direct serveur -> agent devient une etape transitoire et non la cible

## Livrables de Phase 1

- schema de protocole : ce document
- diagramme de flux : decrit dans ce document
- etats agent : fixes dans ce document
