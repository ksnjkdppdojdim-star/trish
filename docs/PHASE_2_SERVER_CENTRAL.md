# Phase 2 - Serveur Central Unique

Cette phase implemente la premiere version exploitable du serveur central unique et du suivi actif des agents.

## Objectif

Donner a `trish-server` un vrai role d'orchestrateur central en :

- tenant un registre persistant des agents
- gardant les sessions agents actives
- dispatchant les commandes via ces sessions
- marquant les agents offline en cas d'absence
- empechant plusieurs serveurs concurrents sur le meme host

## Ce qui a ete implemente

## 1. Sessions agents actives cote serveur

Le serveur maintient maintenant des sessions en memoire pour les agents connectes.

Concretement :

- un agent se connecte au serveur
- il envoie `agent.register`
- le serveur enregistre la session
- `cli exec` est route via cette session active
- le serveur n'ouvre plus une nouvelle connexion vers l'agent pour chaque commande

Impact :

- on bascule reellement vers le modele `agent -> server` persistant
- le modele precedent `server -> dial agent` n'est plus le flux principal

## 2. Registre agent enrichi

Les entrees du registre contiennent maintenant en plus :

- `machine_id`
- `version`
- `status`
- `last_seen`

Statuts utilises a ce stade :

- `online`
- `offline`

Les autres statuts de la roadmap restent a exploiter dans les phases suivantes.

## 3. Heartbeat et bascule offline

L'agent envoie des heartbeats periodiques au serveur.

Le serveur :

- met a jour `last_seen`
- conserve l'etat online tant que les heartbeats arrivent
- marque l'agent offline apres expiration

Base actuelle :

- heartbeat : 30 secondes
- bascule offline : 90 secondes sans signal

## 4. Verrou d'unicite du serveur sur l'hote

Un verrou de processus a ete ajoute pour empecher plusieurs `trish-server` concurrents sur la meme machine.

Concretement :

- un fichier lock est cree au demarrage
- un second serveur sur le meme host echoue au lancement

Note :

- cela garantit l'unicite locale sur l'hote
- l'unicite logique d'environnement sera completement encadree plus tard avec `trish start` et les conventions de deploiement

## 5. CLI toujours centralise

Le CLI continue de parler uniquement au serveur, mais passe maintenant par le nouveau protocole de messages typees.

Commandes conservees :

- `list`
- `info`
- `exec`

## Etat du systeme apres phase 2

Flux courant :

1. `trish-agent` ouvre une connexion vers `trish-server`
2. `trish-server` garde cette session
3. `trish-cli exec ...` envoie la commande au serveur
4. le serveur pousse `server.exec.dispatch`
5. l'agent repond avec `agent.exec.result`
6. le serveur renvoie le resultat au CLI

## Limitations restantes

Ce n'est pas encore termine pour la phase produit globale :

- `trish start` n'est pas encore implemente
- l'agent n'est pas encore un vrai service Windows
- le lock serveur est local a la machine et non un mecanisme d'orchestration global
- la securite reseau n'est pas encore en place
- l'audit detaille n'est pas encore implemente

## Fichiers touches dans cette phase

- `core/protocol.go`
- `core/client.go`
- `core/registry.go`
- `server/server.go`
- `server/session.go`
- `server/lock.go`
- `agent/agent.go`
- `cmd/server/main.go`
- `cmd/agent/main.go`

## Resultat de phase 2

La base serveur central unique est maintenant posee :

- transport oriente sessions agents
- metadata agent enrichies
- heartbeat/offline de base
- verrou d'unicite locale

La prochaine phase logique reste :

- transformation de `trish-agent.exe` en bootstrap d'installation + service Windows
