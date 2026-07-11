package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"guardian.com/server/services"
)

// writeJSON verilen değeri JSON olarak yazar.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// withIdentity, audit kaydında doğru kimliğin görünmesi için verilen kimliği
// request context'ine yerleştiren bir context döndürür.
func withIdentity(r *http.Request, ident *services.AdminIdentity) context.Context {
	return context.WithValue(r.Context(), services.AdminIdentityContextKey, ident)
}
