package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"guardian.com/server/services"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token       string `json:"token"`
	Username    string `json:"username"`
	Role        string `json:"role"`
	DisplayName string `json:"display_name"`
}

// Login kullanıcı adı + parola doğrular ve oturum token'ı döndürür.
func Login(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Geçersiz istek gövdesi.", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.Username) == "" || req.Password == "" {
			http.Error(w, "Kullanıcı adı ve parola gereklidir.", http.StatusBadRequest)
			return
		}

		token, ident, err := services.Authenticate(db, req.Username, req.Password)
		if err != nil {
			if errors.Is(err, services.ErrInvalidCredentials) {
				http.Error(w, "Kullanıcı adı veya parola hatalı.", http.StatusUnauthorized)
				return
			}
			http.Error(w, "Sunucu hatası.", http.StatusInternalServerError)
			return
		}

		services.Record(db, r.WithContext(withIdentity(r, ident)), services.AuditLog{
			Action:     services.ActionLogin,
			TargetType: "admin_user",
			TargetID:   ident.ID,
			Status:     "SUCCESS",
		})

		writeJSON(w, http.StatusOK, loginResponse{
			Token:       token,
			Username:    ident.Username,
			Role:        ident.Role,
			DisplayName: ident.DisplayName,
		})
	}
}

// Logout mevcut oturum token'ını geçersiz kılar.
func Logout(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if token := bearerToken(r); token != "" {
			services.InvalidateSession(db, token)
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// Me context'teki kimliği döndürür (eski /auth/check yerine).
func Me() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ident, ok := services.IdentityFromContext(r.Context())
		if !ok || ident == nil {
			http.Error(w, "Yetkilendirme bulunamadı.", http.StatusUnauthorized)
			return
		}
		writeJSON(w, http.StatusOK, ident)
	}
}

// ChangeOwnPassword giriş yapmış kullanıcının kendi parolasını değiştirir.
func ChangeOwnPassword(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ident, ok := services.IdentityFromContext(r.Context())
		if !ok || ident == nil {
			http.Error(w, "Yetkilendirme bulunamadı.", http.StatusUnauthorized)
			return
		}
		var body struct {
			CurrentPassword string `json:"current_password"`
			NewPassword     string `json:"new_password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Geçersiz istek gövdesi.", http.StatusBadRequest)
			return
		}
		if len(body.NewPassword) < 6 {
			http.Error(w, "Yeni parola en az 6 karakter olmalıdır.", http.StatusBadRequest)
			return
		}
		// Mevcut parolayı doğrula.
		if _, _, err := services.Authenticate(db, ident.Username, body.CurrentPassword); err != nil {
			http.Error(w, "Mevcut parola hatalı.", http.StatusUnauthorized)
			return
		}
		hash, err := services.HashPassword(body.NewPassword)
		if err != nil {
			http.Error(w, "Sunucu hatası.", http.StatusInternalServerError)
			return
		}
		if _, err := db.Exec(`UPDATE admin_users SET password_hash = $1 WHERE id = $2`, hash, ident.ID); err != nil {
			http.Error(w, "Parola güncellenemedi.", http.StatusInternalServerError)
			return
		}
		// Güvenlik için diğer oturumları düşür (mevcut token da düşer; UI logout eder).
		services.InvalidateUserSessions(db, ident.ID)
		w.WriteHeader(http.StatusNoContent)
	}
}
