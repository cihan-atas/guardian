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

func TestListAdminUsers_ParsesData(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id":1,"username":"admin","role":"admin","display_name":"Yönetici","disabled":false}]`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "tok")
	admins, err := c.ListAdminUsers()
	if err != nil {
		t.Fatalf("ListAdminUsers hata: %v", err)
	}
	if len(admins) != 1 || admins[0].Username != "admin" || admins[0].Role != "admin" {
		t.Fatalf("yönetici verisi yanlış ayrıştırıldı: %+v", admins)
	}
	if gotPath != "/admin-users" {
		t.Fatalf("beklenen /admin-users, alınan: %s", gotPath)
	}
}

func TestBanKey_SendsPayloadAndParses201(t *testing.T) {
	var gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":5,"public_key_id":3,"reason":"test","banned_until":"2030-01-01T00:00:00Z"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "tok")
	ban, err := c.BanKey(3, 90, "test")
	if err != nil {
		t.Fatalf("BanKey hata: %v", err)
	}
	if ban.PublicKeyID != 3 {
		t.Fatalf("ban verisi yanlış ayrıştırıldı: %+v", ban)
	}
	if gotPath != "/keys/3/ban" {
		t.Fatalf("beklenen /keys/3/ban, alınan: %s", gotPath)
	}
	if !strings.Contains(gotBody, `"duration_minutes":90`) || !strings.Contains(gotBody, `"reason":"test"`) {
		t.Fatalf("istek gövdesi süre/gerekçe içermeli, alınan: %s", gotBody)
	}
}

func TestApproveAccessRequest_204IsSuccess(t *testing.T) {
	var gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "tok")
	if err := c.ApproveAccessRequest(12); err != nil {
		t.Fatalf("204'te başarı bekleniyordu, hata: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/access-requests/12/approve" {
		t.Fatalf("beklenen POST /access-requests/12/approve, alınan: %s %s", gotMethod, gotPath)
	}
}

func TestGetDashboardStats_ParsesData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"active_sessions":2,"total_servers":5,"banned_keys":1}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "tok")
	stats, err := c.GetDashboardStats()
	if err != nil {
		t.Fatalf("GetDashboardStats hata: %v", err)
	}
	if stats.ActiveSessions != 2 || stats.TotalServers != 5 || stats.BannedKeys != 1 {
		t.Fatalf("istatistik verisi yanlış ayrıştırıldı: %+v", stats)
	}
}

func TestSearchCommands_EncodesQuery(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"session_id":1,"command":"rm -rf /","username":"root","server_hostname":"web-1"}]`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "tok")
	matches, err := c.SearchCommands("rm -rf", 50)
	if err != nil {
		t.Fatalf("SearchCommands hata: %v", err)
	}
	if len(matches) != 1 || matches[0].SessionID != 1 {
		t.Fatalf("eşleşme verisi yanlış ayrıştırıldı: %+v", matches)
	}
	if !strings.Contains(gotQuery, "q=rm+-rf") && !strings.Contains(gotQuery, "q=rm%20-rf") {
		t.Fatalf("sorgu kodlanmalı, alınan: %s", gotQuery)
	}
	if !strings.Contains(gotQuery, "limit=50") {
		t.Fatalf("limit parametresi eksik, alınan: %s", gotQuery)
	}
}

func TestUpdateSettings_PutsFullObject(t *testing.T) {
	var gotMethod, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "tok")
	err := c.UpdateSettings(NotificationSettings{WebhookURL: "https://hook", RetentionDays: 30})
	if err != nil {
		t.Fatalf("UpdateSettings hata: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Fatalf("PUT bekleniyordu, alınan: %s", gotMethod)
	}
	if !strings.Contains(gotBody, `"webhook_url":"https://hook"`) || !strings.Contains(gotBody, `"retention_days":30`) {
		t.Fatalf("istek gövdesi ayar alanlarını içermeli, alınan: %s", gotBody)
	}
}

func TestGenerateEnrollToken_ParsesData(t *testing.T) {
	var gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"token":"tok123","server_id":4,"server_hostname":"db-1","install_command":"curl ... | bash","binary_available":true}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "tok")
	resp, err := c.GenerateEnrollToken(4, 730, "linux")
	if err != nil {
		t.Fatalf("GenerateEnrollToken hata: %v", err)
	}
	if resp.Token != "tok123" || resp.InstallCommand == "" || !resp.BinaryAvailable {
		t.Fatalf("enroll yanıtı yanlış ayrıştırıldı: %+v", resp)
	}
	if gotPath != "/servers/4/enroll-token" {
		t.Fatalf("beklenen /servers/4/enroll-token, alınan: %s", gotPath)
	}
	if !strings.Contains(gotBody, `"validity_days":730`) || !strings.Contains(gotBody, `"os":"linux"`) {
		t.Fatalf("istek gövdesi os/gün içermeli, alınan: %s", gotBody)
	}
}

func TestExportSessionAsciicast_ReturnsBytes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"version":2}` + "\n"))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "tok")
	data, err := c.ExportSessionAsciicast(9)
	if err != nil {
		t.Fatalf("ExportSessionAsciicast hata: %v", err)
	}
	if !strings.Contains(string(data), `"version":2`) {
		t.Fatalf("cast içeriği bekleniyordu, alınan: %s", string(data))
	}
}
