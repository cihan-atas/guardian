package client

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestClient, plain-HTTP httptest sunucusuna bağlanan bir Client döndürür.
// New()'in CA dosyası gereksinimini atlamak için alanlar doğrudan doldurulur
// (test aynı pakette).
func newTestClient(baseURL, token string) *Client {
	return &Client{
		httpClient: http.DefaultClient,
		baseURL:    baseURL,
		adminToken: token,
	}
}

func TestLogin_StoresToken(t *testing.T) {
	var gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"token": "oturum-token-123"})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "")
	if err := c.Login("admin", "sifre"); err != nil {
		t.Fatalf("Login hata döndürdü: %v", err)
	}
	if c.adminToken != "oturum-token-123" {
		t.Fatalf("token saklanmadı, alınan: %q", c.adminToken)
	}
	if gotPath != "/auth/login" {
		t.Fatalf("beklenen path /auth/login, alınan: %s", gotPath)
	}
	if !strings.Contains(gotBody, `"username":"admin"`) || !strings.Contains(gotBody, `"password":"sifre"`) {
		t.Fatalf("istek gövdesi kullanıcı adı/parola içermeli, alınan: %s", gotBody)
	}
}

func TestLogin_EmptyTokenIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"token": ""})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "")
	if err := c.Login("admin", "sifre"); err == nil {
		t.Fatal("boş token'da hata bekleniyordu")
	}
}

func TestLogin_HTTPErrorIsReported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "geçersiz kimlik", http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "")
	if err := c.Login("admin", "yanlis"); err == nil {
		t.Fatal("401 yanıtında hata bekleniyordu")
	}
}

func TestSendRequest_SetsBearerAuth(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "tok-abc")
	resp, err := c.sendRequest("GET", "/servers", nil)
	if err != nil {
		t.Fatalf("sendRequest hata: %v", err)
	}
	resp.Body.Close()
	if gotAuth != "Bearer tok-abc" {
		t.Fatalf("beklenen 'Bearer tok-abc', alınan: %q", gotAuth)
	}
}

func TestListServers_ParsesPaginatedData(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"id":1,"hostname":"web-1","ip_address":"10.0.0.1"},{"id":2,"hostname":"web-2","ip_address":"10.0.0.2"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "tok")
	servers, err := c.ListServers()
	if err != nil {
		t.Fatalf("ListServers hata: %v", err)
	}
	if len(servers) != 2 {
		t.Fatalf("2 sunucu bekleniyordu, alınan: %d", len(servers))
	}
	if servers[0].Hostname != "web-1" || servers[1].IPAddress != "10.0.0.2" {
		t.Fatalf("sunucu verisi yanlış ayrıştırıldı: %+v", servers)
	}
	if gotPath != "/servers" || !strings.Contains(gotQuery, "limit=1000") {
		t.Fatalf("beklenen GET /servers?limit=1000, alınan: %s?%s", gotPath, gotQuery)
	}
}

func TestCreateServer_ErrorOnNon201(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "çakışma", http.StatusConflict)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "tok")
	if _, err := c.CreateServer(CreateServerPayload{Hostname: "x", IPAddress: "1.2.3.4"}); err == nil {
		t.Fatal("201 dışı yanıtta hata bekleniyordu")
	}
}

func TestDeleteServer_204IsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("DELETE bekleniyordu, alınan: %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "tok")
	if err := c.DeleteServer(7); err != nil {
		t.Fatalf("204'te başarı bekleniyordu, hata: %v", err)
	}
}
