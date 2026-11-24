package model

import (
	"crypto/sha256"
	"net/netip"

	"golang.org/x/crypto/bcrypt"
)

// IPv6Account is the stored credential and source address for one SOCKS5 user.
type IPv6Account struct {
	Address      netip.Prefix `json:"address"`
	PasswordHash string       `json:"password_hash"`
}

func (a *IPv6Account) SetPassword(password string) error {
	// RFC 1929 allows up to 255 bytes; bcrypt accepts at most 72.
	digest := sha256.Sum256([]byte(password))
	hash, err := bcrypt.GenerateFromPassword(digest[:], bcrypt.DefaultCost)
	if err == nil {
		a.PasswordHash = string(hash)
	}
	return err
}

func (a IPv6Account) Authenticate(password []byte) bool {
	digest := sha256.Sum256(password)
	return len(password) > 0 && bcrypt.CompareHashAndPassword([]byte(a.PasswordHash), digest[:]) == nil
}
