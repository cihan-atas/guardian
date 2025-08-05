package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"guardian.com/server/services"
)

func seedExampleData(db *sql.DB) error {
	log.Println("⏳ Örnek veriler kontrol ediliyor...")

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("transaction başlatılamadı: %w", err)
	}
	defer tx.Rollback()

	serverID, err := createExampleServer(tx)
	if err != nil {
		return err
	}

	userID, err := createExampleUser(tx)
	if err != nil {
		return err
	}

	keyID, err := createExampleKey(tx)
	if err != nil {
		return err
	}

	if serverID > 0 && userID > 0 && keyID > 0 {
		if err := createExampleRule(tx, serverID, userID, keyID); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("transaction onaylanamadı: %w", err)
	}

	log.Println("✅ Örnek veri kontrolü tamamlandı.")
	return nil
}

func createExampleServer(tx *sql.Tx) (int, error) {
	var serverID int
	err := tx.QueryRow("SELECT id FROM servers WHERE hostname = 'example-server'").Scan(&serverID)
	if err != nil {
		if err == sql.ErrNoRows {
			err = tx.QueryRow(`
				INSERT INTO servers (hostname, ip_address, description) 
				VALUES ('example-server', '192.0.2.1', 'Bu, silinemeyen bir örnek sunucudur.')
				RETURNING id
			`).Scan(&serverID)
			if err != nil {
				return 0, fmt.Errorf("örnek sunucu oluşturulamadı: %w", err)
			}
			log.Println("  -> Örnek sunucu oluşturuldu.")
			return serverID, nil
		}
		return 0, err
	}
	return serverID, nil
}

func createExampleUser(tx *sql.Tx) (int, error) {
	var userID int
	err := tx.QueryRow("SELECT id FROM system_users WHERE username = 'example-user'").Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows {
			err = tx.QueryRow(`
				INSERT INTO system_users (username, description) 
				VALUES ('example-user', 'Bu, silinemeyen bir örnek kullanıcıdır.')
				RETURNING id
			`).Scan(&userID)
			if err != nil {
				return 0, fmt.Errorf("örnek kullanıcı oluşturulamadı: %w", err)
			}
			log.Println("  -> Örnek kullanıcı oluşturuldu.")
			return userID, nil
		}
		return 0, err
	}
	return userID, nil
}

func createExampleKey(tx *sql.Tx) (int, error) {
	var keyID int
	err := tx.QueryRow("SELECT id FROM public_keys WHERE key_name = 'example-key'").Scan(&keyID)
	if err != nil {
		if err == sql.ErrNoRows {
			examplePublicKey := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCzH6dGjfsXA33t8CjlplE6OsEzoliynScv4m+Hx+l6LphnSI5WUO292U6KSj5vfCmzfu6OKHTbV/IBZwxq9tv5IJRSKIvyUwlXMaCYAVxIEJ9gE0jU4O9cnQ9xWLeQNPIshP2tm7389ZJAFz2YKW3u4UPTNo4Mdm4Bu7TpiQC4AlPCeYsc6+vbv3MNomIWlObxGpnFw8uoiMiOPIDqTnj0QsEueKgqG24XrGqVYvMDXt/BYlQEhzHuzucYpk3eBDSzqORZnIiA7zmDPWxBJKAikzaSstskJTYSoizx0k0F8XQBpLbwQpsDebafGBfo82myyFrb7xaJZw7egWTePWOX example@guardian"
			fingerprint, err := services.GenerateFingerprint(examplePublicKey)
			if err != nil {
				return 0, fmt.Errorf("örnek anahtar için parmak izi oluşturulamadı: %w", err)
			}
			err = tx.QueryRow(`
				INSERT INTO public_keys (key_name, ssh_public_key, fingerprint_sha256) 
				VALUES ('example-key', $1, $2)
				RETURNING id
			`, examplePublicKey, fingerprint).Scan(&keyID)
			if err != nil {
				return 0, fmt.Errorf("örnek anahtar oluşturulamadı: %w", err)
			}
			log.Println("  -> Örnek anahtar oluşturuldu.")
			return keyID, nil
		}
		return 0, err
	}
	return keyID, nil
}

func createExampleRule(tx *sql.Tx, serverID, userID, keyID int) error {
	var count int
	err := tx.QueryRow("SELECT COUNT(*) FROM access_rules WHERE server_id = $1 AND system_user_id = $2 AND public_key_id = $3", serverID, userID, keyID).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {

		validFrom := time.Now().UTC()
		validUntil := validFrom.Add(100 * 365 * 24 * time.Hour) // 100 yıl sonrası

		_, err := tx.Exec(`
			INSERT INTO access_rules (server_id, system_user_id, public_key_id, valid_from, valid_until, status)
			VALUES ($1, $2, $3, $4, $5, 'active')
		`, serverID, userID, keyID, validFrom, validUntil)

		if err != nil {
			return fmt.Errorf("örnek kural oluşturulamadı: %w", err)
		}
		log.Println("  -> Örnek kural oluşturuldu.")
	}
	return nil
}
