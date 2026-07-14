package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"

	"guardian.com/server/services"
)

// writeJSON verilen değeri JSON olarak yazar.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// withIdentity, audit kaydında doğru kimliğin görünmesi için verilen kimliği
// request context'ine yerleştiren bir context döndürür.
func withIdentity(r *http.Request, ident *services.AdminIdentity) context.Context {
	return context.WithValue(r.Context(), services.AdminIdentityContextKey, ident)
}

// commitWithAudit, SUCCESS denetim kaydını mutasyonla AYNI transaction'a yazıp
// commit eder. Audit yazımı veya commit başarısızsa hata döner; çağıran zaten
// `defer tx.Rollback()` ile mutasyonu geri alır → mutasyon ve denetim izi ya
// birlikte kalıcı olur ya hiç olmaz (aynı-transaction bütünlüğü).
func commitWithAudit(tx *sql.Tx, r *http.Request, logEntry services.AuditLog) error {
	if err := services.RecordTx(tx, r, logEntry); err != nil {
		return err
	}
	return tx.Commit()
}
