package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"guardian.com/server/services"
)

// RenewServerCert (admin): Guardian sunucusunun kendi TLS sertifikasını seçilen
// süreyle (validity_days) yeniden imzalar. Yeni sertifikanın etkin olması için
// sunucunun yeniden başlatılması gerekir. POST /api/certificates/server/renew
func RenewServerCert(db *sql.DB, ca *services.CA, certPath, keyPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ca == nil {
			http.Error(w, "CA anahtarı yapılandırılmadığı için yenileme yapılamıyor (TLS_CA_KEY_FILE).", http.StatusServiceUnavailable)
			return
		}
		var body struct {
			ValidityDays int `json:"validity_days"`
		}
		json.NewDecoder(r.Body).Decode(&body)

		info, err := ca.RenewServerCert(certPath, keyPath, body.ValidityDays)
		if err != nil {
			http.Error(w, "Server sertifikası yenilenemedi: "+err.Error(), http.StatusInternalServerError)
			return
		}
		services.Record(db, r, services.AuditLog{Action: services.ActionUpdateSetting, TargetType: "server_cert", Status: "SUCCESS"})

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"cert":             info,
			"restart_required": true,
		})
	}
}
