# Architecture cible — TodoList-CLI en stockage distant

> **Document de décision d'architecture (ADR).** Pas d'implémentation de production : ce document arrête les choix techniques et décrit la cible. Les exemples de DDL SQL et de signatures sont **illustratifs**.
>
> - **Statut** : Décidé (s'appuie sur [`docs/remote-storage-analysis.md`](./remote-storage-analysis.md), recommandation §7).
> - **Date** : 2026-06-20.
> - **Étape suivante** (hors périmètre) : plan d'implémentation détaillé puis code.

## 1. Choix de solutions arrêtés

Reprise directe de la recommandation de l'analyse (cf. `remote-storage-analysis.md` §7), avec justification courte.

| Domaine | Choix arrêté | Justification (renvoi analyse) |
| --- | --- | --- |
| **Backend de stockage** | **PostgreSQL managé sur free tier — Neon** (Supabase = repli) | Analyse §3 Option A & §4 : multi-device/multi-user natif via transactions + contraintes SQL, gratuit/pérenne, modèle relationnel aligné sur le "SQLite planned" du README. |
| **Auth** | **JWT** validés côté serveur, émis par **Supabase Auth** (repli : JWT maison + OAuth GitHub/Google) | Analyse §5 & §7 : standard, sans état serveur, portable vers le web ; Supabase Auth réduit le code d'auth. |
| **Transport / TLS** | **gRPC sur TLS** (fin de `insecure.NewCredentials()`) | Analyse §2 écart #2 & §8 : obligatoire dès qu'on quitte localhost, sinon tokens en clair. |
| **Hébergement serveur** | **PaaS free tier — Fly.io** (replis : Railway, Render) | Analyse §3 Option A : héberge le serveur gRPC ; évite la charge ops/sécurité d'un VPS (Option E rejetée). |
| **Futur front web** | **gRPC-Gateway** (REST/JSON généré depuis le `.proto`) ; gRPC-Web en complément possible | Analyse §6 & §7 : un seul backend gRPC, deux clients (CLI + web), logique métier/sécurité centralisée. |
| **Isolation des données** | **Par `user_id`**, appliquée dans chaque RPC et chaque requête SQL | Analyse §2 écart #3 & §8 : ne jamais faire confiance au client pour le périmètre des données. |

**Principe directeur** : le serveur gRPC reste l'**unique gardien des données** (seul à parler à la base). On change la couche persistance, pas la couture gRPC. C'est ce qui distingue l'option retenue (A) de Firestore (B, rejetée car court-circuite le backend).

## 2. Schéma de données cible

### 2.1 Du modèle JSON actuel au modèle relationnel

Modèle actuel (`libs/storage/common.go`) : un fichier JSON unique, mono-utilisateur, listes indexées par titre.

```go
// Actuel — un seul fichier, pas de notion d'utilisateur
type TodoData struct {
    Lists map[string][]TodoItem // clé = title
}
```

| Aspect | Modèle JSON actuel | Modèle SQL cible |
| --- | --- | --- |
| Portée | Mono-utilisateur (fichier local) | Multi-utilisateur (`user_id` partout) |
| Clé d'une liste | `title` (global) | `(user_id, title)` unique |
| Items | tableau dans `map[title]` | table `items` (FK vers `lists`), ordre explicite |
| Concurrence | réécriture complète du fichier, pas de lock | transactions + contraintes ACID |
| Suppression cascade | manuelle (réécriture map) | `ON DELETE CASCADE` |
| `priority` | chaîne libre | enum / `CHECK` contraint |

### 2.2 Tables, clés et contraintes

Trois tables : `users` (peut être déléguée à Supabase Auth), `lists`, `items`.

- **`users`** — identité. PK `id`. Si Supabase Auth est utilisé, cette table = `auth.users` géré par Supabase ; on référence simplement son `id` (UUID).
- **`lists`** — une todolist appartenant à un utilisateur. **Contrainte clé** : `UNIQUE(user_id, title)` (remplace l'indexation par titre seul). FK `user_id → users(id) ON DELETE CASCADE`.
- **`items`** — les éléments d'une liste. FK `list_id → lists(id) ON DELETE CASCADE`. `position` pour préserver l'ordre (le JSON s'appuyait sur l'ordre du tableau et sur `item_index`).

DDL **illustratif** (cible Postgres) :

```sql
-- users : géré par Supabase Auth (auth.users) si Supabase ; sinon table maison.
CREATE TABLE users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email       TEXT UNIQUE NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE lists (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, title)            -- isolation + unicité par utilisateur
);

CREATE TABLE items (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    list_id     UUID NOT NULL REFERENCES lists(id) ON DELETE CASCADE,
    position    INT  NOT NULL,         -- ordre stable (remplace item_index)
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    completed   BOOLEAN NOT NULL DEFAULT false,
    due_date    DATE,
    priority    TEXT NOT NULL DEFAULT 'none'
                CHECK (priority IN ('none','low','medium','high')),
    UNIQUE (list_id, position)
);

CREATE INDEX idx_lists_user  ON lists(user_id);
CREATE INDEX idx_items_list  ON items(list_id);
```

> Mapping des champs : `Item{title, description, completed, dueDate, priority}` du `.proto` → colonnes `items`. `dueDate` (string libre côté proto) → `DATE` (ou `TEXT` si on veut préserver le format brut au début). `priority` (string libre) → contrainte `CHECK`.

## 3. Architecture cible end-to-end

Le CLI ne parle jamais à la base ; il passe toujours par le serveur gRPC distant, sur TLS, avec un JWT en metadata. Le futur front web emprunte le **même** serveur via gRPC-Gateway (REST/JSON) ou gRPC-Web.

```
                         AUJOURD'HUI (local, mono-user)
   ┌──────────────┐   gRPC insecure    ┌───────────────┐   os.WriteFile   ┌──────────────────┐
   │  cmd/todo    │ ─────────────────▶ │  cmd/server   │ ───────────────▶ │ ~/.config/.../   │
   │  (CLI)       │   127.0.0.1:50051  │  (gRPC local) │   (JSON 0644)    │   data.json      │
   └──────────────┘                    └───────────────┘                  └──────────────────┘
                                   (launchd daemon macOS)


                          CIBLE (distant, multi-user, multi-device)

   ┌──────────────┐                                          ┌───────────────────────────┐
   │  cmd/todo    │                                          │  Supabase Auth (IdP)      │
   │  (CLI)       │ ── signup/login (HTTPS) ───────────────▶ │  émet access + refresh JWT │
   │              │ ◀── access JWT + refresh JWT ──────────  └───────────────────────────┘
   │  token store │
   │  0600        │
   └──────┬───────┘
          │  gRPC over TLS
          │  metadata: authorization: Bearer <access JWT>
          ▼
   ┌─────────────────────────────────────────┐         ┌──────────────────────────────┐
   │      cmd/server  (gRPC, hébergé PaaS)     │  pgx /  │   PostgreSQL managé (Neon)   │
   │  ┌─────────────────────────────────────┐ │  SQL    │   tables users/lists/items   │
   │  │ intercepteur TLS + auth :           │ │ ──────▶ │   isolation par user_id      │
   │  │  valide JWT → user_id dans context  │ │ ◀────── │   transactions / contraintes │
   │  └─────────────────────────────────────┘ │         └──────────────────────────────┘
   │  RPC métier : scoping par user_id         │
   └──────────────▲────────────────────────────┘
                  │ (futur)
   ┌──────────────┴───────────────┐
   │  gRPC-Gateway (REST/JSON)     │ ◀── Front web futur (HTTPS / gRPC-Web)
   │  généré depuis le .proto      │
   └──────────────────────────────┘
```

Flux d'un appel métier (ex. `GetTodoLists`) :

1. Le CLI lit l'access JWT dans son store local et l'attache en metadata gRPC (`authorization: Bearer …`).
2. Connexion **TLS** au serveur gRPC distant.
3. L'**intercepteur** serveur valide le JWT (signature + expiration), extrait le `user_id`, l'injecte dans le `context.Context`.
4. Le handler RPC lit le `user_id` depuis le context et exécute une requête SQL **paramétrée** filtrée `WHERE user_id = $1`.
5. Réponse renvoyée au CLI (ou au front web via la Gateway).

## 4. Impact sur le contrat `proto` et sur le code

### 4.1 Où passe l'identité : metadata, PAS champ message

- **Décision** : l'identité de l'utilisateur transite par les **metadata gRPC** (`authorization: Bearer <JWT>`), **jamais** par un champ `user_id` dans les messages — sinon un client malveillant pourrait usurper un autre utilisateur (analyse §5 et §6).
- Le `user_id` est dérivé du JWT **côté serveur** par l'intercepteur, puis lu depuis le `context`. Le client ne le fournit ni ne le voit.

### 4.2 Contrat `proto` — ce qui change et ce qui ne change pas

- Les **7 RPC métier existants restent inchangés** dans leur signature (`CreateTodoList`, `GetTodoLists`, `ShowTodoListItems`, `DeleteTodoList`, `DeleteTodoListItems`, `UpdateTodoList`, `UpdateTodoListItem`). Ils continuent à indexer par `title` côté client ; le serveur applique la clé logique **`(user_id, title)`**.
- **Aucun champ `user_id` ajouté** aux messages (cf. 4.1).
- **Option (futur)** : un `AuthService` distinct si on n'utilise PAS l'auth managée :

  ```proto
  // Illustratif — uniquement si auth maison (sinon Supabase Auth s'en charge)
  service AuthService {
      rpc Signup  (SignupRequest)  returns (TokenResponse);
      rpc Login   (LoginRequest)   returns (TokenResponse);
      rpc Refresh (RefreshRequest) returns (TokenResponse);
  }
  message TokenResponse {
      string access_token  = 1;
      string refresh_token = 2;
      int64  expires_in    = 3;
  }
  ```

  Avec **Supabase Auth**, ce service n'est pas nécessaire : le CLI dialogue directement avec l'endpoint d'auth Supabase (HTTPS) et le serveur gRPC se contente de **valider** les JWT émis par Supabase.

- **Futur web** : annotations `google.api.http` sur les RPC pour générer la REST via **gRPC-Gateway**.

### 4.3 Couches de code impactées

| Couche | Aujourd'hui | Cible |
| --- | --- | --- |
| `libs/storage` | JSON `os.WriteFile` (réécriture complète) | **Accès Postgres via `pgx`** : requêtes paramétrées scoping `user_id`, transactions. Signatures prennent un `user_id` (issu du context). C'est le **gros du changement**. |
| `cmd/server` | gRPC local, pas d'intercepteur, pas de TLS | Ajout **TLS** (creds), **intercepteur auth** (JWT → `user_id` dans context), lecture config via env, **scoping `user_id`** dans chaque handler. |
| `cmd/todo` (CLI) | dial `insecure` 127.0.0.1, pas de token, pas de timeout | **TLS creds**, endpoint **configurable** (env/flag/config), **attache le JWT** en metadata, **timeouts/retries** (réseau distant), nouvelles sous-commandes **`login` / `signup` / `logout`**, **stockage/refresh du token**. |
| `proto` | 7 RPC, aucune notion d'utilisateur | **7 RPC inchangés** ; (optionnel) `AuthService` ; (futur) annotations HTTP pour la Gateway. |

Signature illustrative de la couche storage (cible) :

```go
// Illustratif — user_id vient du context (injecté par l'intercepteur), pas d'un argument client
func (s *Store) GetLists(ctx context.Context, userID string) ([]ListSummary, error)
func (s *Store) CreateList(ctx context.Context, userID, title string, items []Item) error
```

## 5. Flux d'authentification et multi-device

### 5.1 Signup / Login

```
1. todo signup --email <e> (mot de passe saisi via prompt)   ┐
   todo login  --email <e>                                    │ HTTPS → Supabase Auth
2. Supabase Auth vérifie et renvoie { access_token (court),   ┘
   refresh_token (long) }
3. Le CLI écrit les tokens dans ~/.config/todolist/credentials.json  (perms 0600)
4. Les appels gRPC métier attachent: authorization: Bearer <access_token>
```

### 5.2 Stockage du token côté client

- Fichier **`~/.config/todolist/credentials.json`**, permissions **`0600`** (lecture/écriture propriétaire uniquement). C'est le même répertoire que l'actuel `data.json`, mais ce dernier disparaît (données désormais en base).
- Contenu : `access_token`, `refresh_token`, `expires_at`, `endpoint` (serveur). **Aucun secret dans le repo.**

### 5.3 Refresh

- Quand l'access JWT est expiré (réponse `Unauthenticated` ou `expires_at` dépassé), le CLI utilise le **refresh_token** pour obtenir un nouvel access JWT (auprès de Supabase Auth, ou via `AuthService.Refresh` en mode maison), puis rejoue l'appel. Transparent pour l'utilisateur.
- `logout` supprime `credentials.json` (et révoque le refresh côté provider si supporté).

### 5.4 Multi-device

```
PC A ──login──▶ Supabase Auth ──▶ credentials.json (PC A)  ┐
PC B ──login──▶ Supabase Auth ──▶ credentials.json (PC B)  ┘  même compte (même user_id)
        │                                  │
        └────── gRPC over TLS ─────────────┘
                        ▼
              serveur gRPC distant
                        ▼
          Postgres (lists/items WHERE user_id = …)
```

- Même compte connecté sur plusieurs machines ; **chaque device a ses propres tokens** mais le **même `user_id`** → il voit les mêmes données.
- La **cohérence** entre devices est garantie par la base (transactions, `UNIQUE(user_id, title)`), ce qui résout l'écart #4 de l'analyse (écritures concurrentes) que le fichier JSON ne gérait pas.

### 5.5 Vérification des JWT côté serveur (modes d'auth)

Le serveur **valide** seulement les JWT entrants (signature + expiry + `sub`) ; il
ne parle jamais à Supabase. Trois modes existent, sélectionnés par environnement
dans `AuthInterceptorFromEnv` (`server/authconfig.go`), du plus spécifique au
fallback :

| Précédence | Variable(s) | Vérificateur | Usage |
| --- | --- | --- | --- |
| 1 (gagne toujours) | `JWT_SIGNING_KEY` | `HMACVerifier` (HS256) | Override **home/dev** — un développeur peut toujours forcer le secret maison. |
| 2 | `SUPABASE_URL` **ou** `SUPABASE_JWKS_URL` | `JWKSVerifier` (ES256/RS256) | **Mode JWKS / cible (Option B)** : valide les tokens asymétriques via la clé publique du projet. |
| 3 | `SUPABASE_JWT_SECRET` | `HMACVerifier` (HS256) | **Legacy (Option A)** : valide les tokens HS256 contre le secret partagé du projet. |
| 4 (défaut) | — (aucune) | `DevUserIDInterceptor` | Auth OFF — le run local par défaut continue de marcher. |

- **Mode JWKS (Option B, cible).** Quand `SUPABASE_URL` est défini, le serveur
  dérive l'endpoint JWKS `{SUPABASE_URL}/auth/v1/.well-known/jwks.json` (même
  préfixe `/auth/v1` que le CLI). `SUPABASE_JWKS_URL` permet d'**override** cet
  endpoint avec une URL complète (tests / déploiements non standard) et prime sur
  `SUPABASE_URL`. Le `JWKSVerifier` récupère la (les) clé(s) publique(s) de
  façon **paresseuse au premier `Verify`** (le démarrage du serveur ne dépend pas
  de la joignabilité du JWKS), les met en cache par `kid` avec un TTL et un
  refetch sur `kid` inconnu (rotation de clés), et vérifie la signature en
  ES256/RS256.
- **La clé de signature ACTIVE du projet Supabase peut rester ECC/ES256.** C'est
  précisément le choix Option B : plus besoin de rétrograder le projet en HS256.
  Le serveur valide directement les tokens asymétriques via le JWKS public.
- **Forcer le legacy HS256 (Option A).** Pour rester sur le secret HS256
  partagé, ne définir que `SUPABASE_JWT_SECRET` côté serveur (sans `SUPABASE_URL`
  ni `SUPABASE_JWKS_URL`). Cela suppose que le projet émet toujours des tokens
  HS256.
- Les trois modes renvoient la même `Identity{UserID: sub, Email: <claim email>}`,
  donc l'isolation par `user_id` et le provisioning JIT des `users` fonctionnent à
  l'identique quel que soit le mode.

## 6. Changements de configuration et de déploiement

### 6.1 Du daemon launchd local au serveur hébergé

- **Aujourd'hui** : le serveur tourne en local via **launchd** (`docs/daemon-setup.md`) sur la machine de l'utilisateur, écoute `127.0.0.1:50051`.
- **Cible** : le serveur est **hébergé sur un PaaS free tier (Fly.io)**, accessible publiquement en **TLS**. **Fin du launchd local** ; `daemon-setup.md` deviendra "déploiement serveur distant". Le CLI local ne lance plus de serveur.

### 6.2 Variables d'environnement et secrets

Côté **serveur** (jamais dans le repo — secrets PaaS / env) :

| Variable | Rôle |
| --- | --- |
| `DATABASE_URL` | Chaîne de connexion Postgres (Neon) — secret |
| `SUPABASE_URL` | URL projet bare (`https://<ref>.supabase.co`) → active le **mode JWKS / ES256 (Option B)** ; le serveur dérive `{SUPABASE_URL}/auth/v1/.well-known/jwks.json` |
| `SUPABASE_JWKS_URL` | Override de l'endpoint JWKS (URL complète) ; prime sur `SUPABASE_URL` |
| `SUPABASE_JWT_SECRET` | Secret HS256 legacy pour **valider** les JWT (mode Supabase **Option A**) |
| `JWT_SIGNING_KEY` | Secret de signature (mode auth maison/dev — gagne sur tout le reste) |
| `TLS_CERT_PATH` / `TLS_KEY_PATH` | Certificat TLS (ou TLS terminé par le PaaS) |
| `PORT` | Port d'écoute fourni par le PaaS |

Précédence d'auth serveur : `JWT_SIGNING_KEY` (home/dev) > `SUPABASE_URL`/`SUPABASE_JWKS_URL` (JWKS, cible) > `SUPABASE_JWT_SECRET` (HS256 legacy) > aucune (auth OFF). Détail en §5.5.

Côté **CLI** :

| Variable / flag | Rôle |
| --- | --- |
| `TODO_SERVER_ENDPOINT` (env) ou `--endpoint` (flag) ou config | Adresse du serveur gRPC distant (remplace le `127.0.0.1:50051` codé en dur) |
| `~/.config/todolist/credentials.json` (0600) | Tokens d'auth (généré par `login`) |

### 6.3 Gestion des secrets

- Secrets stockés via le **secret manager du PaaS** (Fly.io secrets) et/ou variables d'env d'exécution. **Jamais** committés.
- `DATABASE_URL` et le secret JWT ne quittent jamais le serveur ; le CLI ne connaît **que** son endpoint et ses tokens.

## 7. Prérequis sécurité (rappel de l'analyse §8)

Repris de `remote-storage-analysis.md` §8 — à traiter à l'implémentation (étape suivante) :

- [ ] **TLS** sur le transport gRPC (remplace `insecure.NewCredentials()`).
- [ ] **Intercepteur d'authentification** serveur : validation JWT → `user_id` dans le `context`.
- [ ] **Isolation par `user_id`** dans chaque RPC et chaque requête SQL **paramétrée**.
- [ ] **Secrets** (`DATABASE_URL`, secret JWT) via env / secret manager, hors repo.
- [ ] **Timeouts / retries** côté client gRPC (absents aujourd'hui), pour un réseau distant.
- [ ] **Sauvegardes** et plan de restauration de la base.

---

*Document de décision/conception uniquement — aucune implémentation de production. DDL et signatures fournis à titre illustratif. Étape suivante : plan d'implémentation détaillé.*
