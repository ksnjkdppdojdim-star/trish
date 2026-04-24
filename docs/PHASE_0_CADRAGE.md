# Phase 0 - Cadrage Produit

Ce document fixe la base produit et architecture de `Trish` avant les refontes techniques.

## Objectif

`Trish` doit permettre a une equipe IT d'executer des commandes distantes encadrees sur des PC de travail Windows, depuis un poste admin, avec une architecture centralisee, un serveur unique et des agents persistants installes sur les postes employes.

## Roles Officiels

### `trish-agent`

Role :

- composant installe sur chaque PC employe
- tourne comme service Windows
- demarre automatiquement apres reboot
- maintient la communication avec le serveur central
- execute les commandes autorisees
- renvoie les resultats et l'etat de la machine

Contraintes :

- installation une seule fois
- pas d'usage quotidien manuel par l'employe
- pas d'interface console permanente en production

### `trish-server`

Role :

- serveur central unique
- source d'autorite sur les agents connectes
- orchestre les commandes et les sessions
- conserve le registre des agents
- gere l'audit et l'etat global

Contraintes :

- une seule instance logique par environnement
- peut etre heberge en Docker ou en executable, mais pas les deux en meme temps

### `trish-cli`

Role :

- client d'administration
- permet a un ou plusieurs admins d'envoyer des commandes
- n'execute jamais directement sur les agents
- passe toujours par `trish-server`

Contraintes :

- plusieurs instances admin autorisees
- aucune responsabilite d'orchestration centrale

## Flux Officiel des Commandes

Flux V1 :

1. l'admin lance une commande dans `trish-cli`
2. `trish-cli` envoie la requete a `trish-server`
3. `trish-server` valide la cible et route la commande
4. `trish-agent` execute la commande autorisee
5. `trish-agent` renvoie le resultat a `trish-server`
6. `trish-server` renvoie le resultat a `trish-cli`
7. `trish-cli` affiche le resultat

Principe fondamental :

- le CLI ne parle jamais directement aux postes employes en cible produit
- tout passe par le serveur central

## Architecture Cible V1

Architecture retenue :

- un serveur central unique `trish-server`
- un agent `trish-agent` sur chaque PC employe
- un ou plusieurs `trish-cli` cote admin
- liaison reseau centralisee via le serveur

Direction technique retenue pour la suite :

- cible architecture : connexion persistante sortante `agent -> server`

Pourquoi ce choix :

- plus simple a faire accepter en reseau d'entreprise
- evite d'exposer un port entrant sur chaque PC employe
- facilite firewall, NAT, supervision et reconnexion
- permet au serveur d'orchestrer sans ouvrir l'acces direct depuis les CLI

## Perimetre V1

### Inclus en V1

- serveur central unique
- enregistrement des agents
- etat agent de base : online / offline / last seen
- CLI admin de base
- commandes :
  - `list`
  - `info <agent-id>`
  - `exec <agent-id> <command>`
- plugins de base :
  - `ipconfig`
  - `dir`
  - `cd`
  - `pwd` a ajouter rapidement
- agent installe comme service Windows
- auto-start apres reboot
- logs de base cote serveur et agent
- heartbeat de base
- configuration simple et persistante

### Hors V1

- interface graphique
- execution shell arbitraire libre par defaut
- multi-tenant
- RBAC complet
- chiffrement et PKI avances
- orchestration multi-serveurs
- groupes de machines et scheduling avances

## Contraintes Entreprise

Contraintes non negociables :

- un seul serveur central actif par environnement
- l'agent doit survivre au reboot via service Windows
- l'agent ne doit pas rester en console visible apres installation
- chaque commande doit etre tracable
- les erreurs doivent etre comprehensibles
- le systeme doit fonctionner meme si certains agents sont hors ligne
- la solution doit etre deploable simplement sur des PC de travail

Contraintes operationnelles :

- installation simple sur poste employe
- maintenance possible sans reinstaller tout le parc
- compatibilite Windows prioritaire
- comportement deterministe en cas de double lancement serveur

## Cas d'Usage Prioritaires

### UC1 - Installation d'un agent

1. un technicien copie `trish-agent.exe` sur le poste employe
2. il le lance une fois
3. l'installateur configure et cree le service Windows
4. la fenetre se ferme
5. l'agent tourne en arriere-plan
6. apres reboot, le service redemarre automatiquement

### UC2 - Lister les postes disponibles

1. l'admin lance `trish-cli list`
2. le serveur renvoie les agents connus
3. le CLI affiche les postes online/offline

### UC3 - Obtenir les informations d'un poste

1. l'admin lance `trish-cli info <agent-id>`
2. le serveur renvoie l'etat et les capacites de l'agent
3. le CLI affiche les metadonnees utiles

### UC4 - Executer une commande distante

1. l'admin lance `trish-cli exec <agent-id> dir C:\`
2. le serveur route la commande vers l'agent
3. l'agent execute le plugin `dir`
4. le resultat remonte et s'affiche dans le CLI

### UC5 - Agent temporairement hors ligne

1. l'agent perd la connexion
2. le serveur le marque offline apres timeout
3. l'agent se reconnecte plus tard
4. le serveur met son etat a jour

## Conventions de Deploiement

### Mode serveur officiel

Deux modes admis, mais un seul a la fois par environnement :

- mode A : `trish-server` en Docker
- mode B : `trish-server.exe` sur un hote Windows dedie

Regle :

- si Docker est le mode choisi, l'executable serveur local ne doit pas etre lance en parallele
- si l'executable est le mode choisi, aucun conteneur serveur concurrent ne doit tourner

### Mode agent officiel

- `trish-agent.exe` sert d'installeur/bootstrap
- le runtime normal de l'agent doit tourner comme service Windows
- le double-clic ne doit pas laisser une console ouverte indefiniment en production

### Mode CLI officiel

- `trish-cli.exe` peut etre execute sur plusieurs postes admin
- le CLI n'est jamais une instance mere
- il ne conserve pas l'etat central du parc

## Convention de Demarrage

### `trish start`

Commande cible a implementer dans les phases suivantes :

- demarre le serveur central
- si une ancienne instance existe, elle est remplacee ou redemarree proprement
- a la fin, une seule instance logique de `trish-server` existe

### Agent

Commande cible a implementer :

- `trish-agent.exe` sans argument : installation guidee
- `trish-agent.exe install` : installation explicite
- `trish-agent.exe run-service` : mode interne du service
- `trish-agent.exe uninstall` : suppression propre
- `trish-agent.exe repair` : reparation / mise a jour

## Identite et Configuration

### Identite agent V1

Identite minimale retenue :

- `machine_id`
- `hostname`
- `ip`
- `version`

Le `hostname` seul ne suffit pas a long terme.

### Configuration minimale V1

Configuration agent :

- adresse serveur
- port serveur
- identite agent
- version agent
- chemin des logs

Configuration serveur :

- port d'ecoute
- stockage registre
- stockage audit
- configuration TLS future

## Risques Connus

- l'implementation actuelle est encore en mode prototype pour l'agent
- la securite n'est pas encore au niveau entreprise
- le transport actuel doit encore converger vers la connexion persistante `agent -> server`
- l'unicite stricte du serveur n'est pas encore implementee

## Decisions Figees en Fin de Phase 0

- `trish-agent` sera un service Windows
- `trish-server` sera l'unique autorite centrale
- `trish-cli` sera un client admin uniquement
- l'architecture cible passe par un serveur central unique
- le mode reseau cible est la connexion persistante `agent -> server`
- `trish start` devra garantir l'unicite logique du serveur
- le double-clic sur `trish-agent.exe` devra lancer une installation et non un runtime console permanent

## Livrables de Phase 0

- note d'architecture cible : ce document
- perimetre V1 : fixe dans ce document
- conventions de deploiement : fixees dans ce document
