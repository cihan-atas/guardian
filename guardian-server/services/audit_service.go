package services

import (
	"database/sql"
	"log"
	"net/http"
)

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
	ActionLogin         AuditAction = "LOGIN"
	ActionUpdateSetting AuditAction = "UPDATE_SETTINGS"
	ActionCreateAdmin   AuditAction = "CREATE_ADMIN"
	ActionUpdateAdmin   AuditAction = "UPDATE_ADMIN"
	ActionDeleteAdmin   AuditAction = "DELETE_ADMIN"
	ActionRequestAccess AuditAction = "REQUEST_ACCESS"
	ActionApproveAccess AuditAction = "APPROVE_ACCESS"
	ActionRejectAccess  AuditAction = "REJECT_ACCESS"
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
	// Gerçek yönetici kimliğini (RBAC) context'ten oku; yoksa 'system'.
	adminRef := "system"
	if r != nil {
		if ident, ok := IdentityFromContext(r.Context()); ok && ident != nil && ident.Username != "" {
			adminRef = ident.Username
		}
	}
	if len(adminRef) > 255 {
		adminRef = adminRef[:255]
	}
	logEntry.AdminRef = adminRef

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
