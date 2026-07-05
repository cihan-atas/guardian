package handlers

import (
	"crypto/subtle"
	"net/http"
	"os"
)

// AdminWSAuth, canlı oturum izleme WebSocket bağlantıları için kullanılan admin
// kimlik doğrulamasıdır. Tarayıcılar WebSocket handshake'inde özel header
// gönderemediğinden, token burada istisnai olarak URL query parametresinden
// (?token=...) okunur. Diğer tüm admin endpoint'leri AdminAuth middleware'ini
// (yalnızca Authorization header) kullanmalıdır.
func AdminWSAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedToken := os.Getenv("GUARDIAN_ADMIN_TOKEN")
		if expectedToken == "" {
			http.Error(w, "Sunucu güvenlik yapılandırması eksik (ADMIN_TOKEN)", http.StatusInternalServerError)
			return
		}

		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "Yetki bilgisi (token) URL parametresinde bulunamadı.", http.StatusUnauthorized)
			return
		}

		if subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
			http.Error(w, "Geçersiz veya yetkisiz token.", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
