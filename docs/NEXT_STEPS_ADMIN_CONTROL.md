# Next Steps - Admin Control

Ce document fixe les commandes d'administration a ajouter apres la validation de l'installation agent.

## Objectif

Donner a l'admin des moyens simples de :

- voir les agents
- verifier leur etat
- stopper un agent
- relancer un agent
- geler temporairement un agent

## Ce qui existe deja

- `list`
- `info`
- `exec`
- `ping`
- `agent freeze <agent-id>`
- `agent unfreeze <agent-id>`
- `agent stop <agent-id>`

## Capacites d'administration a ajouter

### Visibilite

- `trish list`
- `trish info <agent-id>`
- `trish logs <agent-id>`
- `trish ping <agent-id>`

### Controle agent

- `trish agent stop <agent-id>`
- `trish agent restart <agent-id>`
- `trish agent disable <agent-id>`
- `trish agent enable <agent-id>`
- `trish agent freeze <agent-id>`
- `trish agent unfreeze <agent-id>`

## Semantique recommandee

### `stop`

- arrete le runtime agent sur la machine cible
- le service peut rester installe

### `disable`

- desactive le service Windows
- empeche le redemarrage automatique apres reboot

### `freeze`

- garde le service vivant
- refuse temporairement les commandes distantes
- utile pour maintenance, investigation ou quarantaine douce

### `unfreeze`

- reactive l'acceptation des commandes

## Recommendation produit

Pour V1, il faut au minimum :

- voir les agents
- connaitre leur etat
- stopper/redemarrer proprement un agent

Le mode `freeze` est utile, mais peut venir juste apres.

Etat actuel :

- `ping` valide
- `freeze / unfreeze` valides
- `stop` valide
- `restart` code mais pas encore valide en condition service Windows reelle
- `start / disable / enable` pas encore implementes

## Impact technique

Pour supporter ca proprement, il faudra :

- enrichir les statuts agent
- ajouter des commandes d'administration serveur -> agent
- ajouter des transitions d'etat cote agent
- journaliser ces actions
