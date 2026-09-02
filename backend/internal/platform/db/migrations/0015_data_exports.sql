-- Eksporti i të dhënave personale (§16). Përdoruesi e kërkon, worker-i e ndërton, objekti skadon vetë.
-- Skedari nuk shërbehet kurrë nga API-ja: shkarkohet me URL të nënshkruar jetëshkurtër, si dokumentet.

CREATE TABLE data_exports (
    id           uuid PRIMARY KEY,
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    status       text NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending', 'building', 'ready', 'failed', 'expired')),
    object_key   text,
    size_bytes   bigint,
    error_code   text,
    requested_at timestamptz NOT NULL DEFAULT now(),
    started_at   timestamptz,
    completed_at timestamptz,
    -- Objekti fshihet pas kësaj date; kërkesa mbetet si gjurmë se eksporti u bë.
    expires_at   timestamptz
);

-- Një kërkesë e hapur për përdorues: dyfishimi vetëm do të ndërtonte të njëjtin skedar dy herë.
CREATE UNIQUE INDEX data_exports_one_open_idx
    ON data_exports (user_id)
    WHERE status IN ('pending', 'building');

CREATE INDEX data_exports_user_idx ON data_exports (user_id, requested_at DESC);

-- Worker-i i merr me këtë renditje: më e vjetra e para.
CREATE INDEX data_exports_pending_idx ON data_exports (requested_at) WHERE status = 'pending';

-- Skedarët e skaduar pastrohen nga puna periodike.
CREATE INDEX data_exports_expiring_idx ON data_exports (expires_at) WHERE status = 'ready';
