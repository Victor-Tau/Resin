package model

import (
	"crypto/sha256"
	"net/netip"

	"golang.org/x/crypto/bcrypt"
)

// IPv6Account is the stored credential and source address for one SOCKS5 user.
type IPv6Account struct {
	Address      netip.Prefix `json:"address"`
	Password     string       `json:"password,omitempty"`
	PasswordHash string       `json:"password_hash"`
	Interface    string       `json:"-"` // Runtime override for independently configured accounts.
}

type IPv6Config struct {
	ID          string
	PrefixID    string
	Username    string
	Account     IPv6Account
	EndpointIDs []string
}

type IPv6Prefix struct {
	ID         string       `json:"id"`
	Label      string       `json:"label"`
	Interface  string       `json:"interface"`
	Prefix     netip.Prefix `json:"prefix"`
	ProxyCount int          `json:"proxy_count"`
}

func (a *IPv6Account) SetPassword(password string) error {
	// RFC 1929 allows up to 255 bytes; bcrypt accepts at most 72.
	digest := sha256.Sum256([]byte(password))
	hash, err := bcrypt.GenerateFromPassword(digest[:], bcrypt.DefaultCost)
	if err == nil {
		a.Password = password
		a.PasswordHash = string(hash)
	}
	return err
}

func (a IPv6Account) Authenticate(password []byte) bool {
	digest := sha256.Sum256(password)
	return len(password) > 0 && bcrypt.CompareHashAndPassword([]byte(a.PasswordHash), digest[:]) == nil
}
