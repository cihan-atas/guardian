package handlers

import (
	"context"
	"crypto/subtle"
	"net/http"
	"os"
	"strings"
)

type agentTokenKey string

const AgentTokenContextKey = agentTokenKey("agentToken")

// AgentAuth, ajan→sunucu isteklerini doğrular. Öncelikli yöntem mTLS'dir:
// ajan, TLS el sıkışmasında CA tarafından imzalı bir istemci sertifikası
// sunarsa (sunucunun TLS katmanı zinciri zaten doğrulamıştır) istek kabul
// edilir. mTLS yoksa, eski kurulumlarla uyumluluk için paylaşımlı
// GUARDIAN_SECRET_TOKEN yedeğine düşülür.
func AgentAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1) mTLS: geçerli (CA imzalı) istemci sertifikası sunulmuşsa kabul et.
		if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
			cn := r.TLS.PeerCertificates[0].Subject.CommonName
			ctx := context.WithValue(r.Context(), AgentTokenContextKey, "mtls:"+cn)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// 2) Yedek: paylaşımlı token (eski ajanlar). Token yapılandırılmamışsa
		// yalnızca mTLS kabul edildiği için istek reddedilir.
		expectedToken := os.Getenv("GUARDIAN_SECRET_TOKEN")
		if expectedToken == "" {
			http.Error(w, "Ajan kimlik doğrulaması başarısız: istemci sertifikası (mTLS) gerekli.", http.StatusUnauthorized)
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
		if subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
			http.Error(w, "Geçersiz veya yetkisiz ajan token'ı.", http.StatusForbidden)
			return
		}
		ctx := context.WithValue(r.Context(), AgentTokenContextKey, token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
