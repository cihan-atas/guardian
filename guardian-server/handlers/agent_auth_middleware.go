package handlers

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
)

type agentTokenKey string

const AgentTokenContextKey = agentTokenKey("agentToken")

func AgentAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedToken := os.Getenv("GUARDIAN_SECRET_TOKEN")
		if expectedToken == "" {
			http.Error(w, "Sunucu güvenlik yapılandırması eksik (SECRET_TOKEN)", http.StatusInternalServerError)
			return
		}

		var token string

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Yetki bilgisi (token) başlıkta bulunamadı.", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			token = parts[1]
		}

		if token == "" {
			http.Error(w, "Geçersiz token formatı. 'Bearer <token>' bekleniyor.", http.StatusUnauthorized)
			return
		}
		log.Printf("[DEBUG] Server tarafı: Gelen token: '%s', Beklenen: '%s'", token, expectedToken)
		if token != expectedToken {
			http.Error(w, "Geçersiz veya yetkisiz ajan token'ı.", http.StatusForbidden)
			return
		}
		ctx := context.WithValue(r.Context(), AgentTokenContextKey, token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
