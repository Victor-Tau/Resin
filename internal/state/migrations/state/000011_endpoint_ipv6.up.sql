ALTER TABLE endpoints ADD COLUMN socks5_ipv6_interface TEXT NOT NULL DEFAULT '';
ALTER TABLE endpoints ADD COLUMN socks5_accounts_json TEXT NOT NULL DEFAULT '{}';
