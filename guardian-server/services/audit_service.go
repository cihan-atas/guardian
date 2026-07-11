package services

import (
	"database/sql"
	"log"
	"net/http"
)

type adminTokenKey string

const AdminTokenContextKey = adminTokenKey("adminToken")

type AuditAction string

const (
	ActionCreateServer  AuditAction = "CREATE_SERVER"
	ActionUpdateServer  AuditAction = "UPDATE_SERVER"
	ActionPatchServer   AuditAction = "PATCH_SERVER"
	ActionDeleteServer  AuditAction = "DELETE_SERVER"
	ActionCreateUser    AuditAction = "CREATE_USER"
	ActionPatchUser     AuditAction = "PATCH_USER"
	ActionDeleteUser    AuditAction = "DELETE_USER"
	ActionCreateKey     AuditAction = "CREATE_KEY"
	ActionPatchKey      AuditAction = "PATCH_KEY"
	ActionDeleteKey     AuditAction = "DELETE_KEY"
	ActionCreateRule    AuditAction = "CREATE_RULE"
	ActionUpdateRule    AuditAction = "UPDATE_RULE"
	ActionPatchRule     AuditAction = "PATCH_RULE"
	ActionDeleteRule    AuditAction = "DELETE_RULE"
	ActionTerminateSess AuditAction = "TERMINATE_SESSION"
	ActionBanKey        AuditAction = "BAN_KEY"
	ActionUnbanKey      AuditAction = "UNBAN_KEY"
)

type AuditLog struct {
	AdminRef     string
	Action       AuditAction
	TargetType   string
	TargetID     int
	Status       string
	ErrorMessage string
}

func Record(db *sql.DB, r *http.Request, logEntry AuditLog) {
	var adminToken string

	if r != nil {
		token, ok := r.Context().Value(AdminTokenContextKey).(string)
		if ok && token != "" {
			adminToken = token
		}
	}

	if adminToken == "" {
		log.Printf("[INFO] Audit log: Context'te admin token bulunamadı. 'system' olarak kaydediliyor.")
		adminToken = "system"
	}

	if len(adminToken) > 8 {
		logEntry.AdminRef = adminToken[:8]
	} else {
		logEntry.AdminRef = adminToken
	}

	query := `
  		INSERT INTO audit_logs (admin_ref, action, target_type, target_id, status, error_message)
  		VALUES ($1, $2, $3, $4, $5, $6)`

	go func() {
		_, err := db.Exec(query,
			logEntry.AdminRef,
			logEntry.Action,
			logEntry.TargetType,
			sql.NullInt64{Int64: int64(logEntry.TargetID), Valid: logEntry.TargetID > 0},
			logEntry.Status,
			sql.NullString{String: logEntry.ErrorMessage, Valid: logEntry.ErrorMessage != ""},
		)

		if err != nil {
			log.Printf("[CRITICAL] DENETİM KAYDI YAZILAMADI: %v", err)
		}
	}()
}
