-- =============================================================================
-- KREJT — 0012: kodi i marrjes (§25 ride QR, §60 trust UX): klienti sheh 4 shifra / QR të nënshkruar;
-- shoferi e fut (ose skanon) para se të nisë udhëtimin → klienti i duhur hip në makinën e duhur.
-- =============================================================================
ALTER TABLE rides ADD COLUMN pickup_code char(4);
