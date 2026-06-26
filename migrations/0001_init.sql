-- Migration 0001 — schéma initial Postgres pour TodoList-CLI.
--
-- Reprend le DDL illustratif de docs/target-architecture.md §2.2.
-- Phase 1 de docs/implementation-plan.md : le schéma multi-utilisateur est
-- déjà en place (user_id partout) mais l'application n'utilise qu'un user_id
-- FIXE/placeholder à ce stade ; l'authentification réelle et l'isolation par
-- utilisateur arrivent en Phases 2/3. Un utilisateur placeholder est donc
-- inséré ici pour que PgStore puisse fonctionner en mono-utilisateur.
--
-- HORS périmètre Phase 1 : auth (P3), TLS (P4), isolation user_id réelle (P2),
-- migration de data.json (P5).

CREATE EXTENSION IF NOT EXISTS pgcrypto; -- fournit gen_random_uuid()

CREATE TABLE IF NOT EXISTS users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email       TEXT UNIQUE NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS lists (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, title)            -- isolation + unicité par utilisateur
);

CREATE TABLE IF NOT EXISTS items (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    list_id     UUID NOT NULL REFERENCES lists(id) ON DELETE CASCADE,
    position    INT  NOT NULL,         -- ordre stable (remplace item_index)
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    completed   BOOLEAN NOT NULL DEFAULT false,
    due_date    TEXT NOT NULL DEFAULT '', -- conserve le format brut du proto (string) en Phase 1
    priority    TEXT NOT NULL DEFAULT '',
    UNIQUE (list_id, position)
);

CREATE INDEX IF NOT EXISTS idx_lists_user ON lists(user_id);
CREATE INDEX IF NOT EXISTS idx_items_list ON items(list_id);

-- Utilisateur placeholder utilisé par PgStore tant que l'auth n'existe pas
-- (Phase 1, mono-utilisateur). UUID fixe pour rester déterministe entre
-- redémarrages et environnements de test.
INSERT INTO users (id, email)
VALUES ('00000000-0000-0000-0000-000000000001', 'placeholder@todolist.local')
ON CONFLICT (id) DO NOTHING;
