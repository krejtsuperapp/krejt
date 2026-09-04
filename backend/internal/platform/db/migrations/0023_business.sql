-- KREJT Business (§34): llogaria e një ndërmarrjeje, punonjësit e saj dhe kufijtë e shpenzimit.
--
-- Kuletë e vetën te libri, me kodin `business:{id}:wallet`, si çdo llogari tjetër: kështu shpenzimi
-- i një punonjësi është një regjistrim i zakonshëm me dy hyrje dhe jo një rrugë e dytë paralele.
--
-- Kufiri mbahet te anëtarësia e jo te përdoruesi: i njëjti person mund të punojë për dy ndërmarrje
-- me dy kufij të ndryshëm, dhe kufiri i njërës nuk ka pse ta dijë tjetra.

CREATE TABLE businesses (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name          text NOT NULL,
  -- Numri unik i biznesit; e mban vetë ndërmarrja, ne nuk e verifikojmë.
  tax_id        text,
  address_line1 text,
  city          text,
  billing_email text,
  status        text NOT NULL DEFAULT 'active' CHECK (status IN ('active','suspended')),
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);

-- Anëtarësia: kush lejohet të shpenzojë, sa, dhe kush i miraton të tjerët.
CREATE TABLE business_members (
  business_id         uuid NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
  user_id             uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role                text NOT NULL CHECK (role IN ('owner','admin','member')),
  -- Kufiri mujor në cent; NULL = pa kufi. Zero do të thoshte "asnjë shpenzim", ndaj dallohen.
  monthly_limit_minor bigint CHECK (monthly_limit_minor IS NULL OR monthly_limit_minor >= 0),
  active              boolean NOT NULL DEFAULT true,
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (business_id, user_id)
);

CREATE INDEX business_members_user_idx ON business_members (user_id) WHERE active;

-- Çdo ndërmarrje ka të paktën një pronar. Zbatohet te shërbimi (heqja e pronarit të fundit
-- refuzohet), sepse një kufizim i vetëm rreshti nuk e sheh dot tabelën e tërë.

-- Shpenzimet e faturuara te ndërmarrja. Rreshti shkruhet kur udhëtimi ose porosia mbyllet, dhe
-- lidhet me regjistrimin te libri: fatura mujore nuk rillogaritet kurrë nga çmimet e ruajtura.
CREATE TABLE business_charges (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  business_id  uuid NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
  user_id      uuid NOT NULL REFERENCES users(id),
  kind         text NOT NULL CHECK (kind IN ('ride','order','parcel','service')),
  subject_id   uuid NOT NULL,
  amount_minor bigint NOT NULL CHECK (amount_minor > 0),
  currency     text NOT NULL DEFAULT 'EUR',
  -- Regjistrimi te libri; ledger_entries.id është bigserial, ndaj lidhja bëhet me transaksionin.
  tx_id        uuid REFERENCES ledger_transactions(id) ON DELETE SET NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  -- I njëjti udhëtim nuk faturohet dy herë.
  UNIQUE (kind, subject_id)
);

CREATE INDEX business_charges_period_idx ON business_charges (business_id, created_at DESC);
CREATE INDEX business_charges_user_idx ON business_charges (business_id, user_id, created_at DESC);
