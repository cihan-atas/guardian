package handlers

import (
	"context"
	"database/sql"
	"net/http"

	"guardian.com/server/services"
)

// AdminWSAuth, canlı oturum izleme WebSocket bağlantıları için kimlik
// doğrulamasıdır. Tarayıcılar WebSocket handshake'inde özel header
// gönderemediğinden, oturum token'ı burada istisnai olarak URL query
// parametresinden (?token=...) okunur ve admin_sessions'ta doğrulanır.
// Diğer tüm admin endpoint'leri AdminAuth (Authorization header) kullanır.
func AdminWSAuth(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.URL.Query().Get("token")
			if token == "" {
				http.Error(w, "Yetki bilgisi (token) URL parametresinde bulunamadı.", http.StatusUnauthorized)
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
