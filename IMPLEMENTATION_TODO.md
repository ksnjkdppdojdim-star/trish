# Trish Implementation TODO

Ce document sert de backlog principal pour construire `Trish` comme un systeme de gestion distante de PC utilisable en entreprise.

## Vision

`Trish` doit permettre, depuis un poste admin, d'executer des commandes encadrees sur des PC de travail distants via :

- un seul serveur central `trish-server`
- un agent `trish-agent` installe une fois sur chaque PC employe
- un ou plusieurs clients admin `trish-cli`

Flux cible :

1. l'admin lance une commande avec `trish-cli`
2. `trish-cli` envoie la requete a `trish-server`
3. `trish-server` route la commande vers `trish-agent`
4. `trish-agent` execute la commande
5. le resultat remonte vers `trish-server`
6. `trish-cli` affiche le resultat

## Principes Produit

- Un seul serveur central actif par environnement
- L'agent s'installe une seule fois sur le PC employe
- L'agent redemarre automatiquement apres reboot
- Le CLI sert uniquement a l'administration
- Les commandes distantes sont tracees
- Le systeme doit etre securise et administrable

## Etat Actuel

Deja present :

- separation de base `agent / server / cli`
- serveur TCP central
- enregistrement d'agents
- commandes de base `ipconfig`, `dir`, `cd`
- execution distante elementaire

Encore insuffisant pour la production :

- pas de service Windows pour l'agent
- pas de TLS
- pas d'authentification forte
- pas de connexion persistante agent -> serveur
- pas de vrai audit
- pas de gestion stricte du serveur unique

## Priorite Actuelle

Les deux chantiers a traiter maintenant avant le prochain redeploiement agent sont :

- [x] Renforcer l'authentification `trish-cli -> trish-server` avec signature, timestamp et nonce
- [x] Etendre `superexec` pour executer `cmd`, `powershell` et des executables directs sur le poste distant

## Priorites

### P0 - Fondations indispensables

- [x] Fixer l'architecture cible officielle
- [x] Imposer un seul `trish-server` central
- [x] Transformer `trish-agent` en vrai service Windows
- [x] Faire du double-clic de `trish-agent.exe` un mode installation
- [x] Ajouter auto-start et auto-restart du service Windows
- [x] Ajouter configuration propre serveur/agent
- [x] Stabiliser `list`, `info`, `exec`
- [x] Ajouter heartbeat fiable
- [ ] Ajouter logs de base cote agent et cote serveur

### P1 - Securite et robustesse

- [ ] Ajouter authentification agent <-> serveur
- [x] Ajouter authentification admin CLI <-> serveur
- [ ] Ajouter TLS
- [ ] Gerer les reconnexions proprement
- [ ] Ajouter timeouts, retries et erreurs standardisees
- [ ] Ajouter audit des commandes

### P2 - Exploitation et produit

- [ ] Ajouter packaging/deploiement propre
- [ ] Ajouter mises a jour de l'agent
- [ ] Ajouter commandes admin avancees
- [ ] Ajouter formats de sortie plus riches
- [ ] Ajouter tests d'integration complets
- [ ] Ajouter documentation d'exploitation

## Roadmap Detaillee

## Phase 0 - Cadrage Produit

- [x] Definir formellement les roles :
  - `trish-agent` sur chaque PC employe
  - `trish-server` comme serveur central unique
  - `trish-cli` sur les postes admin
- [x] Definir le flux officiel des commandes
- [x] Definir le perimetre V1
- [x] Definir les contraintes entreprise
- [x] Definir les cas d'usage prioritaires

Livrables :

- [x] note d'architecture cible
- [x] perimetre V1 valide
- [x] conventions de deploiement

## Phase 1 - Refonte Architecture

- [x] Supprimer toute ambiguite entre prototype local et architecture distribuee
- [x] Definir un protocole unique de messages
- [x] Faire converger tout le trafic via le serveur central
- [x] Evaluer et valider le mode cible :
  - connexion persistante sortante `agent -> server`
- [x] Definir les messages :
  - `register`
  - `heartbeat`
  - `exec request`
  - `exec result`
  - `list`
  - `info`
  - `health`
- [x] Definir les etats d'un agent :
  - `installing`
  - `online`
  - `offline`
  - `degraded`
  - `removed`

Livrables :

- [x] schema de protocole
- [x] diagramme de flux
- [x] etats agent documentes

## Phase 2 - Serveur Central Unique

- [x] Faire du `trish-server` l'unique autorite centrale
- [x] Ajouter un mecanisme anti double-instance
- [ ] Gerer le registre agents proprement
- [x] Ajouter les metadonnees agent :
  - `hostname`
  - `machine_id`
  - `ip`
  - `version`
  - `last_seen`
  - `status`
- [x] Ajouter heartbeat robuste
- [x] Ajouter expiration/offline apres absence de heartbeat
- [ ] Definir un stockage serveur propre
- [ ] Concevoir `trish start`
- [ ] Garantir qu'un second `trish start` remplace/refreshe sans creer une deuxieme instance mere

Livrables :

- [ ] serveur unique fiable
- [x] registre persistant
- [ ] cycle de vie agent stable

## Phase 3 - Agent Windows Professionnel

- [x] Transformer `trish-agent.exe` en bootstrap d'installation
- [x] Definir les modes :
  - `install`
  - `run-service`
  - `uninstall`
  - `repair`
- [x] Faire du double-clic le mode `install`
- [x] Afficher l'installation etape par etape
- [x] Verifier les droits admin
- [x] Copier les binaires dans un dossier fixe
- [x] Ecrire la configuration locale
- [x] Creer le service Windows `TrishAgent`
- [x] Configurer le demarrage automatique
- [x] Configurer le restart sur crash
- [x] Demarrer le service
- [x] Tester l'enregistrement vers le serveur
- [ ] Fermer la fenetre automatiquement apres succes
- [x] Ajouter uninstall propre
- [x] Ajouter mode repair/update

Livrables :

- [x] agent installable en un clic
- [x] service Windows fonctionnel
- [ ] persistence apres reboot

## Phase 4 - Securite Reseau

- [ ] Ajouter l'authentification des agents
- [x] Ajouter l'authentification des admins
- [x] Signer les requetes admin `cli -> server` avec HMAC
- [x] Ajouter timestamp et nonce anti-replay pour les requetes admin
- [ ] Ajouter TLS
- [ ] Definir une strategie de certificats ou tokens
- [ ] Proteger l'enrolement initial des agents
- [ ] Empcher les faux agents
- [ ] Empcher les CLI non autorises
- [ ] Ajouter rotation de secrets
- [ ] Ajouter compatibilite de version/protocole

Livrables :

- [ ] connexion chiffree
- [ ] enrlement securise
- [ ] acces admin controle

## Phase 5 - Moteur d'Execution des Commandes

- [ ] Definir une whitelist de commandes V1
- [ ] Normaliser les plugins
- [x] Etendre `superexec` avec modes `cmd`, `powershell`, `exec`
- [ ] Ajouter validation stricte des arguments
- [ ] Ajouter timeout par commande
- [ ] Ajouter taille max de sortie
- [ ] Ajouter code retour standard
- [ ] Ajouter structure de resultat :
  - `stdout`
  - `stderr`
  - `exit_code`
  - `duration`
- [ ] Gerer les erreurs de maniere coherente
- [ ] Prevoir annulation ou timeout visible cote CLI

Livrables :

- [ ] moteur d'execution stable
- [ ] format de resultat clair
- [ ] erreurs previsibles

## Phase 6 - Plugins et Commandes Metier

- [ ] Stabiliser `ipconfig`
- [ ] Stabiliser `dir`
- [ ] Stabiliser `cd`
- [ ] Definir la semantique exacte de `cd`
- [ ] Ajouter `pwd`
- [ ] Ajouter `hostname`
- [ ] Ajouter `whoami`
- [ ] Evaluer `ps` / `services`
- [ ] Ajouter convention de creation de plugin
- [ ] Ajouter tests unitaires par plugin

Livrables :

- [ ] plugins V1 robustes
- [ ] convention plugin documentee

## Phase 7 - CLI Admin

- [x] Faire du `trish-cli` un client admin pur
- [x] Stabiliser :
  - `list`
  - `info <agent>`
  - `exec <agent> <cmd>`
- [x] Ajouter `ping`
- [ ] Ajouter `version`
- [ ] Ajouter `logs`
- [ ] Ajouter ciblage multiple
- [ ] Ajouter sortie JSON optionnelle
- [ ] Ajouter historique local
- [ ] Ajouter mode interactif admin propre

Livrables :

- [ ] CLI admin exploitable
- [ ] sorties lisibles
- [ ] gestion d'erreurs propre

## Phase 8 - Audit et Observabilite

- [ ] Logger chaque commande executee
- [ ] Logger qui a lance la commande
- [ ] Logger sur quel agent
- [ ] Logger resultat, duree et statut
- [ ] Ajouter logs serveur
- [ ] Ajouter logs agent
- [ ] Ajouter rotation des logs
- [ ] Ajouter diagnostics :
  - etat serveur
  - etat agent
  - derniers heartbeats

Livrables :

- [ ] audit minimal exploitable
- [ ] logs de diagnostic

## Phase 9 - Stockage et Persistance

- [ ] Evaluer le remplacement du JSON simple par un stockage plus robuste
- [ ] Definir les donnees persistantes :
  - agents
  - statuts
  - audit
  - version
  - configuration
- [ ] Definir migration de schema
- [ ] Definir backup/restauration
- [ ] Definir retention/nettoyage

Livrables :

- [ ] persistance fiable
- [ ] strategie de migration

## Phase 10 - Robustesse et Resilience

- [ ] Gerer les coupures reseau
- [ ] Gerer les agents hors ligne
- [ ] Gerer les timeouts serveur/agent/cli
- [ ] Gerer les reconnexions
- [ ] Gerer les doublons d'identite agent
- [ ] Eviter les leaks de connexions/goroutines
- [ ] Tester arrets brutaux et redemarrages

Livrables :

- [ ] comportement degrade propre
- [ ] recuperation fiable

## Phase 11 - Deploiement

- [ ] Definir package agent Windows
- [ ] Definir package serveur Windows
- [ ] Definir image Docker serveur
- [ ] Definir variables d'environnement et fichiers de config
- [ ] Definir ports officiels
- [ ] Definir procedure d'installation poste employe
- [ ] Definir procedure d'installation serveur
- [ ] Definir procedure de mise a jour
- [ ] Definir rollback

Livrables :

- [ ] mode deploiement officiel
- [ ] procedure standardisee

## Phase 12 - Tests

- [ ] Ajouter tests unitaires `core`
- [ ] Ajouter tests unitaires `server`
- [ ] Ajouter tests unitaires `agent`
- [ ] Ajouter tests unitaires plugins
- [ ] Ajouter tests d'integration `cli -> server -> agent`
- [ ] Ajouter tests de reconnexion
- [ ] Ajouter tests de redemarrage service
- [ ] Ajouter tests de non-regression

Livrables :

- [ ] suite de tests de base
- [ ] tests d'integration minimum

## Phase 13 - Documentation

- [ ] Reecrire le `README`
- [ ] Ajouter diagramme d'architecture
- [ ] Ajouter guide d'installation agent
- [ ] Ajouter guide d'installation serveur
- [ ] Ajouter guide admin CLI
- [ ] Ajouter guide securite
- [ ] Ajouter guide de depannage
- [ ] Ajouter roadmap maintenue

Livrables :

- [ ] documentation utilisable
- [ ] documentation d'exploitation

## Definition of Done V1

La V1 est consideree prete quand les points suivants sont vrais :

- [ ] un seul serveur central peut etre demarre proprement
- [ ] un agent peut etre installe une seule fois sur un PC employe
- [ ] l'agent devient un service Windows
- [ ] l'agent redemarre automatiquement apres reboot
- [x] le CLI peut lister les agents
- [x] le CLI peut demander des infos agent
- [x] le CLI peut executer des commandes a distance
- [ ] les commandes sont journalisees
- [ ] le serveur et les agents ont des logs utilisables
- [ ] la configuration est propre et persistante
- [x] le systeme supporte les deconnexions/reconnexions de base

## Notes d'Implementation

- Commencer simple mais dans la bonne direction
- Ne pas rajouter de fonctions "demo" qui contredisent l'architecture cible
- Toujours privilegier :
  - serveur central unique
  - agent installe comme service
  - CLI purement admin
  - securite et audit

## Premiere Sequence Recommandee

1. Finaliser l'architecture cible
2. Refaire le serveur central unique
3. Refaire l'agent pour en faire un service Windows
4. Ajouter configuration propre
5. Stabiliser le flux `exec`
6. Ajouter logs et heartbeat fiables
7. Ajouter securite minimale
8. Ajouter tests d'integration
