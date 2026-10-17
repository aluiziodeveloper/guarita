-- 0001: definições fixadas, uma por ferramenta em cada servidor.
CREATE TABLE pins (
    server      TEXT NOT NULL,
    tool        TEXT NOT NULL,
    fingerprint TEXT NOT NULL,
    definition  TEXT NOT NULL,
    pinned_at   TEXT NOT NULL,
    PRIMARY KEY (server, tool)
) STRICT;
