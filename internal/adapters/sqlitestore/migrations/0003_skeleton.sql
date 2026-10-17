-- 0003: a silhueta do nome, para achar a mesma ferramenta em dois
-- servidores. Os pins existentes recebem a silhueta calculada em SQL,
-- com a mesma regra de domain.Skeleton.
ALTER TABLE pins ADD COLUMN skeleton TEXT NOT NULL DEFAULT '';
UPDATE pins SET skeleton = replace(replace(lower(tool), '-', '_'), '.', '_');
CREATE INDEX pins_skeleton ON pins (skeleton);
