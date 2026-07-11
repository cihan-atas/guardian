package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"guardian.com/server/models"
)

// ListAuditLogs, denetim kayıtlarını filtre + sayfalamayla döndürür.
// Filtreler: ?search= (admin_ref/target_type ILIKE), ?action=, ?status=.
func ListAuditLogs(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := strconv.Atoi(r.URL.Query().Get("page"))
		if err != nil || page < 1 {
			page = 1
		}
		limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
		if err != nil || limit < 1 || limit > 200 {
			limit = 20
		}
		offset := (page - 1) * limit

		search := strings.TrimSpace(r.URL.Query().Get("search"))
		action := strings.TrimSpace(r.URL.Query().Get("action"))
		status := strings.TrimSpace(r.URL.Query().Get("status"))

		conds := []string{}
		args := []interface{}{}
		if search != "" {
			args = append(args, "%"+search+"%")
			n := len(args)
			conds = append(conds, fmt.Sprintf("(admin_ref ILIKE $%d OR target_type ILIKE $%d)", n, n))
		}
		if action != "" {
			args = append(args, action)
			conds = append(conds, fmt.Sprintf("action = $%d", len(args)))
		}
		if status != "" {
			args = append(args, status)
			conds = append(conds, fmt.Sprintf("status = $%d", len(args)))
		}
		where := ""
		if len(conds) > 0 {
			where = " WHERE " + strings.Join(conds, " AND ")
		}

		countArgs := append([]interface{}{}, args...)
		var total int
		db.QueryRow("SELECT COUNT(*) FROM audit_logs"+where, countArgs...).Scan(&total)

		args = append(args, limit, offset)
		query := fmt.Sprintf(`
			SELECT id, admin_ref, action, target_type, target_id, status, error_message, created_at
			FROM audit_logs%s
			ORDER BY created_at DESC
			LIMIT $%d OFFSET $%d`, where, len(args)-1, len(args))

		rows, err := db.Query(query, args...)
		if err != nil {
			http.Error(w, "Sunucu hatası.", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		results := []models.AuditLog{}
		for rows.Next() {
			var e models.AuditLog
			if err := rows.Scan(&e.ID, &e.AdminRef, &e.Action, &e.TargetType, &e.TargetID, &e.Status, &e.ErrorMessage, &e.CreatedAt); err != nil {
				http.Error(w, "Sunucu hatası.", http.StatusInternalServerError)
				return
			}
			results = append(results, e)
		}

		writeJSON(w, http.StatusOK, struct {
			TotalRecords int               `json:"total_records"`
			Page         int               `json:"page"`
			Limit        int               `json:"limit"`
			Data         []models.AuditLog `json:"data"`
		}{total, page, limit, results})
	}
}
