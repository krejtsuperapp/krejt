-- Të preferuarat e klientit (§21): lokalet dhe dyqanet që ai i ruan vetë.
--
-- Pa distancë dhe pa renditje sipas afërsisë: një restorant i preferuar mbetet i preferuar edhe
-- kur je në qytet tjetër. Zbulimi e pret listën te 15 km; kjo jo.
--
-- Fshirja e llogarisë ose e tregtarit e heq rreshtin vetvetiu (ON DELETE CASCADE): një e preferuar
-- e mbetur pas një tregtari të fshirë do të ishte një rresht që nuk hapet dot.

CREATE TABLE merchant_favourites (
  user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  merchant_id uuid NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
  created_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, merchant_id)
);

-- Lista e një përdoruesi lexohet gjithmonë nga më e reja te më e vjetra.
CREATE INDEX merchant_favourites_user_idx ON merchant_favourites (user_id, created_at DESC);
