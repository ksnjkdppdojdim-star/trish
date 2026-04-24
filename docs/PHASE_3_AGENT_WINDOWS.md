# Phase 3 - Agent Windows Professionnel

Cette phase pose la structure de l'agent en mode installation + service Windows, sans encore clore toute la validation terrain admin.

## Objectif

Faire evoluer `trish-agent.exe` d'un simple runtime console vers un composant deploiement-friendly avec :

- mode installation
- mode reparation
- mode desinstallation
- mode service interne
- mode foreground de debug

## Ce qui a ete implemente

## 1. Nouveau cycle de vie de `trish-agent.exe`

L'entree agent supporte maintenant les modes suivants :

- `install`
- `repair`
- `uninstall`
- `run-service`
- `run-foreground`

Comportement actuel :

- sans argument, `trish-agent.exe` lance `install`
- `run-service` est reserve au service Windows
- `run-foreground` sert au debug local

## 2. Configuration persistante de l'agent

Une configuration agent persistante a ete introduite avec :

- `server_addr`
- `server_port`
- `listen_port`
- `install_dir`
- `log_dir`
- `version`

Chemins cibles par defaut :

- install : `C:\ProgramData\Trish\agent`
- config : `C:\ProgramData\Trish\agent\agent-config.json`
- logs : `C:\ProgramData\Trish\agent\logs`

## 3. Runtime agent factorise

Le runtime de connexion persistante au serveur a ete conserve et factorise pour etre reutilisable :

- en mode service
- en mode foreground

Les plugins par defaut sont enregistres dans une couche dediee.

## 4. Installation Windows

Le flux d'installation code actuellement :

1. verification admin
2. creation du dossier d'installation
3. copie du binaire vers le chemin cible
4. ecriture de la configuration
5. creation ou reconfiguration du service `TrishAgent`
6. configuration du restart automatique
7. demarrage du service

## 5. Service Windows

Un mode service Windows natif a ete ajoute via l'API de controle de services Windows.

Le service :

- charge la config
- ouvre les logs
- demarre le runtime agent
- attend le stop du SCM
- arrete proprement l'agent

## 6. Logs agent

Un logger fichier a ete ajoute cote agent.

Fichier cible :

- `C:\ProgramData\Trish\agent\logs\agent.log`

## Validation effectuee

Validation reelle confirmee sur la machine de travail :

- demande UAC affichee correctement
- installation executee jusqu'au bout
- service `TrishAgent` cree
- service demarre en `Automatic`
- configuration ecrite dans `C:\ProgramData\Trish\agent`
- logs ecrits dans `C:\ProgramData\Trish\agent\logs\agent.log`

## Limites connues de cette phase

Cette phase n'est pas encore totalement fermee fonctionnellement :

- la fermeture automatique de la fenetre apres succes n'est pas encore implementee
- la relance admin automatique n'est pas encore implementee
- le mode `install` ne fait pas encore de test explicite de sante du service juste apres demarrage

## Ce qui est pret pour la suite

- structure `install / repair / uninstall / run-service / run-foreground`
- config persistante
- runtime reutilisable
- service Windows code
- logs agent

## Ce qui reste pour clore vraiment la phase 3

- test de redemarrage machine
- fermeture automatique de la fenetre apres succes
- eventuel auto-elevate UAC
- verification propre de l'etat du service apres installation

## Fichiers touches

- `cmd/agent/main.go`
- `agent/config.go`
- `agent/runtime.go`
- `agent/install_windows.go`
- `agent/service_windows.go`
- `agent/service_nonwindows.go`
