package state

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/netip"

	"github.com/Resinat/Resin/internal/model"
)

func (r *StateRepo) ListIPv6Configs() ([]model.IPv6Config, error) {
	rows, err := r.db.Query(`
		SELECT id, COALESCE(prefix_id, ''), interface, username, address, password_hash, endpoint_ids_json
		FROM ipv6_configs ORDER BY username, id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	configs := make([]model.IPv6Config, 0)
	for rows.Next() {
		var config model.IPv6Config
		var address, bindings string
		if err := rows.Scan(&config.ID, &config.PrefixID, &config.Account.Interface, &config.Username, &address,
			&config.Account.PasswordHash, &bindings); err != nil {
			return nil, err
		}
		config.Account.Address, err = netip.ParsePrefix(address)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(bindings), &config.EndpointIDs); err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}
	return configs, rows.Err()
}

// WriteIPv6Configs atomically saves a batch or deletes one configuration.
func (r *StateRepo) WriteIPv6Configs(configs []model.IPv6Config, deleteID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, config := range configs {
		bindings, err := json.Marshal(config.EndpointIDs)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`
			INSERT INTO ipv6_configs (id, prefix_id, interface, username, address, password_hash, endpoint_ids_json)
			VALUES (?, NULLIF(?, ''), ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				prefix_id = excluded.prefix_id, interface = excluded.interface, username = excluded.username, address = excluded.address,
				password_hash = excluded.password_hash, endpoint_ids_json = excluded.endpoint_ids_json
		`, config.ID, config.PrefixID, config.Account.Interface, config.Username, config.Account.Address.String(),
			config.Account.PasswordHash, string(bindings))
		if err != nil {
			return err
		}
	}
	if deleteID != "" {
		if _, err := tx.Exec("DELETE FROM ipv6_configs WHERE id = ?", deleteID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

const ipv6PrefixSelect = `
	SELECT id, label, interface, prefix,
	       (SELECT COUNT(*) FROM ipv6_configs WHERE prefix_id = ipv6_prefixes.id)
	FROM ipv6_prefixes
`

func scanIPv6Prefix(scan func(...any) error) (model.IPv6Prefix, error) {
	var prefix model.IPv6Prefix
	var cidr string
	if err := scan(&prefix.ID, &prefix.Label, &prefix.Interface, &cidr, &prefix.ProxyCount); err != nil {
		if err == sql.ErrNoRows {
			return prefix, ErrNotFound
		}
		return prefix, err
	}
	var err error
	prefix.Prefix, err = netip.ParsePrefix(cidr)
	return prefix, err
}

func (r *StateRepo) ListIPv6Prefixes() ([]model.IPv6Prefix, error) {
	rows, err := r.db.Query(ipv6PrefixSelect + " ORDER BY label, interface, prefix")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	prefixes := make([]model.IPv6Prefix, 0)
	for rows.Next() {
		prefix, err := scanIPv6Prefix(rows.Scan)
		if err != nil {
			return nil, err
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes, rows.Err()
}

func (r *StateRepo) GetIPv6Prefix(id string) (model.IPv6Prefix, error) {
	return scanIPv6Prefix(r.db.QueryRow(ipv6PrefixSelect+" WHERE id = ?", id).Scan)
}

func (r *StateRepo) SaveIPv6Prefix(prefix model.IPv6Prefix) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, err := r.db.Exec(`
		INSERT INTO ipv6_prefixes (id, label, interface, prefix) VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET label = excluded.label, interface = excluded.interface, prefix = excluded.prefix
	`, prefix.ID, prefix.Label, prefix.Interface, prefix.Prefix.String())
	if isSQLiteUniqueConstraint(err) {
		return fmt.Errorf("%w: IPv6 prefix already saved on this interface", ErrConflict)
	}
	return err
}

func (r *StateRepo) DeleteIPv6Prefix(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	result, err := r.db.Exec("DELETE FROM ipv6_prefixes WHERE id = ?", id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count == 0 {
		return ErrNotFound
	}
	return err
}
