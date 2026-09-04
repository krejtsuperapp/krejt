-- Vlerësimi i një porosie (§30).
--
-- Deri tani vlerësimet ekzistonin vetëm për udhëtimet, dhe `merchants.rating_sum` nuk e prekte
-- asnjë rresht kodi: çdo lokal mbetej përjetë pa yll, ndërsa lista e Ushqimit e kishte vendin
-- gati për ta treguar. Një notë që askush nuk mund ta japë nuk duhet të shfaqet fare.
--
-- Tabelë e veçantë nga `ride_reviews` me qëllim: ajo ka çelës të huaj te `rides` dhe unike sipas
-- udhëtimit. Një kolonë `subject_type` do ta bënte të pamundur ruajtjen e këtyre kufizimeve.

CREATE TABLE order_reviews (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id          uuid NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  reviewer_id       uuid NOT NULL REFERENCES users(id),
  merchant_id       uuid NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
  rating            smallint NOT NULL CHECK (rating BETWEEN 1 AND 5),
  tags              text[] NOT NULL DEFAULT '{}',
  comment           text,
  moderation_status text NOT NULL DEFAULT 'visible' CHECK (moderation_status IN ('visible','flagged','hidden')),
  report_reason     text,
  reported_at       timestamptz,
  created_at        timestamptz NOT NULL DEFAULT now(),
  -- Një vlerësim për person për porosi (anti-abuzim), si te udhëtimet.
  UNIQUE (order_id, reviewer_id)
);

CREATE INDEX order_reviews_merchant_idx ON order_reviews (merchant_id, created_at DESC);
