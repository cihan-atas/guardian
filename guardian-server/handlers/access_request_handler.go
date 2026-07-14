package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/lib/pq"
	"guardian.com/server/agentclient"
	"guardian.com/server/services"
)

type accessRequestDTO struct {
	ID             int        `json:"id"`
	ServerID       int        `json:"server_id"`
	PublicKeyID    int        `json:"public_key_id"`
	SystemUserID   int        `json:"system_user_id"`
	ValidFrom      time.Time  `json:"valid_from"`
	ValidUntil     time.Time  `json:"valid_until"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	ServerHostname string     `json:"server_hostname"`
	Username       string     `json:"username"`
	KeyName        string     `json:"key_name"`
	RequestReason  string     `json:"request_reason"`
	RejectReason   string     `json:"reject_reason,omitempty"`
	RequestedBy    string     `json:"requested_by,omitempty"`
	ApprovedBy     string     `json:"approved_by,omitempty"`
	DecidedAt      *time.Time `json:"decided_at,omitempty"`
}

type createAccessRequestBody struct {
	ServerID     int       `json:"server_id"`
	PublicKeyID  int       `json:"public_key_id"`
	SystemUserID int       `json:"system_user_id"`
	ValidFrom    time.Time `json:"valid_from"`
	ValidUntil   time.Time `json:"valid_until"`
	Reason       string    `json:"reason"`
}

// CreateAccessRequest, bir operatörün erişim talebi oluşturmasını sağlar.
// Talep 'awaiting_approval' durumunda kaydedilir; scheduler tarafından
// etkinleştirilmez (yalnızca admin onayından sonra 'pending' olur).
func CreateAccessRequest(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ident, _ := services.IdentityFromContext(r.Context())
		var body createAccessRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Geçersiz istek gövdesi.", http.StatusBadRequest)
			return
		}
		if body.ServerID == 0 || body.PublicKeyID == 0 || body.SystemUserID == 0 {
			http.Error(w, "Sunucu, anahtar ve kullanıcı seçilmelidir.", http.StatusBadRequest)
			return
		}
		if body.ValidUntil.Before(time.Now()) || !body.ValidUntil.After(body.ValidFrom) {
			http.Error(w, "Geçerlilik bitişi gelecekte ve başlangıçtan sonra olmalıdır.", http.StatusBadRequest)
			return
		}

		// Referansların varlığını doğrula.
		var exists int
		if err := db.QueryRow(
			`SELECT 1 FROM servers s, system_users su, public_keys pk WHERE s.id=$1 AND su.id=$2 AND pk.id=$3`,
			body.ServerID, body.SystemUserID, body.PublicKeyID,
		).Scan(&exists); err != nil {
			http.Error(w, "Geçersiz sunucu, kullanıcı veya anahtar.", http.StatusBadRequest)
			return
		}

		// Yasaklı anahtar için talep açılamaz.
		if ban, banErr := services.ActiveBan(db, body.PublicKeyID); banErr == nil && ban != nil {
			http.Error(w, fmt.Sprintf("Bu SSH anahtarı %s tarihine kadar yasaklı.", ban.BannedUntil.Local().Format("2006-01-02 15:04")), http.StatusForbidden)
			return
		}

		var reqBy interface{}
		if ident != nil {
			reqBy = ident.ID
		}

		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "Sunucu hatası.", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		var id int
		err = tx.QueryRow(`
			INSERT INTO access_rules (server_id, public_key_id, system_user_id, valid_from, valid_until, status, requested_by, request_reason)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
			body.ServerID, body.PublicKeyID, body.SystemUserID, body.ValidFrom, body.ValidUntil,
			services.StatusAwaitingApproval, reqBy, strings.TrimSpace(body.Reason),
		).Scan(&id)
		if err != nil {
			http.Error(w, "Talep oluşturulamadı.", http.StatusInternalServerError)
			return
		}
		if err := commitWithAudit(tx, r, services.AuditLog{Action: services.ActionRequestAccess, TargetType: "access_request", TargetID: id, Status: "SUCCESS"}); err != nil {
			http.Error(w, "Sunucu hatası.", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]int{"id": id})
	}
}

// ListAccessRequests, talepleri (opsiyonel ?status= filtresiyle) listeler.
func ListAccessRequests(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := strings.TrimSpace(r.URL.Query().Get("status"))
		where := ""
		args := []interface{}{}
		if status != "" {
			parts := strings.Split(status, ",")
			where = " WHERE ar.status = ANY($1)"
			args = append(args, pq.Array(parts))
		}
		query := `
			SELECT ar.id, ar.server_id, ar.public_key_id, ar.system_user_id,
			       ar.valid_from, ar.valid_until, ar.status, ar.created_at,
			       s.hostname, su.username, pk.key_name,
			       COALESCE(ar.request_reason,''), COALESCE(ar.reject_reason,''),
			       COALESCE(req.username,''), COALESCE(apr.username,''), ar.decided_at
			FROM access_rules ar
			JOIN servers s ON ar.server_id = s.id
			JOIN system_users su ON ar.system_user_id = su.id
			JOIN public_keys pk ON ar.public_key_id = pk.id
			LEFT JOIN admin_users req ON ar.requested_by = req.id
			LEFT JOIN admin_users apr ON ar.approved_by = apr.id` + where + `
			ORDER BY ar.id DESC LIMIT 200`

		rows, err := db.Query(query, args...)
		if err != nil {
			http.Error(w, "Sunucu hatası.", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		out := []accessRequestDTO{}
		for rows.Next() {
			var d accessRequestDTO
			var decidedAt sql.NullTime
			if err := rows.Scan(
				&d.ID, &d.ServerID, &d.PublicKeyID, &d.SystemUserID,
				&d.ValidFrom, &d.ValidUntil, &d.Status, &d.CreatedAt,
				&d.ServerHostname, &d.Username, &d.KeyName,
				&d.RequestReason, &d.RejectReason, &d.RequestedBy, &d.ApprovedBy, &decidedAt,
			); err != nil {
				http.Error(w, "Sunucu hatası.", http.StatusInternalServerError)
				return
			}
			if decidedAt.Valid {
				t := decidedAt.Time
				d.DecidedAt = &t
			}
			out = append(out, d)
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// ApproveAccessRequest, bekleyen bir talebi onaylar: agent'ta kullanıcı
// doğrulanır, yasak kontrol edilir ve durum 'pending' yapılır (scheduler
// valid_from geldiğinde etkinleştirir).
func ApproveAccessRequest(db *sql.DB, ac agentclient.AgentCommunicator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(chi.URLParam(r, "requestID"))
		if err != nil {
			http.Error(w, "Geçersiz ID.", http.StatusBadRequest)
			return
		}
		ident, _ := services.IdentityFromContext(r.Context())

		var (
			serverID, keyID, userID int
			status                  string
			validUntil              time.Time
			targetIP, targetUser    string
		)
		err = db.QueryRow(`
			SELECT ar.server_id, ar.public_key_id, ar.system_user_id, ar.status, ar.valid_until, s.ip_address, su.username
			FROM access_rules ar
			JOIN servers s ON ar.server_id = s.id
			JOIN system_users su ON ar.system_user_id = su.id
			WHERE ar.id = $1`, id,
		).Scan(&serverID, &keyID, &userID, &status, &validUntil, &targetIP, &targetUser)
		if err == sql.ErrNoRows {
			http.Error(w, "Talep bulunamadı.", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "Sunucu hatası.", http.StatusInternalServerError)
			return
		}
		if status != services.StatusAwaitingApproval {
			http.Error(w, "Yalnızca onay bekleyen talepler onaylanabilir.", http.StatusConflict)
			return
		}
		if validUntil.Before(time.Now()) {
			http.Error(w, "Talebin geçerlilik süresi zaten dolmuş.", http.StatusBadRequest)
			return
		}
		if ban, banErr := services.ActiveBan(db, keyID); banErr == nil && ban != nil {
			http.Error(w, "Bu SSH anahtarı yasaklı; talep onaylanamaz.", http.StatusForbidden)
			return
		}
		if err := ac.ValidateUser(targetIP, targetUser); err != nil {
			http.Error(w, fmt.Sprintf("Kullanıcı agent'ta doğrulanamadı: %v", err), http.StatusBadRequest)
			return
		}

		var approver interface{}
		if ident != nil {
			approver = ident.ID
		}

		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "Sunucu hatası.", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		if _, err := tx.Exec(
			`UPDATE access_rules SET status = 'pending', approved_by = $1, decided_at = now() WHERE id = $2`,
			approver, id,
		); err != nil {
			http.Error(w, "Onaylanamadı.", http.StatusInternalServerError)
			return
		}
		if err := commitWithAudit(tx, r, services.AuditLog{Action: services.ActionApproveAccess, TargetType: "access_request", TargetID: id, Status: "SUCCESS"}); err != nil {
			http.Error(w, "Sunucu hatası.", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// RejectAccessRequest, bekleyen bir talebi reddeder.
func RejectAccessRequest(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(chi.URLParam(r, "requestID"))
		if err != nil {
			http.Error(w, "Geçersiz ID.", http.StatusBadRequest)
			return
		}
		ident, _ := services.IdentityFromContext(r.Context())
		var body struct {
			Reason string `json:"reason"`
		}
		json.NewDecoder(r.Body).Decode(&body)

		var approver interface{}
		if ident != nil {
			approver = ident.ID
		}

		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "Sunucu hatası.", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		res, err := tx.Exec(
			`UPDATE access_rules SET status = 'rejected', approved_by = $1, reject_reason = $2, decided_at = now()
			 WHERE id = $3 AND status = $4`,
			approver, strings.TrimSpace(body.Reason), id, services.StatusAwaitingApproval,
		)
		if err != nil {
			http.Error(w, "Reddedilemedi.", http.StatusInternalServerError)
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			http.Error(w, "Talep bulunamadı veya zaten karara bağlanmış.", http.StatusConflict)
			return
		}
		if err := commitWithAudit(tx, r, services.AuditLog{Action: services.ActionRejectAccess, TargetType: "access_request", TargetID: id, Status: "SUCCESS"}); err != nil {
			http.Error(w, "Sunucu hatası.", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
