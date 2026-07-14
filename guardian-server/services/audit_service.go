package services

import (
	"database/sql"
	"log"
	"net/http"
	"time"
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

// auditExecer, hem *sql.DB hem de *sql.Tx tarafından karşılanır. RecordTx'in
// aynı transaction içine yazabilmesi için minimum arayüz.
type auditExecer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

const auditInsertQuery = `
  		INSERT INTO audit_logs (admin_ref, action, target_type, target_id, status, error_message)
  		VALUES ($1, $2, $3, $4, $5, $6)`

// resolveAdminRef, denetim kaydı için gerçek yönetici kimliğini (RBAC) context'ten
// çözer; yoksa 'system'. 255 karakterle sınırlar.
func resolveAdminRef(r *http.Request) string {
	adminRef := "system"
	if r != nil {
		if ident, ok := IdentityFromContext(r.Context()); ok && ident != nil && ident.Username != "" {
			adminRef = ident.Username
		}
	}
	if len(adminRef) > 255 {
		adminRef = adminRef[:255]
	}
	return adminRef
}

func auditArgs(logEntry AuditLog) []interface{} {
	return []interface{}{
		logEntry.AdminRef,
		logEntry.Action,
		logEntry.TargetType,
		sql.NullInt64{Int64: int64(logEntry.TargetID), Valid: logEntry.TargetID > 0},
		logEntry.Status,
		sql.NullString{String: logEntry.ErrorMessage, Valid: logEntry.ErrorMessage != ""},
	}
}

// RecordTx, denetim kaydını çağıranın verdiği transaction (veya DB) üzerinden
// yazar ve hatayı DÖNDÜRÜR. Böylece mutasyon ile audit tek transaction'da atomik
// commit edilebilir: audit yazımı başarısızsa çağıran rollback eder → "ya ikisi
// birden ya hiçbiri" bütünlüğü. Yeniden deneme YOK; başarısız bir statement
// zaten transaction'ı iptal ettiğinden çağıranın rollback etmesi gerekir.
//
// SUCCESS kayıtları bu yolla yazılmalı (mutasyonla aynı tx). FAILURE kayıtları
// ise Record ile bağımsız yazılır: bağlanacak bir mutasyon yoktur ve mutasyon
// tx'i rollback edildiğinde denetim izinin kaybolmaması gerekir.
func RecordTx(ex auditExecer, r *http.Request, logEntry AuditLog) error {
	logEntry.AdminRef = resolveAdminRef(r)
	_, err := ex.Exec(auditInsertQuery, auditArgs(logEntry)...)
	return err
}

// Record, denetim kaydını BAĞIMSIZ ve SENKRON yazar (kendi bağlantısında).
// Kayıt, denetlenen işlemi yapan handler dönmeden önce kalıcı olur. Geçici DB
// hatalarına karşı kısa bir yeniden deneme yapılır.
//
// Bağımsız yazım şu durumlar için doğrudur: (1) FAILURE kayıtları — mutasyon
// gerçekleşmedi/rollback edildi, denetim izi yine de kalmalı; (2) mutasyonu
// kendi transaction'ını yöneten bir servis (ör. BanKey) yapıyorsa veya işlemin
// DB dışı yan etkileri (agent çağrısı) varsa. Mutasyonun doğrudan handler'da
// yapıldığı SUCCESS yollarında RecordTx tercih edilir (atomik bütünlük).
func Record(db *sql.DB, r *http.Request, logEntry AuditLog) {
	logEntry.AdminRef = resolveAdminRef(r)

	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 100 * time.Millisecond)
		}
		_, err = db.Exec(auditInsertQuery, auditArgs(logEntry)...)
		if err == nil {
			return
		}
	}
	log.Printf("[CRITICAL] DENETİM KAYDI YAZILAMADI (%d deneme): %v", 3, err)
}
