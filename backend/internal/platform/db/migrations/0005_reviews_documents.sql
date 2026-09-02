-- =============================================================================
-- KREJT — 0005: vlerësimet dyanëshe të udhëtimit (§30) dhe dokumentet e shoferit me skadim (§18).
-- =============================================================================

-- ------------------------------ vlerësimet --------------------------------------
CREATE TABLE ride_reviews (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  ride_id           uuid NOT NULL REFERENCES rides(id) ON DELETE CASCADE,
  reviewer_id       uuid NOT NULL REFERENCES users(id),
  reviewee_id       uuid NOT NULL REFERENCES users(id),
  reviewer_role     text NOT NULL CHECK (reviewer_role IN ('customer','driver')),
  rating            smallint NOT NULL CHECK (rating BETWEEN 1 AND 5),
  tags              text[] NOT NULL DEFAULT '{}',
  comment           text,
  moderation_status text NOT NULL DEFAULT 'visible' CHECK (moderation_status IN ('visible','flagged','hidden')),
  report_reason     text,
  reported_at       timestamptz,
  created_at        timestamptz NOT NULL DEFAULT now(),
  UNIQUE (ride_id, reviewer_id)                      -- një vlerësim për person për udhëtim (anti-abuzim)
);
CREATE INDEX ride_reviews_reviewee_idx ON ride_reviews(reviewee_id, created_at DESC);
CREATE INDEX ride_reviews_flagged_idx ON ride_reviews(reported_at) WHERE moderation_status = 'flagged';

-- vlerësimi i klientit (shoferët vlerësojnë klientët) — agregat, si te drivers
ALTER TABLE users
  ADD COLUMN rating_sum   int NOT NULL DEFAULT 0,
  ADD COLUMN rating_count int NOT NULL DEFAULT 0;

-- ------------------------------ dokumentet e shoferit ----------------------------
CREATE TABLE driver_documents (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  driver_id        uuid NOT NULL REFERENCES drivers(user_id) ON DELETE CASCADE,
  type             text NOT NULL CHECK (type IN (
                     'driving_license','id_card','vehicle_registration','insurance','taxi_permit','criminal_record','profile_photo')),
  object_key       text NOT NULL UNIQUE,              -- S3: drivers/{driver_id}/{type}/{uuid}.{ext}
  content_type     text NOT NULL,
  size_bytes       bigint NOT NULL CHECK (size_bytes > 0),
  expires_on       date,                              -- skadimi i dokumentit (patentë, sigurim, certifikatë)
  status           text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected','expired','replaced')),
  rejection_reason text,
  reviewed_by      uuid REFERENCES users(id),
  reviewed_at      timestamptz,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX driver_documents_driver_idx ON driver_documents(driver_id, type, created_at DESC);
CREATE INDEX driver_documents_pending_idx ON driver_documents(created_at) WHERE status = 'pending';
CREATE INDEX driver_documents_expiry_idx ON driver_documents(expires_on) WHERE status = 'approved' AND expires_on IS NOT NULL;
