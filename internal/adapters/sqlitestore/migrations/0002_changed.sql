-- 0002: a definição divergente que o servidor anunciou depois do pin.
-- Pins antigos ficam como estavam: changed começa em 0.
ALTER TABLE pins ADD COLUMN changed INTEGER NOT NULL DEFAULT 0;
ALTER TABLE pins ADD COLUMN seen_fingerprint TEXT;
ALTER TABLE pins ADD COLUMN seen_definition TEXT;
ALTER TABLE pins ADD COLUMN changed_at TEXT;
