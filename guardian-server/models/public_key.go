package models

import "time"

type PublicKey struct {
	ID                int       `json:"id"`
	KeyName           string    `json:"key_name"`
	SshPublicKey      string    `json:"ssh_public_key"`
	FingerprintSHA256 string    `json:"fingerprint_sha256,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	// Liste görünümünde aktif yasak bilgisi (LEFT JOIN key_bans).
	// Yasak yoksa null döner; UI ek istek atmadan rozeti buradan basar.
	BannedUntil *time.Time `json:"banned_until,omitempty"`
	BanReason   *string    `json:"ban_reason,omitempty"`
}

type KeyBan struct {
	ID          int       `json:"id"`
	PublicKeyID int       `json:"public_key_id"`
	Reason      string    `json:"reason,omitempty"`
	BannedAt    time.Time `json:"banned_at"`
	BannedUntil time.Time `json:"banned_until"`
}
