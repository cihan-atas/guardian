package services

import (
	"fmt"

	"golang.org/x/crypto/ssh"
)

func GenerateFingerprint(publicKeyStr string) (string, error) {

	pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(publicKeyStr))
	if err != nil {
		return "", fmt.Errorf("geçersiz SSH public anahtar formatı: %w", err)
	}

	fingerprint := ssh.FingerprintSHA256(pubKey)

	return fingerprint, nil
}
