-- Fotoja e profilit të përdoruesit (§16, §43): çelësi i objektit në bucket-in e medias.
-- Logot/kopertinat e vendeve dhe imazhet e produkteve i kanë kolonat që nga 0013.
ALTER TABLE users ADD COLUMN IF NOT EXISTS photo_key text;
