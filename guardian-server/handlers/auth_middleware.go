package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"guardian.com/server/services"
)

// AdminAuth, Authorization: Bearer <oturum-token> başlığını doğrular. Token
// admin_sessions tablosunda aranır; geçerliyse yönetici kimliği request
// context'ine eklenir. (Eski statik GUARDIAN_ADMIN_TOKEN kaldırıldı.)
func AdminAuth(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearerToken(r)
			if token == "" {
				http.Error(w, "Yetki bilgisi (token) bulunamadı.", http.StatusUnauthorized)
				return
			}

			ident, err := services.ValidateSession(db, token)
			if err != nil {
				http.Error(w, "Oturum geçersiz veya süresi dolmuş.", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), services.AdminIdentityContextKey, ident)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole, context'teki kimliğin en az verilen role sahip olmasını şart
// koşar. AdminAuth'tan sonra kullanılmalıdır.
func RequireRole(minRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ident, ok := services.IdentityFromContext(r.Context())
			if !ok || ident == nil {
				http.Error(w, "Yetkilendirme bulunamadı.", http.StatusUnauthorized)
				return
			}
			if !services.RoleAtLeast(ident.Role, minRole) {
				http.Error(w, "Bu işlem için yetkiniz yok.", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func bearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}
	parts := strings.Split(authHeader, " ")
	if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
		return strings.TrimSpace(parts[1])
	}
	return ""
}
