package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAdminWSAuth_NoToken, token query parametresi verilmediğinde AdminWSAuth'un
// isteği 401 ile reddettiğini doğrular. Bu yol DB'ye dokunmadan önce döndüğü
// için veritabanı gerektirmez (db = nil ile çalışır).
func TestAdminWSAuth_NoToken(t *testing.T) {
	handler := AdminWSAuth(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("token yokken alt handler çağrılmamalıydı")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/ws/sessions/1", nil)
	rr := executeRequest(req, handler)

	checkResponseCode(t, http.StatusUnauthorized, rr.Code)
}

// TestAdminWSAuth_EmptyToken, ?token= (boş değer) verildiğinde de reddedildiğini
// doğrular. Boş token yine DB'ye dokunmadan 401 üretir.
func TestAdminWSAuth_EmptyToken(t *testing.T) {
	handler := AdminWSAuth(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("boş token ile alt handler çağrılmamalıydı")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/ws/sessions/1?token=", nil)
	rr := executeRequest(req, handler)

	checkResponseCode(t, http.StatusUnauthorized, rr.Code)
}

// TestAdminWSAuth_InvalidToken, geçersiz/bilinmeyen bir token verildiğinde
// isteğin 401 ile reddedildiğini doğrular. Bu yol ValidateSession'ı çağırdığı
// için test veritabanı gereklidir; yoksa requireDB ile atlanır.
func TestAdminWSAuth_InvalidToken(t *testing.T) {
	requireDB(t)

	handler := AdminWSAuth(app.DB)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("geçersiz token ile alt handler çağrılmamalıydı")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/ws/sessions/1?token=gecersiz-token-degeri", nil)
	rr := executeRequest(req, handler)

	checkResponseCode(t, http.StatusUnauthorized, rr.Code)
}
