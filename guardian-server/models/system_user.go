// guardian/guardian-server/models/system_user.go

package models

import (
	"database/sql" // sql paketini import et
	"time"
)

type SystemUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	// DEĞİŞİKLİK: string yerine sql.NullString kullan
	Description sql.NullString `json:"description,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}
