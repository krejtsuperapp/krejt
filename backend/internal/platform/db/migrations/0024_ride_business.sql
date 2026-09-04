-- Udhëtimi i paguar nga ndërmarrja (§18, §34).
--
-- Metoda e re rri krah `cash` dhe `wallet` e nuk i zëvendëson: i njëjti punonjës mund të udhëtojë
-- sot për punë dhe nesër për vete, dhe të dyja duhet të ekzistojnë njëkohësisht.
--
-- `business_id` mbahet te vetë udhëtimi e nuk lexohet nga anëtarësia në kohën e faturimit: një
-- punonjës mund të largohet nga ndërmarrja pasi ka udhëtuar, dhe ai udhëtim mbetet i saj.

ALTER TABLE rides DROP CONSTRAINT IF EXISTS rides_payment_method_check;
ALTER TABLE rides ADD CONSTRAINT rides_payment_method_check
  CHECK (payment_method IN ('cash','wallet','card','business'));

ALTER TABLE rides ADD COLUMN business_id uuid REFERENCES businesses(id) ON DELETE SET NULL;

-- Ndërmarrja jepet saktësisht kur pagesa është e saj. Pa këtë, një udhëtim me metodë `business`
-- dhe pa ndërmarrje do të mbetej i papagueshëm, dhe një me ndërmarrje e pa metodën do të fshihte
-- një lidhje që askush nuk e përdor.
ALTER TABLE rides ADD CONSTRAINT rides_business_method
  CHECK ((payment_method = 'business') = (business_id IS NOT NULL));

CREATE INDEX rides_business_idx ON rides (business_id, requested_at DESC) WHERE business_id IS NOT NULL;
