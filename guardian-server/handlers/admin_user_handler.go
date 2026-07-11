package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"guardian.com/server/services"
)

type adminUserDTO struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	Role        string `json:"role"`
	DisplayName string `json:"display_name"`
	Disabled    bool   `json:"disabled"`
	CreatedAt   string `json:"created_at"`
	LastLogin   string `json:"last_login,omitempty"`
}

// ListAdminUsers tüm yönetici hesaplarını döndürür (parola hariç).
func ListAdminUsers(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(`
			SELECT id, username, role, display_name, disabled, created_at, last_login
			FROM admin_users ORDER BY id ASC`)
		if err != nil {
			http.Error(w, "Sunucu hatası.", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		users := []adminUserDTO{}
		for rows.Next() {
			var (
				u           adminUserDTO
				displayName sql.NullString
				createdAt   sql.NullTime
				lastLogin   sql.NullTime
			)
			if err := rows.Scan(&u.ID, &u.Username, &u.Role, &displayName, &u.Disabled, &createdAt, &lastLogin); err != nil {
				http.Error(w, "Sunucu hatası.", http.StatusInternalServerError)
				return
			}
			u.DisplayName = displayName.String
			if createdAt.Valid {
				u.CreatedAt = createdAt.Time.Format("2006-01-02T15:04:05Z07:00")
			}
			if lastLogin.Valid {
				u.LastLogin = lastLogin.Time.Format("2006-01-02T15:04:05Z07:00")
			}
			users = append(users, u)
		}
		writeJSON(w, http.StatusOK, users)
	}
}

type createAdminRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	Role        string `json:"role"`
	DisplayName string `json:"display_name"`
}

// CreateAdminUser yeni bir yönetici hesabı oluşturur (admin yetkisi gerekir).
func CreateAdminUser(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createAdminRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Geçersiz istek gövdesi.", http.StatusBadRequest)
			return
		}
		req.Username = strings.TrimSpace(req.Username)
		if req.Username == "" || len(req.Password) < 6 {
			http.Error(w, "Kullanıcı adı gerekli ve parola en az 6 karakter olmalı.", http.StatusBadRequest)
			return
		}
		if !services.ValidRole(req.Role) {
			http.Error(w, "Geçersiz rol.", http.StatusBadRequest)
			return
		}
		hash, err := services.HashPassword(req.Password)
		if err != nil {
			http.Error(w, "Sunucu hatası.", http.StatusInternalServerError)
			return
		}
		var id int
		err = db.QueryRow(
			`INSERT INTO admin_users (username, password_hash, role, display_name) VALUES ($1,$2,$3,$4) RETURNING id`,
			req.Username, hash, req.Role, req.DisplayName,
		).Scan(&id)
		if err != nil {
			if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
				http.Error(w, "Bu kullanıcı adı zaten kullanılıyor.", http.StatusConflict)
				return
			}
			http.Error(w, "Yönetici oluşturulamadı.", http.StatusInternalServerError)
			return
		}
		services.Record(db, r, services.AuditLog{Action: services.ActionCreateAdmin, TargetType: "admin_user", TargetID: id, Status: "SUCCESS"})
		writeJSON(w, http.StatusCreated, adminUserDTO{ID: id, Username: req.Username, Role: req.Role, DisplayName: req.DisplayName})
	}
}

type updateAdminRequest struct {
	Role        *string `json:"role"`
	DisplayName *string `json:"display_name"`
	Disabled    *bool   `json:"disabled"`
	Password    *string `json:"password"`
}

// UpdateAdminUser bir yönetici hesabını günceller (admin yetkisi gerekir).
func UpdateAdminUser(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(chi.URLParam(r, "adminID"))
		if err != nil {
			http.Error(w, "Geçersiz ID.", http.StatusBadRequest)
			return
		}
		var req updateAdminRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Geçersiz istek gövdesi.", http.StatusBadRequest)
			return
		}

		ident, _ := services.IdentityFromContext(r.Context())

		// Kendini devre dışı bırakma / son admin'i kilitleme koruması.
		if req.Disabled != nil && *req.Disabled && ident != nil && ident.ID == id {
			http.Error(w, "Kendinizi devre dışı bırakamazsınız.", http.StatusBadRequest)
			return
		}
		if (req.Role != nil && *req.Role != services.RoleAdmin) || (req.Disabled != nil && *req.Disabled) {
			if wouldRemoveLastAdmin(db, id) {
				http.Error(w, "Sistemde en az bir aktif admin kalmalıdır.", http.StatusBadRequest)
				return
			}
		}

		set := []string{}
		args := []interface{}{}
		i := 1
		if req.Role != nil {
			if !services.ValidRole(*req.Role) {
				http.Error(w, "Geçersiz rol.", http.StatusBadRequest)
				return
			}
			set = append(set, "role = $"+strconv.Itoa(i))
			args = append(args, *req.Role)
			i++
		}
		if req.DisplayName != nil {
			set = append(set, "display_name = $"+strconv.Itoa(i))
			args = append(args, *req.DisplayName)
			i++
		}
		if req.Disabled != nil {
			set = append(set, "disabled = $"+strconv.Itoa(i))
			args = append(args, *req.Disabled)
			i++
		}
		if req.Password != nil && *req.Password != "" {
			if len(*req.Password) < 6 {
				http.Error(w, "Parola en az 6 karakter olmalı.", http.StatusBadRequest)
				return
			}
			hash, err := services.HashPassword(*req.Password)
			if err != nil {
				http.Error(w, "Sunucu hatası.", http.StatusInternalServerError)
				return
			}
			set = append(set, "password_hash = $"+strconv.Itoa(i))
			args = append(args, hash)
			i++
		}
		if len(set) == 0 {
			http.Error(w, "Güncellenecek alan yok.", http.StatusBadRequest)
			return
		}
		args = append(args, id)
		if _, err := db.Exec(`UPDATE admin_users SET `+strings.Join(set, ", ")+` WHERE id = $`+strconv.Itoa(i), args...); err != nil {
			http.Error(w, "Güncellenemedi.", http.StatusInternalServerError)
			return
		}
		// Parola değişti ya da hesap devre dışı bırakıldıysa oturumları düşür.
		if (req.Password != nil && *req.Password != "") || (req.Disabled != nil && *req.Disabled) {
			services.InvalidateUserSessions(db, id)
		}
		services.Record(db, r, services.AuditLog{Action: services.ActionUpdateAdmin, TargetType: "admin_user", TargetID: id, Status: "SUCCESS"})
		w.WriteHeader(http.StatusNoContent)
	}
}

// DeleteAdminUser bir yönetici hesabını siler (admin yetkisi gerekir).
func DeleteAdminUser(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(chi.URLParam(r, "adminID"))
		if err != nil {
			http.Error(w, "Geçersiz ID.", http.StatusBadRequest)
			return
		}
		ident, _ := services.IdentityFromContext(r.Context())
		if ident != nil && ident.ID == id {
			http.Error(w, "Kendi hesabınızı silemezsiniz.", http.StatusBadRequest)
			return
		}
		if wouldRemoveLastAdmin(db, id) {
			http.Error(w, "Sistemde en az bir aktif admin kalmalıdır.", http.StatusBadRequest)
			return
		}
		if _, err := db.Exec(`DELETE FROM admin_users WHERE id = $1`, id); err != nil {
			http.Error(w, "Silinemedi.", http.StatusInternalServerError)
			return
		}
		services.Record(db, r, services.AuditLog{Action: services.ActionDeleteAdmin, TargetType: "admin_user", TargetID: id, Status: "SUCCESS"})
		w.WriteHeader(http.StatusNoContent)
	}
}

// wouldRemoveLastAdmin, verilen kullanıcının aktif tek admin olup olmadığını
// döndürür (rol düşürme/silme/devre dışı bırakma öncesi koruma).
func wouldRemoveLastAdmin(db *sql.DB, id int) bool {
	var activeAdmins int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM admin_users WHERE role = $1 AND disabled = false`, services.RoleAdmin,
	).Scan(&activeAdmins); err != nil {
		return false
	}
	if activeAdmins > 1 {
		return false
	}
	// Tek aktif admin var; hedef o mu?
	var role string
	var disabled bool
	if err := db.QueryRow(`SELECT role, disabled FROM admin_users WHERE id = $1`, id).Scan(&role, &disabled); err != nil {
		return false
	}
	return role == services.RoleAdmin && !disabled
}
