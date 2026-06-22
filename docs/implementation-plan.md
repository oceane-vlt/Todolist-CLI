# Plan d'implémentation — TodoList-CLI vers stockage distant

> **Document de planification.** Pas d'implémentation de production : ce document découpe le passage de l'architecture actuelle à l'architecture cible en phases incrémentales, livrables et testables. Les extraits (signatures, commandes, snippets proto/SQL) sont **illustratifs**.
>
> - **Statut** : Proposé.
> - **Date** : 2026-06-20.
> - **S'appuie sur** : [`docs/remote-storage-analysis.md`](./remote-storage-analysis.md) (analyse, recommandation §7) et [`docs/target-architecture.md`](./target-architecture.md) (décisions d'architecture).
> - **Cible** : PostgreSQL managé (Neon) + auth JWT (Supabase Auth) + gRPC/TLS + hébergement Fly.io + gRPC-Gateway (futur web).

## 1. Principe directeur

- **Incrémental, jamais big-bang.** Chaque phase est **livrable et testable** indépendamment et **ne casse pas** le projet entre deux phases : à tout moment, `go test ./...` passe et le CLI reste utilisable.
- **Feature flag de stockage** : une variable `TODO_STORAGE=json|postgres` permet de basculer entre l'ancien stockage JSON et le nouveau Postgres pendant toute la transition. Tant que l'implémentation distante n'est pas validée, on reste sur `json` par défaut. Rollback = repasser le flag sur `json`.
- **Dérisquage local d'abord** : l'auth est testée avec un JWT signé localement **avant** de brancher Supabase ; le TLS est testé avec un certificat auto-signé **avant** de déployer sur Fly.io.
- **Migration non destructive** : `~/.config/todolist/data.json` n'est jamais supprimé pendant la transition (renommé en `.bak` au plus).
- **Cohérence documentaire** : le plan respecte la recommandation de l'analyse (§7) et les décisions de l'ADR (choix arrêtés §1, schéma §2, identité via metadata §4, flux auth §5, déploiement §6).

## 2. Vue d'ensemble des phases

| # | Phase | Couches principales | Effort | Dépend de |
| --- | --- | --- | --- | --- |
| 0 | Abstraction `Store` (interface) | `libs/storage`, `cmd/server` | **S** | — |
| 1 | Backend Postgres derrière l'interface (mono-user, sans auth/TLS) | `libs/storage`, `go.mod`, migrations SQL, `Makefile` | **M** | 0 |
| 2 | Multi-utilisateur & isolation `user_id` | `libs/storage`, `cmd/server` | **M** | 1 |
| 3 | Auth JWT + intercepteur serveur + sous-commandes CLI | `cmd/server`, `cmd/todo`, `go.mod`, `proto` (optionnel) | **M/L** | 2 |
| 4 | TLS sur le transport | `cmd/server`, `cmd/todo` | **M** | 3 |
| 5 | Migration `data.json` → Postgres + rétrocompat | `cmd/todo`, `libs/storage` | **M** | 1, 2, 3 |
| 6 | Hébergement Fly.io + secrets + CI | déploiement, `Makefile`, `.github/workflows`, docs | **M/L** | 1–5 |
| 7 | gRPC-Gateway + front web (**HORS v1**) | `proto`, déploiement | **L** | 6 |

**Ordre recommandé** : 0 → 1 → 2 → 3 → 4 → 5 → 6, puis 7 plus tard (hors première version). Les phases 4 (TLS) et 5 (migration) peuvent se préparer en parallèle de la phase 3 une fois la 2 terminée, mais l'ordre linéaire ci-dessus minimise les risques.

## 3. Phases détaillées

### Phase 0 — Abstraction `Store` (interface)

- **Objectif** : introduire une interface `Store` dans `libs/storage` qui capture le comportement de persistance actuel, et faire passer `cmd/server` par cette interface. **Aucun changement fonctionnel** : l'implémentation JSON existante devient `JSONStore`, qui satisfait l'interface. C'est la **couture** qui dérisque tout le reste.
- **Fichiers / couches touchés** : `libs/storage/` (nouveau `store.go` avec l'interface + adaptation des fichiers par-opération `AddData.go`, `createList.go`, `deleteData.go`, `deleteItemsData.go`, `readData.go`, `showData.go`, `UpdateItemData.go`, `common.go`), `cmd/server/main.go` (dépend de l'interface, pas de l'impl concrète).
- **Dépendances** : aucune.
- **Validation / tests** : `go test ./...` reste vert (les tests existants `AddData_test.go`, `deleteItemsData_test.go`, `readData_test.go` continuent de passer) ; comportement CLI **identique** à aujourd'hui (smoke test manuel : create/list/show/delete).
- **Risques & dérisquage** : risque faible. Risque = casser la signature des opérations existantes → garder l'impl JSON inchangée derrière l'interface, refactoring purement mécanique.
- **Effort** : **S**.

> Interface illustrative :
> ```go
> type Store interface {
>     GetLists(ctx context.Context) ([]ListSummary, error)
>     CreateList(ctx context.Context, title string, items []Item) error
>     // ... une méthode par RPC métier existant
> }
> ```

### Phase 1 — Backend Postgres derrière l'interface (mono-user, sans auth/TLS)

- **Objectif** : ajouter une implémentation `PgStore` (via `pgx`) qui satisfait l'interface `Store`, avec le schéma SQL de l'ADR §2.2 — mais avec un `user_id` **fixe/placeholder** pour cette phase (le multi-user vient en phase 2). Brancher le **feature flag** `TODO_STORAGE=json|postgres`.
- **Fichiers / couches touchés** : `libs/storage/pgstore.go` (nouveau), migrations SQL (`migrations/0001_init.sql`), sélection de l'impl selon `TODO_STORAGE` (dans `cmd/server/main.go` ou une factory `libs/storage`), `go.mod` (ajout `pgx`), `Makefile` (cible de migration + éventuellement `docker-compose` Postgres local pour les tests).
- **Dépendances** : Phase 0 (l'interface doit exister).
- **Validation / tests** : tests d'intégration `PgStore` sur un Postgres local (Docker) ; **tests de parité** comparant le résultat des opérations `JSONStore` vs `PgStore` sur les mêmes entrées ; `TODO_STORAGE=postgres` permet de faire tourner le CLI contre Postgres en mono-user.
- **Risques & dérisquage** : divergence schéma/modèle entre JSON et SQL → **tests de parité** systématiques ; outillage migrations à trancher (`golang-migrate` vs `sqlc` vs SQL brut) → choisir avant de coder la phase (voir §5).
- **Effort** : **M**.

### Phase 2 — Multi-utilisateur & isolation `user_id`

- **Objectif** : propager un `user_id` (lu depuis le `context.Context`) dans `PgStore` (`WHERE user_id = $1`, clé logique `(user_id, title)`) et dans les handlers de `cmd/server` (scoping). Le `user_id` est encore **injecté en dur / via variable de dev** tant que l'auth (phase 3) n'est pas branchée.
- **Fichiers / couches touchés** : `libs/storage/pgstore.go` (signatures prennent `userID`, requêtes filtrées), `cmd/server/main.go` (handlers passent le `user_id` du context au store).
- **Dépendances** : Phase 1.
- **Validation / tests** : test d'isolation — deux `user_id` distincts ne voient **pas** les listes l'un de l'autre ; `UNIQUE(user_id, title)` autorise le même titre pour deux utilisateurs différents ; les anciens tests restent verts.
- **Risques & dérisquage** : oubli d'un filtre `user_id` dans une requête (fuite de données) → revue ciblée + test d'isolation couvrant **chaque** RPC.
- **Effort** : **M**.

### Phase 3 — Auth JWT + intercepteur serveur + sous-commandes CLI

- **Objectif** : authentifier réellement l'appelant. Côté serveur : un **intercepteur gRPC** valide le JWT (signature + expiration) et injecte le `user_id` dans le `context` (qui alimente la phase 2). Côté CLI : sous-commandes `login` / `signup` / `logout`, stockage du token dans `~/.config/todolist/credentials.json` (perms `0600`), attache `authorization: Bearer <access JWT>` en metadata, refresh transparent sur `Unauthenticated`.
- **Fichiers / couches touchés** : `cmd/server/main.go` (intercepteur unaire), `cmd/todo/` (commandes `login`/`signup`/`logout`, gestion `credentials.json`, attache du token, logique de refresh), `go.mod` (ajout `golang-jwt`), `proto/todoList.proto` (**optionnel** : `AuthService` Signup/Login/Refresh **uniquement en mode auth maison** ; non nécessaire avec Supabase Auth, cf. ADR §4.2).
- **Dépendances** : Phase 2 (le `user_id` du context doit déjà scoper le storage).
- **Validation / tests** : appel **sans** token → `Unauthenticated` ; token **expiré** → `Unauthenticated` puis refresh transparent rejoue l'appel ; token valide → accès aux seules listes de l'utilisateur ; `credentials.json` créé en `0600`.
- **Risques & dérisquage** : complexité d'intégration Supabase → **dérisquer en local d'abord** avec un JWT signé par une clé de test (mode maison), valider tout le flux intercepteur/refresh, **puis** brancher la validation des JWT Supabase (JWKS/secret). Secret de signature à protéger (env, jamais dans le repo).
- **Effort** : **M/L**.

### Phase 4 — TLS sur le transport

- **Objectif** : chiffrer le transport gRPC (fin de `insecure.NewCredentials()`). Côté serveur : credentials TLS. Côté CLI : credentials TLS, **endpoint configurable** (`TODO_SERVER_ENDPOINT` / `--endpoint`, remplace le `127.0.0.1:50051` codé en dur), et ajout de **timeouts/retries** (réseau distant).
- **Fichiers / couches touchés** : `cmd/server/main.go` (`credentials.NewTLS`), `cmd/todo/main.go` (creds TLS, endpoint configurable, `context.WithTimeout`, politique de retry).
- **Dépendances** : Phase 3 (les tokens transitent désormais sur un canal qui doit être chiffré).
- **Validation / tests** : connexion **insecure refusée** par le serveur TLS ; handshake TLS réussi avec un certificat auto-signé en local ; un appel dépassant le timeout échoue proprement (pas de blocage).
- **Risques & dérisquage** : erreurs de configuration TLS → **tester avec un certificat auto-signé en local** avant tout déploiement distant (phase 6). Sur Fly.io, le TLS peut être terminé par le PaaS (cf. ADR §6.2).
- **Effort** : **M**.

### Phase 5 — Migration `data.json` → Postgres + rétrocompatibilité

- **Objectif** : permettre à un utilisateur existant de transférer son `~/.config/todolist/data.json` vers Postgres pour son compte connecté, sans perte. Commande one-shot `todo migrate` : lit le JSON local, **upsert** dans Postgres pour le `user_id` courant, **idempotente** (rejouable sans doublon grâce à `UNIQUE(user_id, title)`). Le `data.json` est **conservé** (renommé `.bak`), jamais supprimé.
- **Fichiers / couches touchés** : `cmd/todo/` (sous-commande `migrate`), `libs/storage` (lecture JSON existante réutilisée + écriture via `PgStore`).
- **Dépendances** : Phases 1 (PgStore), 2 (scoping `user_id`), 3 (le user doit être authentifié pour avoir un `user_id`).
- **Validation / tests** : migration d'un `data.json` d'exemple → listes/items présents en base sous le bon `user_id` ; **ré-exécution idempotente** (pas de doublon, pas d'erreur) ; le fichier d'origine est préservé en `.bak`.
- **Risques & dérisquage** : perte de données → ne jamais supprimer `data.json`, migration idempotente, dry-run possible. Rétrocompat assurée par le **feature flag** : tant que `TODO_STORAGE=json`, rien ne change pour l'utilisateur ; bascule sur `postgres` après migration validée.
- **Effort** : **M**.

### Phase 6 — Hébergement Fly.io + secrets + CI

- **Objectif** : déployer le serveur gRPC sur un PaaS free tier (Fly.io), accessible publiquement en TLS, et **remplacer le launchd local**. Mettre en place les secrets et une CI.
- **Fichiers / couches touchés** : `Dockerfile` (serveur), `fly.toml`, secrets via `fly secrets` (`DATABASE_URL`, `SUPABASE_JWT_SECRET`/`SUPABASE_JWKS_URL`, `TLS_*`, `PORT` — cf. ADR §6.2), documentation (`docs/daemon-setup.md` → guide de déploiement distant ; le CLI ne lance plus de serveur), `.github/workflows/` (CI **nouvelle** : build + `go test` + `golangci-lint`, absente aujourd'hui).
- **Dépendances** : Phases 1–5 (le serveur doit être complet et sécurisé avant d'être exposé publiquement).
- **Validation / tests** : déploiement sur un environnement de staging Fly.io ; **smoke test** du CLI distant (`login` puis `create`/`list` contre l'endpoint déployé) ; la CI passe sur une PR.
- **Risques & dérisquage** : secret committé par erreur → secrets exclusivement via le secret manager Fly.io, `.gitignore` vérifié, scan de secrets en CI. Mise en veille du free tier → documenter le comportement (cold start). Déployer en **staging** avant prod.
- **Effort** : **M/L**.

### Phase 7 — gRPC-Gateway + front web (HORS v1)

- **Objectif** : exposer le même service gRPC en REST/JSON via **gRPC-Gateway** (annotations `google.api.http` sur les RPC), pour un futur front web (cf. ADR §3 et analyse §6).
- **Fichiers / couches touchés** : `proto/todoList.proto` (annotations HTTP), génération Gateway (`Makefile`/`buf`), déploiement (exposer la Gateway).
- **Dépendances** : Phase 6.
- **Statut** : **explicitement hors première version** — à planifier après la v1 CLI distante stabilisée.
- **Effort** : **L**.

## 4. Gestion de la migration et rétrocompatibilité

- **Feature flag `TODO_STORAGE`** (`json` | `postgres`) actif **pendant toute la transition** : par défaut `json` jusqu'à validation complète du chemin distant, puis bascule vers `postgres`. Permet un rollback instantané.
- **`data.json` jamais détruit** : la migration (phase 5) le renomme en `.bak` au plus ; aucune suppression automatique.
- **Outil `todo migrate` idempotent** : rejouable sans créer de doublons (garanti par `UNIQUE(user_id, title)`), donc sûr en cas d'interruption.
- **Rollback** : repasser `TODO_STORAGE=json` restaure immédiatement le comportement local d'origine (les données locales sont toujours là).
- **Pas de fenêtre de casse** : entre deux phases, le projet compile, les tests passent et le CLI fonctionne (sur JSON par défaut).

## 5. Nouvelles dépendances Go, config, secrets et CI

### Dépendances Go à ajouter

| Dépendance | Rôle | Phase |
| --- | --- | --- |
| `github.com/jackc/pgx` (v5) | Driver / pool Postgres, requêtes paramétrées | 1 |
| `golang-migrate` **ou** `sqlc` (à trancher) | Migrations SQL / génération de code typé depuis le SQL | 1 |
| `github.com/golang-jwt/jwt` (v5) | Validation/parsing des JWT côté serveur | 3 |
| `supabase-go` (éventuel) | Dialogue CLI ↔ Supabase Auth si on ne tape pas l'API HTTP directement | 3 |

> **Décision à prendre en début de phase 1** : `golang-migrate` (migrations versionnées, SQL brut) vs `sqlc` (génère du Go typé depuis les requêtes SQL). Recommandation : trancher avant d'écrire `PgStore` pour ne pas mélanger les approches.

### Config / secrets (cf. ADR §6.2)

- **Serveur** : `DATABASE_URL`, `SUPABASE_JWT_SECRET` / `SUPABASE_JWKS_URL`, `JWT_SIGNING_KEY` (mode maison), `TLS_CERT_PATH` / `TLS_KEY_PATH`, `PORT` — via secret manager Fly.io, **jamais** dans le repo.
- **CLI** : `TODO_SERVER_ENDPOINT` (ou `--endpoint`), `TODO_STORAGE` (transition), `~/.config/todolist/credentials.json` (0600).

### CI (nouvelle)

- Pas de `.github/workflows` aujourd'hui. Ajouter (phase 6) un pipeline GitHub Actions : `go build`, `go test ./...`, `golangci-lint` (la cible `lint` du `Makefile` existe déjà), et idéalement un job d'intégration Postgres (service container).

## 6. Risques transverses et stratégie de dérisquage

| Risque | Phase(s) | Dérisquage |
| --- | --- | --- |
| Divergence comportement JSON ↔ Postgres | 1 | Tests de parité `JSONStore` vs `PgStore` |
| Fuite de données (filtre `user_id` oublié) | 2 | Test d'isolation par RPC + revue ciblée |
| Intégration auth complexe / fragile | 3 | JWT signé localement d'abord, **puis** Supabase ; refresh testé hors-ligne |
| Mauvaise config TLS | 4, 6 | Certificat auto-signé en local avant Fly.io |
| Perte de données à la migration | 5 | `data.json` conservé en `.bak`, migration idempotente, feature flag |
| Secret committé / exposé | 6 | Secret manager PaaS, scan de secrets en CI, `.gitignore` |
| Casse entre deux phases | toutes | Feature flag `TODO_STORAGE`, chaque phase testable, tests verts en continu |

## 7. Hors scope de la première version

- **Front web** et **gRPC-Gateway** (phase 7) — viennent après la v1 CLI distante.
- **Temps réel / notifications** (sync push entre devices) — la cohérence v1 repose sur la base, pas sur du push.
- **Partage de listes entre utilisateurs** — le modèle v1 isole strictement par `user_id`.
- **Fournisseurs OAuth supplémentaires** (GitHub/Google) si Supabase Auth couvre déjà email/mot de passe pour la v1.
- **Observabilité avancée** (tracing distribué, dashboards) — au-delà des logs serveur de base.

---

*Document de planification uniquement — aucune implémentation de production. Extraits (signatures, SQL, proto) fournis à titre illustratif. Cohérent avec `docs/remote-storage-analysis.md` et `docs/target-architecture.md`.*
