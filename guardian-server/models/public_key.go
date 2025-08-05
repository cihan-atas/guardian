package models

import "time"

type PublicKey struct {
	ID                int       `json:"id"`
	KeyName           string    `json:"key_name"`
	SshPublicKey      string    `json:"ssh_public_key"`
	FingerprintSHA256 string    `json:"fingerprint_sha256,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}
