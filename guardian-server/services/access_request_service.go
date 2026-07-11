package services

import (
	"database/sql"
	"fmt"
)

// Onay akışı için access_rules'ta kullanılan ek durumlar.
const (
	StatusAwaitingApproval = "awaiting_approval"
	StatusRejected         = "rejected"
)

// EnsureAccessRequestColumns, onay akışı için access_rules tablosuna gereken
// kolonları (yoksa) ekler. Önceden deploy edilmiş veritabanlarında bu kolonlar
// bulunmayabileceğinden açılışta otomatik migration olarak çalışır.
func EnsureAccessRequestColumns(db *sql.DB) error {
	stmts := []string{
		`ALTER TABLE access_rules ADD COLUMN IF NOT EXISTS requested_by integer`,
		`ALTER TABLE access_rules ADD COLUMN IF NOT EXISTS approved_by integer`,
		`ALTER TABLE access_rules ADD COLUMN IF NOT EXISTS request_reason text`,
		`ALTER TABLE access_rules ADD COLUMN IF NOT EXISTS reject_reason text`,
		`ALTER TABLE access_rules ADD COLUMN IF NOT EXISTS decided_at timestamptz`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("access_rules kolonu eklenemedi: %w", err)
		}
	}
	return nil
}
