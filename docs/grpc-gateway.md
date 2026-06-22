# Gateway REST/JSON (gRPC-Gateway) — Phase 7

> Statut : **hors v1** (le front web lui-même reste hors périmètre). Ce document
> décrit comment exposer le **même** service gRPC en **REST/JSON** pour un futur
> front web, conformément à `docs/target-architecture.md` §3 et à
> `docs/implementation-plan.md` Phase 7.

La Phase 7 est livrée en **deux volets complémentaires** :

| Volet | Quoi | État | Outillage requis |
|-------|------|------|------------------|
| **A — opérationnel** | Adaptateur REST/JSON écrit à la main (`cmd/gateway`) qui réutilise le client gRPC déjà généré | **Fonctionne aujourd'hui**, `go build`/`go test` verts | Aucun (stdlib + `protojson` déjà présents) |
| **B — canonique** | Annotations `google.api.http` dans le `.proto` + config `buf` pour générer le stub officiel `*.gw.pb.go` | **Config + annotations présentes**, génération à exécuter quand l'outillage est installé | `buf` + plugins `protoc-gen-*` |

Les deux volets exposent **exactement le même mapping REST** : le volet A est
l'équivalent manuel des annotations du volet B. On peut donc commencer avec le
volet A et basculer vers le stub généré (volet B) sans changer le contrat HTTP.

---

## 1. Mapping REST des 7 RPC

Identique entre le volet A (`cmd/gateway/handler.go`) et le volet B (annotations
`google.api.http` dans `proto/todoList.proto`) :

| RPC gRPC | Méthode + chemin HTTP | Corps (body) | `{title}` |
|----------|-----------------------|--------------|-----------|
| `CreateTodoList` | `POST /v1/lists` | `*` (toute la requête) | — |
| `GetTodoLists` | `GET /v1/lists` | — | — |
| `DeleteTodoList` | `DELETE /v1/lists` | `*` (liste de titres) | — |
| `ShowTodoListItems` | `GET /v1/lists/{title}/items` | — | depuis le path |
| `UpdateTodoList` | `PUT /v1/lists/{title}/items` | `*` | depuis le path |
| `UpdateTodoListItem` | `PATCH /v1/lists/{title}/items` | `*` | depuis le path |
| `DeleteTodoListItems` | `DELETE /v1/lists/{title}/items` | `*` (indexes) | depuis le path |

Pour les routes avec `{title}`, la valeur du **path écrase** un éventuel `title`
présent dans le body (même sémantique des deux côtés).

La (dé)sérialisation utilise **protojson** (JSON proto3 canonique, champs en
`camelCase` : `dueDate`, `itemIndexes`, `newTitle`…), cohérent avec le stub
gateway officiel.

---

## 2. Authentification : le Bearer est relayé, jamais validé par la gateway

Point central : la gateway **ne valide pas** le JWT. Elle se contente de
**relayer** l'en-tête HTTP `Authorization` vers la **metadata gRPC**
`authorization`. C'est l'intercepteur d'auth du serveur (Phase 3,
`server/authinterceptor.go`) qui valide le token et en dérive le `user_id` —
l'isolation par utilisateur est donc préservée **de bout en bout** :

```
Front web  --HTTP-->  gateway  --gRPC metadata authorization: Bearer <jwt>-->  serveur gRPC
                       (relais)                                                  (validation + user_id)
```

Codes d'erreur : la gateway traduit les codes gRPC en codes HTTP
(`Unauthenticated` → 401, `NotFound` → 404, `AlreadyExists` → 409,
`InvalidArgument` → 400, `Unavailable` → 503, `DeadlineExceeded` → 504, etc.).
Un 401 côté front signale qu'il faut se ré-authentifier (miroir du refresh CLI).

---

## 3. Volet A — lancer la gateway manuelle (aujourd'hui)

Aucune génération de code n'est nécessaire.

```sh
# 1) Démarrer le serveur gRPC (local, défaut JSON/insecure)
make run-server          # écoute 127.0.0.1:50051

# 2) Démarrer la gateway REST/JSON
go run ./cmd/gateway     # écoute 127.0.0.1:8080 (loopback only par défaut)
```

Variables d'environnement (`cmd/gateway/config.go`) :

| Variable | Rôle | Défaut |
|----------|------|--------|
| `TODO_GATEWAY_ADDR` | Adresse d'écoute HTTP de la gateway | `127.0.0.1:8080` (loopback) |
| `TODO_SERVER_ENDPOINT` | Serveur gRPC amont (même var que le CLI) | `127.0.0.1:50051` |
| `TODO_GATEWAY_UPSTREAM_TLS` | TLS vers l'amont via pool système (cible prod) | insecure |

Exemples `curl` (Bearer relayé en metadata) :

```sh
# Lister mes todolists (le JWT identifie l'utilisateur côté serveur)
curl -H "Authorization: Bearer $JWT" http://127.0.0.1:8080/v1/lists

# Créer une liste
curl -X POST http://127.0.0.1:8080/v1/lists \
  -H "Authorization: Bearer $JWT" \
  -H "Content-Type: application/json" \
  -d '{"title":"courses","item":[{"title":"lait"}]}'

# Voir les items d'une liste (titre dans le path)
curl -H "Authorization: Bearer $JWT" http://127.0.0.1:8080/v1/lists/courses/items
```

> Sans serveur d'auth activé (défaut local), le serveur utilise l'intercepteur
> dev (Phase 2) et accepte les appels non authentifiés. Avec `JWT_SIGNING_KEY`
> ou `SUPABASE_JWT_SECRET` côté serveur, un `Authorization: Bearer <jwt>` valide
> est requis (sinon 401).

---

## 4. Volet B — régénérer le stub gateway officiel (`buf`)

Quand on veut le stub canonique `proto/todoList.pb.gw.pb.go` (généré par
`protoc-gen-grpc-gateway`), il faut **`buf`** car l'annotation
`import "google/api/annotations.proto"` **n'est pas vendorée** dans ce repo :
`buf` la résout via la dépendance distante `buf.build/googleapis/googleapis`
(déclarée dans `buf.yaml`).

### ⚠️ Important : `make proto-protoc` (plain protoc) ne fonctionne plus

Tant que les annotations `google.api.http` et leur
`import "google/api/annotations.proto"` sont présents dans
`proto/todoList.proto`, la cible **legacy `make proto-protoc`** (plain `protoc`,
go/go-grpc uniquement) **échoue** : `protoc` ne sait pas résoudre
`google/api/annotations.proto` (absent du include path, non vendoré). Ce n'est
donc **plus un repli viable**.

La voie à utiliser est **`buf`** :

```sh
# Prérequis (une fois) : installer buf + les plugins protoc-gen-*
go install github.com/bufbuild/buf/cmd/buf@latest
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest

# Générer les stubs Go/gRPC + le stub gateway
make proto              # = buf dep update && buf generate
# ou, équivalent explicite pour la gateway :
make proto-gateway

# La régénération introduit la dépendance grpc-gateway/v2 -> mettre à jour go.mod
go mod tidy
```

> Pour réutiliser la cible legacy `proto-protoc` (sans buf), il faudrait soit
> retirer les annotations + l'import du `.proto`, soit vendorer
> `google/api/annotations.proto` et l'ajouter au include path de `protoc`.
> Tant que ce n'est pas le cas, **utiliser `buf`**.

Fichiers de config :
- `buf.yaml` (v2) — module + dépendance `googleapis` + règles lint/breaking.
- `buf.gen.yaml` (v2) — plugins `go`, `go-grpc`, `grpc-gateway` (out `source_relative`).

Après génération, un binaire serveur gateway officiel (basé sur
`runtime.ServeMux` de grpc-gateway) pourrait remplacer le volet A — le contrat
REST restant identique.

---

## 5. Hors scope

- **Le front web lui-même** (HTML/JS/SPA) : hors v1.
- La validation du JWT dans la gateway : elle ne fait que **relayer** le Bearer ;
  la validation reste côté serveur gRPC (intercepteur Phase 3).
- Tout changement du contrat ou de la sémantique des 7 RPC.
- Le déploiement réel de la gateway (exposition publique, TLS entrant) : à
  traiter dans `docs/deployment.md` au moment où le front web sera développé.
