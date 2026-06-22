# Analyse : passage d'un stockage local à un stockage distant

> Document d'**analyse** (pas d'implémentation). Objectif : permettre de retrouver ses todolists depuis plusieurs machines, en gardant un backend réutilisable par un futur front web.

## 1. Contexte et architecture actuelle

État des lieux du code (vérifié sur les sources) :

| Couche | Fichier | État actuel |
| --- | --- | --- |
| CLI client | `cmd/todo/main.go` | Client gRPC, `dial 127.0.0.1:50051`, `insecure.NewCredentials()` — **pas de TLS, pas d'auth, pas de timeout/retry** |
| Serveur | `cmd/server/main.go` | Serveur gRPC, `net.Listen("tcp","127.0.0.1:50051")` — **mono-utilisateur, pas d'auth, pas de TLS, pas d'intercepteurs** ; daemon launchd (macOS) |
| Contrat | `proto/todoList.proto` | `TodoListService`, 7 RPC. Listes indexées par **Title** uniquement. **Aucune notion de compte/utilisateur** |
| Stockage | `libs/storage/common.go` | `TodoData{Lists map[string][]TodoItem}` sérialisé dans un **unique fichier JSON** `~/.config/todolist/data.json` via `os.WriteFile` (réécriture complète, `0644`). **Pas de lock, pas de transaction, pas de contrôle de concurrence** |
| Dépendances | `go.mod` | go 1.24, grpc v1.76, protobuf v1.36, cobra, promptui. **Pas de driver DB, pas de framework HTTP, pas de lib d'auth** |

Le découpage en couches est propre :

```
cmd/todo (client) -> proto (contrat) -> server (impl gRPC) -> libs/storage (persistance)
```

**Le contrat gRPC est la couture (« seam ») naturelle** à conserver. On remplace `libs/storage` (JSON local) par un accès à une base distante, derrière le même serveur gRPC. Un futur front web pourra réutiliser ce backend via **gRPC-Web** ou **gRPC-Gateway** (REST/JSON généré depuis le `.proto`).

## 2. Écarts à combler pour le distant / multi-device / multi-user

1. **Authentification + autorisation** : inexistantes aujourd'hui.
2. **Sécurité du transport (TLS)** : connexion `insecure` aujourd'hui.
3. **Isolation des données par utilisateur** : les listes sont globales (clé = Title).
4. **Sécurité des écritures concurrentes** : aucune (réécriture de fichier complet).
5. **Hébergement du serveur** publiquement accessible.

Ces points sont **transverses** : ils valent quelle que soit l'option de stockage choisie.

## 3. Options de stockage / backend distant

Critères : coût (gratuit si possible), sécurité, multi-device, multi-utilisateur, hébergement, et **réutilisabilité par un front web**.

### Option A — PostgreSQL managé sur free tier (Supabase / Neon / Fly.io Postgres)

- **Principe** : le serveur gRPC actuel reste le seul à parler à la base ; on remplace `libs/storage` par un accès SQL (`pgx`/`database/sql`). On héberge le serveur gRPC quelque part (Fly.io, Railway, Render free tier).
- **Coût** : Neon et Supabase ont des free tiers durables ; Fly.io / Railway / Render aussi (avec limites/mise en veille).
- **Multi-device / multi-user** : excellent. Schéma `users` + `lists` + `items` avec `user_id` ; transactions et contraintes SQL résolvent la concurrence (écart #4) nativement.
- **Réutilisation web** : le front web tape le **même serveur gRPC** (via gRPC-Gateway/gRPC-Web), pas la base directement → backend partagé, logique métier centralisée.
- **Risques sécurité** :
  - Chaîne de connexion (secret) à protéger (variables d'env / secret manager, jamais dans le repo).
  - Surface d'attaque = le serveur gRPC exposé : **exige TLS + auth + isolation par `user_id`** (sinon n'importe qui lit toutes les listes).
  - Injection SQL si requêtes concaténées → utiliser des requêtes paramétrées.
- **Verdict** : aligné avec l'archi actuelle (garde le serveur gRPC comme unique gardien des données), évolue bien, et **colle au "SQLite planned" du README** (Postgres = même modèle relationnel, mais distant).

### Option B — Firebase / Firestore (BaaS, NoSQL)

- **Principe** : les clients (CLI **et** front web) parlent directement à Firestore via les SDK, avec règles de sécurité côté Firebase.
- **Coût** : free tier (Spark) généreux pour un usage perso.
- **Multi-device / multi-user** : natif, sync temps réel inclus.
- **Réutilisation web** : très bon (SDK web first-class).
- **Risques sécurité** :
  - **Contourne le serveur gRPC** : la sécurité repose entièrement sur les *Security Rules* Firestore (faciles à mal configurer → fuite de données). Pas de SDK admin Go côté CLI sans service account à protéger.
  - Lock-in fort sur l'écosystème Google.
  - Le modèle de données NoSQL ne correspond pas au modèle relationnel actuel ; il faudrait repenser le schéma.
- **Verdict** : rapide à démarrer mais **casse la couture gRPC** (le backend métier n'est plus central) → moins aligné avec l'objectif "même backend réutilisé".

### Option C — SQLite + Litestream (réplication vers stockage objet)

- **Principe** : on garde SQLite local côté serveur, répliqué en continu vers un stockage objet (S3/Backblaze B2).
- **Coût** : très faible.
- **Multi-device / multi-user** : **mauvais** pour de l'écriture concurrente multi-machine. Litestream = réplication/restauration (DR), **pas** une base multi-writer. Ne résout pas le multi-device en écriture.
- **Verdict** : **écarté** pour cet objectif (multi-device en écriture est le cœur du besoin).

### Option D — MongoDB Atlas (free tier M0)

- **Principe** : base document managée, le serveur gRPC y accède.
- **Coût** : free tier M0 (512 Mo).
- **Multi-device / multi-user** : bon.
- **Réutilisation web** : bon via le serveur gRPC.
- **Risques sécurité** : config réseau (IP allowlist), secret de connexion ; historiquement, des instances mal configurées ont fuité. Modèle document à concevoir.
- **Verdict** : viable, mais le modèle relationnel (Postgres) colle mieux aux données todolist structurées et au "SQLite planned".

### Option E — VPS auto-hébergé (Postgres + serveur gRPC sur la même VM)

- **Principe** : un petit VPS (Oracle Cloud Free Tier, etc.), full contrôle.
- **Coût** : possible gratuit (Oracle Always Free) mais variable.
- **Risques sécurité** : **toute la charge d'ops/sécurité repose sur soi** (patchs OS, firewall, fail2ban, renouvellement TLS, sauvegardes). Surface et effort élevés.
- **Verdict** : flexible mais coûteux en maintenance/sécurité pour un projet perso.

## 4. Comparaison synthétique

| Option | Coût gratuit | Multi-device (écriture) | Multi-user | Réutilise le serveur gRPC | Effort sécurité/ops | Aligné archi actuelle |
| --- | --- | --- | --- | --- | --- | --- |
| A. Postgres managé | Oui (Neon/Supabase) | Excellent | Excellent | **Oui** | Moyen | **Fort** |
| B. Firestore | Oui | Excellent | Excellent | Non (court-circuité) | Moyen (règles) | Faible |
| C. SQLite+Litestream | Oui | **Faible** | Faible | Oui | Faible | Moyen |
| D. MongoDB Atlas | Oui (M0) | Bon | Bon | Oui | Moyen | Moyen |
| E. VPS auto-hébergé | Variable | Bon | Bon | Oui | **Élevé** | Moyen |

## 5. Authentification — options et risques

Quelle que soit la base, le **serveur gRPC** doit authentifier l'appelant (sauf option B où c'est Firebase).

| Approche | Description | Avantages | Risques / limites |
| --- | --- | --- | --- |
| **Clé API par utilisateur** | Token opaque envoyé en metadata gRPC | Simple à implémenter | Révocation/rotation à gérer ; si fuite, accès total ; pas de standard de scope |
| **JWT signé (court) + refresh** | Le serveur émet un JWT après login | Standard, sans état côté serveur, portable web | Gestion de l'expiration/refresh ; secret de signature à protéger ; révocation = besoin d'une blacklist |
| **OAuth/OIDC (Google, GitHub)** | Délégation d'identité à un fournisseur | Pas de gestion de mots de passe ; bon pour le futur front web | Plus complexe à câbler ; dépendance à un IdP |
| **Auth managée (Supabase Auth / Firebase Auth)** | Le BaaS fournit login + tokens | Réduit le code d'auth ; émet des JWT vérifiables côté serveur | Lock-in ; le serveur doit valider les JWT du fournisseur |

Points communs à tous :
- **TLS obligatoire** dès qu'on quitte le localhost (écart #2) — sinon credentials/tokens en clair.
- Transmettre le token via **metadata gRPC** + un **intercepteur** côté serveur qui authentifie et injecte le `user_id` dans le `context`.
- **Isolation par `user_id`** appliquée dans chaque RPC (écart #3) : ne jamais faire confiance au client pour le périmètre des données.

## 6. Impacts sur le contrat (`proto`) et le futur web

- Le `.proto` actuel n'a **aucune notion d'utilisateur** ; l'identité passera par les **metadata/token** (pas par un champ `user_id` dans les messages — le client ne doit pas pouvoir l'usurper).
- Indexer les listes par `(user_id, title)` côté serveur/base au lieu de `title` seul.
- Pour le **futur front web** : exposer le même service via **gRPC-Gateway** (REST/JSON) ou **gRPC-Web**, généré depuis le `.proto` → un seul backend, deux clients (CLI + web).

## 7. Recommandation argumentée

**Recommandation : Option A — PostgreSQL managé sur free tier (Neon ou Supabase), serveur gRPC conservé comme unique gardien des données, auth par JWT (idéalement via Supabase Auth pour limiter le code), TLS de bout en bout, isolation par `user_id`.**

Pourquoi :

1. **Respecte l'architecture existante** : on garde la couture gRPC propre (`cmd/todo` → `proto` → `server` → persistance). Seul `libs/storage` change (JSON → SQL). Le reste du code bouge peu.
2. **Backend réellement partagé** : contrairement à Firestore (option B), le front web futur tapera le **même serveur gRPC** (via gRPC-Gateway), donc la logique métier et la sécurité restent centralisées en un seul endroit.
3. **Multi-device / multi-user résolu nativement** : transactions et contraintes SQL règlent la concurrence (écart #4), `user_id` règle l'isolation (écart #3).
4. **Gratuit et pérenne** : Neon/Supabase offrent des free tiers durables suffisants pour un usage perso.
5. **Continuité avec la feuille de route** : le README annonce déjà "SQLite planned" ; passer à Postgres distant est l'évolution naturelle du même modèle relationnel.

Choix d'auth recommandé : **Supabase Auth (qui émet des JWT)** si on prend Supabase comme base, sinon **JWT maison + OAuth GitHub/Google** pour préparer le front web sans gérer de mots de passe.

À éviter pour cet objectif : SQLite+Litestream (option C, pas multi-writer) et Firestore (option B, court-circuite le backend gRPC visé).

## 8. Prérequis sécurité (à traiter quelle que soit l'option)

- [ ] Activer **TLS** sur le transport gRPC (remplacer `insecure.NewCredentials()`).
- [ ] Ajouter un **intercepteur d'authentification** côté serveur (validation token → `user_id` dans le `context`).
- [ ] **Isoler les données par `user_id`** dans chaque RPC et chaque requête SQL (requêtes paramétrées).
- [ ] Gérer les **secrets** (chaîne de connexion DB, secret JWT) via variables d'env / secret manager, hors du repo.
- [ ] Ajouter **timeouts/retries** côté client gRPC (absents aujourd'hui) pour un réseau distant.
- [ ] Mettre en place **sauvegardes** et un plan de restauration de la base.

---

*Analyse uniquement — aucune implémentation à ce stade. Étape suivante possible : un plan d'implémentation détaillé (migration `libs/storage` vers Postgres, schéma SQL, intercepteur auth, TLS, gRPC-Gateway).*
