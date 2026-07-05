package handlers

import (
	"context"
	"crypto/subtle"
	"net/http"
	"os"
	"strings"
)

type adminTokenKey string

const AdminTokenContextKey = adminTokenKey("adminToken")

func AdminAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedToken := os.Getenv("GUARDIAN_ADMIN_TOKEN")
		if expectedToken == "" {
			http.Error(w, "Sunucu güvenlik yapılandırması eksik (ADMIN_TOKEN)", http.StatusInternalServerError)
			return
		}

		var token string
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
				token = parts[1]
			}
		}

		if token == "" {
			http.Error(w, "Yetki bilgisi (token) bulunamadı ('Authorization: Bearer <token>' başlığı bekleniyor).", http.StatusUnauthorized)
			return
		}

		if subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
			http.Error(w, "Geçersiz veya yetkisiz token.", http.StatusForbidden)
			return
		}

		ctx := context.WithValue(r.Context(), AdminTokenContextKey, token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
