package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"guardian.com/server/services"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestBearerToken(t *testing.T) {
	cases := []struct {
		header string
		want   string
	}{
		{"", ""},
		{"Bearer abc123", "abc123"},
		{"bearer xyz", "xyz"},         // büyük/küçük harf duyarsız
		{"Bearer   spaced", ""},       // çoklu boşluk → katı ayrıştırma reddeder
		{"Basic abc", ""},             // yanlış şema
		{"abc123", ""},                // şemasız
	}
	for _, c := range cases {
		req := httptest.NewRequest("GET", "/", nil)
		if c.header != "" {
			req.Header.Set("Authorization", c.header)
		}
		if got := bearerToken(req); got != c.want {
			t.Errorf("bearerToken(%q) = %q, beklenen %q", c.header, got, c.want)
		}
	}
}

func TestAdminAuth_NoTokenIsUnauthorized(t *testing.T) {
	// Token yoksa DB'ye hiç gidilmeden 401 dönmeli (db nil güvenli).
	h := AdminAuth(nil)(okHandler())
	req := httptest.NewRequest("GET", "/api/servers", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("token'sız istekte 401 bekleniyordu, alınan: %d", rr.Code)
	}
}

func reqWithRole(req *http.Request, role string) *http.Request {
	ident := &services.AdminIdentity{ID: 1, Username: "u", Role: role}
	return req.WithContext(withIdentity(req, ident))
}

func TestRequireRole_NoIdentity(t *testing.T) {
	h := RequireRole(services.RoleViewer)(okHandler())
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("kimliksiz istekte 401 bekleniyordu, alınan: %d", rr.Code)
	}
}

func TestRequireRole_InsufficientRole(t *testing.T) {
	// viewer, admin gerektiren uca erişemez → 403.
	h := RequireRole(services.RoleAdmin)(okHandler())
	req := reqWithRole(httptest.NewRequest("GET", "/", nil), services.RoleViewer)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("yetersiz rolde 403 bekleniyordu, alınan: %d", rr.Code)
	}
}

func TestRequireRole_SufficientRole(t *testing.T) {
	// admin, operator gerektiren uca erişebilir → 200.
	h := RequireRole(services.RoleOperator)(okHandler())
	req := reqWithRole(httptest.NewRequest("GET", "/", nil), services.RoleAdmin)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("yeterli rolde 200 bekleniyordu, alınan: %d", rr.Code)
	}
}
