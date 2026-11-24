CREATE TABLE ipv6_prefixes (
    id TEXT PRIMARY KEY,
    label TEXT NOT NULL,
    interface TEXT NOT NULL,
    prefix TEXT NOT NULL,
    UNIQUE (interface, prefix)
);

ALTER TABLE ipv6_configs ADD COLUMN prefix_id TEXT REFERENCES ipv6_prefixes(id) ON DELETE SET NULL;
CREATE INDEX ipv6_configs_prefix_id ON ipv6_configs(prefix_id);
