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
	TOTPCode string `json:"totp_code"`
}

type loginResponse struct {
	Token       string `json:"token"`
	Username    string `json:"username"`
	Role        string `json:"role"`
	DisplayName string `json:"display_name"`
	TOTPEnabled bool   `json:"totp_enabled"`
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

		token, ident, err := services.Authenticate(db, req.Username, req.Password, req.TOTPCode)
		if err != nil {
			switch {
			case errors.Is(err, services.ErrTOTPRequired):
				// Parola doğru ama 2FA kodu gerekli — UI ikinci adımda kodu ister.
				writeJSON(w, http.StatusOK, map[string]bool{"totp_required": true})
				return
			case errors.Is(err, services.ErrInvalidTOTP):
				http.Error(w, "İki adımlı doğrulama kodu hatalı.", http.StatusUnauthorized)
				return
			case errors.Is(err, services.ErrInvalidCredentials):
				http.Error(w, "Kullanıcı adı veya parola hatalı.", http.StatusUnauthorized)
				return
			default:
				http.Error(w, "Sunucu hatası.", http.StatusInternalServerError)
				return
			}
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
			TOTPEnabled: ident.TOTPEnabled,
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
		// Mevcut parolayı doğrula (2FA kontrolünü atlamak için doğrudan parola kontrolü).
		if !services.VerifyPassword(db, ident.ID, body.CurrentPassword) {
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

// Setup2FA, giriş yapmış kullanıcı için yeni bir TOTP anahtarı üretir ve
// QR/manuel giriş için gizli anahtar + otpauth URI döner (henüz etkin değil;
// kullanıcı bir kod doğrulayınca Enable2FA ile etkinleşir).
func Setup2FA(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ident, ok := services.IdentityFromContext(r.Context())
		if !ok || ident == nil {
			http.Error(w, "Yetkilendirme bulunamadı.", http.StatusUnauthorized)
			return
		}
		secret, uri, err := services.SetupTOTP(db, ident.ID)
		if err != nil {
			http.Error(w, "2FA kurulumu başlatılamadı.", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"secret": secret, "otpauth_uri": uri})
	}
}

// Enable2FA, kurulum sırasında üretilen anahtara karşı bir kod doğrular ve
// 2FA'yı etkinleştirir.
func Enable2FA(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ident, ok := services.IdentityFromContext(r.Context())
		if !ok || ident == nil {
			http.Error(w, "Yetkilendirme bulunamadı.", http.StatusUnauthorized)
			return
		}
		var body struct {
			Code string `json:"code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Geçersiz istek gövdesi.", http.StatusBadRequest)
			return
		}
		if err := services.EnableTOTP(db, ident.ID, body.Code); err != nil {
			if errors.Is(err, services.ErrInvalidTOTP) {
				http.Error(w, "Doğrulama kodu hatalı.", http.StatusBadRequest)
				return
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		services.Record(db, r, services.AuditLog{
			Action:     "ENABLE_2FA",
			TargetType: "admin_user",
			TargetID:   ident.ID,
			Status:     "SUCCESS",
		})
		w.WriteHeader(http.StatusNoContent)
	}
}

// Disable2FA, mevcut parolayı doğrulayarak 2FA'yı kapatır.
func Disable2FA(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ident, ok := services.IdentityFromContext(r.Context())
		if !ok || ident == nil {
			http.Error(w, "Yetkilendirme bulunamadı.", http.StatusUnauthorized)
			return
		}
		var body struct {
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Geçersiz istek gövdesi.", http.StatusBadRequest)
			return
		}
		if !services.VerifyPassword(db, ident.ID, body.Password) {
			http.Error(w, "Parola hatalı.", http.StatusUnauthorized)
			return
		}
		if err := services.DisableTOTP(db, ident.ID); err != nil {
			http.Error(w, "2FA kapatılamadı.", http.StatusInternalServerError)
			return
		}
		services.Record(db, r, services.AuditLog{
			Action:     "DISABLE_2FA",
			TargetType: "admin_user",
			TargetID:   ident.ID,
			Status:     "SUCCESS",
		})
		w.WriteHeader(http.StatusNoContent)
	}
}
