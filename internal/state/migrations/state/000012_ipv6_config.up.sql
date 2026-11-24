CREATE TABLE ipv6_configs (
    id TEXT PRIMARY KEY,
    interface TEXT NOT NULL,
    username TEXT NOT NULL,
    address TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    endpoint_ids_json TEXT NOT NULL
);

CREATE TRIGGER ipv6_configs_unbind_endpoint AFTER DELETE ON endpoints
BEGIN
    UPDATE ipv6_configs
    SET endpoint_ids_json = (
        SELECT json_group_array(value)
        FROM json_each(endpoint_ids_json)
        WHERE value != OLD.id
    )
    WHERE OLD.id IN (SELECT value FROM json_each(endpoint_ids_json));
END;
