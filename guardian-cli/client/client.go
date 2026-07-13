package client

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	adminToken string
}

func New(baseURL, caCertFile string) (*Client, error) {
	caCert, err := os.ReadFile(caCertFile)
	if err != nil {
		return nil, fmt.Errorf("CA sertifikası okunamadı (%s): %w", caCertFile, err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig := &tls.Config{
		RootCAs: caCertPool,
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	return &Client{
		httpClient: &http.Client{
			Timeout:   15 * time.Second,
			Transport: transport,
		},
		baseURL: baseURL,
	}, nil
}

// Login, kullanıcı adı + parola ile giriş yapar ve dönen oturum token'ını
// istemciye kaydeder. Statik admin token kaldırıldığından tüm yönetim
// istekleri bu oturum token'ıyla yapılır.
func (c *Client) Login(username, password string) error {
	payload := map[string]string{"username": username, "password": password}
	jsonData, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", c.baseURL+"/auth/login", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("giriş isteği oluşturulamadı: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("giriş isteği gönderilemedi: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("giriş başarısız (%s): %s", resp.Status, string(bodyBytes))
	}

	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("giriş yanıtı okunamadı: %w", err)
	}
	if out.Token == "" {
		return fmt.Errorf("giriş yanıtında token yok")
	}
	c.adminToken = out.Token
	return nil
}

type Server struct {
	ID          int       `json:"id"`
	Hostname    string    `json:"hostname"`
	IPAddress   string    `json:"ip_address"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type NullString struct {
	String string `json:"String"`
	Valid  bool   `json:"Valid"`
}

type User struct {
	ID          int        `json:"id"`
	Username    string     `json:"username"`
	Description NullString `json:"description"`
	CreatedAt   time.Time  `json:"created_at"`
}

type PublicKey struct {
	ID                int       `json:"id"`
	KeyName           string    `json:"key_name"`
	SshPublicKey      string    `json:"ssh_public_key"`
	FingerprintSHA256 string    `json:"fingerprint_sha256"`
	CreatedAt         time.Time `json:"created_at"`
}

type AccessRule struct {
	ID             int       `json:"id"`
	ServerID       int       `json:"server_id"`
	PublicKeyID    int       `json:"public_key_id"`
	SystemUserID   int       `json:"system_user_id"`
	Status         string    `json:"status"`
	ValidUntil     time.Time `json:"valid_until"`
	ServerHostname string    `json:"server_hostname"`
	Username       string    `json:"username"`
	KeyName        string    `json:"key_name"`
}

type Session struct {
	ID        int        `json:"id"`
	RuleID    *int       `json:"rule_id"`
	ServerID  int        `json:"server_id"`
	Username  string     `json:"username"`
	StartTime time.Time  `json:"start_time"`
	EndTime   *time.Time `json:"end_time"`
	Status    string     `json:"status"`
}

type SessionDetails struct {
	SessionInfo struct {
		ID             int        `json:"id"`
		Username       string     `json:"username"`
		ServerHostname string     `json:"server_hostname"`
		ServerIP       string     `json:"ip_address"`
		StartTime      time.Time  `json:"start_time"`
		EndTime        *time.Time `json:"end_time"`
		Status         string     `json:"status"`
	} `json:"session_info"`
	Commands []struct {
		Timestamp time.Time `json:"timestamp"`
		Command   string    `json:"command"`
		Output    string    `json:"output"`
	} `json:"commands"`
}

type PaginatedServersResponse struct {
	TotalRecords int      `json:"total_records"`
	Page         int      `json:"page"`
	Limit        int      `json:"limit"`
	Data         []Server `json:"data"`
}

type PaginatedUsersResponse struct {
	TotalRecords int    `json:"total_records"`
	Page         int    `json:"page"`
	Limit        int    `json:"limit"`
	Data         []User `json:"data"`
}

type PaginatedKeysResponse struct {
	TotalRecords int         `json:"total_records"`
	Page         int         `json:"page"`
	Limit        int         `json:"limit"`
	Data         []PublicKey `json:"data"`
}

type PaginatedRulesResponse struct {
	TotalRecords int          `json:"total_records"`
	Page         int          `json:"page"`
	Limit        int          `json:"limit"`
	Data         []AccessRule `json:"data"`
}

type PaginatedSessionsResponse struct {
	TotalRecords int       `json:"total_records"`
	Page         int       `json:"page"`
	Limit        int       `json:"limit"`
	Data         []Session `json:"data"`
}

type CreateServerPayload struct {
	Hostname    string `json:"hostname"`
	IPAddress   string `json:"ip_address"`
	Description string `json:"description"`
}

type CreateUserPayload struct {
	Username    string     `json:"username"`
	Description NullString `json:"description"`
}

type CreateKeyPayload struct {
	KeyName      string `json:"key_name"`
	SshPublicKey string `json:"ssh_public_key"`
}

type CreateRulePayload struct {
	ServerID     int       `json:"server_id"`
	PublicKeyID  int       `json:"public_key_id"`
	SystemUserID int       `json:"system_user_id"`
	ValidFrom    time.Time `json:"valid_from"`
	ValidUntil   time.Time `json:"valid_until"`
}

type UpdateUserPayload struct {
	Description NullString `json:"description"`
}

type UpdateServerPayload struct {
	Hostname    string `json:"hostname,omitempty"`
	IPAddress   string `json:"ip_address,omitempty"`
	Description string `json:"description,omitempty"`
}

type UpdateKeyPayload struct {
	KeyName string `json:"key_name"`
}

type UpdateRulePayload struct {
	ValidFrom  time.Time `json:"valid_from"`
	ValidUntil time.Time `json:"valid_until"`
}

func (c *Client) sendRequest(method, endpoint string, payload interface{}) (*http.Response, error) {
	var body io.Reader
	if payload != nil {
		jsonData, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("payload JSON'a çevrilemedi: %w", err)
		}
		body = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, c.baseURL+endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("istek oluşturulamadı: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.adminToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	return c.httpClient.Do(req)
}

func (c *Client) ListServers() ([]Server, error) {
	resp, err := c.sendRequest("GET", "/servers?limit=1000", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API hatası %s: %s", resp.Status, string(bodyBytes))
	}

	var paginatedResponse PaginatedServersResponse
	if err := json.NewDecoder(resp.Body).Decode(&paginatedResponse); err != nil {
		return nil, fmt.Errorf("API yanıtı okunamadı: %w", err)
	}
	return paginatedResponse.Data, nil
}

func (c *Client) CreateServer(payload CreateServerPayload) (*Server, error) {
	resp, err := c.sendRequest("POST", "/servers", payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API hatası %s: %s", resp.Status, string(bodyBytes))
	}

	var newServer Server
	if err := json.NewDecoder(resp.Body).Decode(&newServer); err != nil {
		return nil, fmt.Errorf("API yanıtı okunamadı: %w", err)
	}
	return &newServer, nil
}

func (c *Client) DeleteServer(serverID int) error {
	endpoint := fmt.Sprintf("/servers/%d", serverID)
	resp, err := c.sendRequest("DELETE", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API hatası %s: %s", resp.Status, string(bodyBytes))
	}
	return nil
}

func (c *Client) UpdateServer(serverID int, payload UpdateServerPayload) (*Server, error) {
	endpoint := fmt.Sprintf("/servers/%d", serverID)
	resp, err := c.sendRequest("PATCH", endpoint, payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API hatası %s: %s", resp.Status, string(bodyBytes))
	}

	var updatedServer Server
	if err := json.NewDecoder(resp.Body).Decode(&updatedServer); err != nil {
		return nil, fmt.Errorf("API yanıtı okunamadı: %w", err)
	}
	return &updatedServer, nil
}

func (c *Client) ListUsers() ([]User, error) {
	resp, err := c.sendRequest("GET", "/users?limit=1000", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API hatası %s: %s", resp.Status, string(bodyBytes))
	}
	var paginatedResponse PaginatedUsersResponse
	if err := json.NewDecoder(resp.Body).Decode(&paginatedResponse); err != nil {
		return nil, fmt.Errorf("API yanıtı okunamadı: %w", err)
	}
	return paginatedResponse.Data, nil
}

func (c *Client) CreateUser(payload CreateUserPayload) (*User, error) {
	resp, err := c.sendRequest("POST", "/users", payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API hatası %s: %s", resp.Status, string(bodyBytes))
	}
	var newUser User
	if err := json.NewDecoder(resp.Body).Decode(&newUser); err != nil {
		return nil, fmt.Errorf("API yanıtı okunamadı: %w", err)
	}
	return &newUser, nil
}

func (c *Client) DeleteUser(userID int) error {
	endpoint := fmt.Sprintf("/users/%d", userID)
	resp, err := c.sendRequest("DELETE", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API hatası %s: %s", resp.Status, string(bodyBytes))
	}
	return nil
}

func (c *Client) UpdateUser(userID int, payload UpdateUserPayload) (*User, error) {
	endpoint := fmt.Sprintf("/users/%d", userID)
	resp, err := c.sendRequest("PATCH", endpoint, payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API hatası %s: %s", resp.Status, string(bodyBytes))
	}

	var updatedUser User
	if err := json.NewDecoder(resp.Body).Decode(&updatedUser); err != nil {
		return nil, fmt.Errorf("API yanıtı okunamadı: %w", err)
	}
	return &updatedUser, nil
}

func (c *Client) ListKeys() ([]PublicKey, error) {
	resp, err := c.sendRequest("GET", "/keys?limit=1000", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API hatası %s: %s", resp.Status, string(bodyBytes))
	}
	var paginatedResponse PaginatedKeysResponse
	if err := json.NewDecoder(resp.Body).Decode(&paginatedResponse); err != nil {
		return nil, fmt.Errorf("API yanıtı okunamadı: %w", err)
	}
	return paginatedResponse.Data, nil
}

func (c *Client) CreateKey(payload CreateKeyPayload) (*PublicKey, error) {
	resp, err := c.sendRequest("POST", "/keys", payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API hatası %s: %s", resp.Status, string(bodyBytes))
	}
	var newKey PublicKey
	if err := json.NewDecoder(resp.Body).Decode(&newKey); err != nil {
		return nil, fmt.Errorf("API yanıtı okunamadı: %w", err)
	}
	return &newKey, nil
}

func (c *Client) DeleteKey(keyID int) error {
	endpoint := fmt.Sprintf("/keys/%d", keyID)
	resp, err := c.sendRequest("DELETE", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API hatası %s: %s", resp.Status, string(bodyBytes))
	}
	return nil
}

func (c *Client) UpdateKey(keyID int, payload UpdateKeyPayload) (*PublicKey, error) {
	endpoint := fmt.Sprintf("/keys/%d", keyID)
	resp, err := c.sendRequest("PATCH", endpoint, payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API hatası %s: %s", resp.Status, string(bodyBytes))
	}

	var updatedKey PublicKey
	if err := json.NewDecoder(resp.Body).Decode(&updatedKey); err != nil {
		return nil, fmt.Errorf("API yanıtı okunamadı: %w", err)
	}
	return &updatedKey, nil
}

func (c *Client) ListRules() ([]AccessRule, error) {
	resp, err := c.sendRequest("GET", "/rules", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API hatası %s: %s", resp.Status, string(bodyBytes))
	}
	var paginatedResponse PaginatedRulesResponse
	if err := json.NewDecoder(resp.Body).Decode(&paginatedResponse); err != nil {
		return nil, fmt.Errorf("API yanıtı okunamadı: %w", err)
	}
	return paginatedResponse.Data, nil
}

func (c *Client) CreateRule(payload CreateRulePayload) (*AccessRule, error) {
	resp, err := c.sendRequest("POST", "/rules", payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API hatası %s: %s", resp.Status, string(bodyBytes))
	}
	var newRule AccessRule
	if err := json.NewDecoder(resp.Body).Decode(&newRule); err != nil {
		return nil, fmt.Errorf("API yanıtı okunamadı: %w", err)
	}
	return &newRule, nil
}

func (c *Client) DeleteRule(ruleID int) error {
	endpoint := fmt.Sprintf("/rules/%d", ruleID)
	resp, err := c.sendRequest("DELETE", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API hatası %s: %s", resp.Status, string(bodyBytes))
	}
	return nil
}

func (c *Client) UpdateRule(ruleID int, payload UpdateRulePayload) (*AccessRule, error) {
	endpoint := fmt.Sprintf("/rules/%d", ruleID)
	resp, err := c.sendRequest("PATCH", endpoint, payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API hatası %s: %s", resp.Status, string(bodyBytes))
	}

	var updatedRule AccessRule
	if err := json.NewDecoder(resp.Body).Decode(&updatedRule); err != nil {
		return nil, fmt.Errorf("API yanıtı okunamadı: %w", err)
	}
	return &updatedRule, nil
}

func (c *Client) ListSessions() ([]Session, error) {
	resp, err := c.sendRequest("GET", "/sessions", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API hatası %s: %s", resp.Status, string(bodyBytes))
	}
	var paginatedResponse PaginatedSessionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&paginatedResponse); err != nil {
		return nil, fmt.Errorf("API yanıtı okunamadı: %w", err)
	}
	return paginatedResponse.Data, nil
}

func (c *Client) GetSessionDetails(sessionID int) (*SessionDetails, error) {
	endpoint := fmt.Sprintf("/sessions/%d/commands", sessionID)
	resp, err := c.sendRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API hatası %s: %s", resp.Status, string(bodyBytes))
	}
	var details SessionDetails
	if err := json.NewDecoder(resp.Body).Decode(&details); err != nil {
		return nil, fmt.Errorf("API yanıtı okunamadı: %w", err)
	}
	return &details, nil
}

func (c *Client) TerminateSession(sessionID int) error {
	endpoint := fmt.Sprintf("/sessions/%d", sessionID)
	resp, err := c.sendRequest("DELETE", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API hatası %s: %s", resp.Status, string(bodyBytes))
	}
	return nil
}

func (c *Client) GetBaseURL() string {
	return c.baseURL
}

// decodeOK, yanıt gövdesini kapatır, durum kodunu doğrular ve (out nil değilse)
// JSON gövdeyi out'a çözer. Yeni metotlardaki tekrarları azaltmak için kullanılır.
func decodeOK(resp *http.Response, wantStatus int, out interface{}) error {
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API hatası %s: %s", resp.Status, string(bodyBytes))
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("API yanıtı okunamadı: %w", err)
		}
	}
	return nil
}

// --- Yönetici hesapları (RBAC) ---

type AdminUser struct {
	ID          int        `json:"id"`
	Username    string     `json:"username"`
	Role        string     `json:"role"`
	DisplayName string     `json:"display_name"`
	Disabled    bool       `json:"disabled"`
	CreatedAt   time.Time  `json:"created_at"`
	LastLogin   *time.Time `json:"last_login"`
}

type CreateAdminPayload struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	Role        string `json:"role"`
	DisplayName string `json:"display_name"`
}

type UpdateAdminPayload struct {
	Role        string `json:"role,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Disabled    *bool  `json:"disabled,omitempty"`
	Password    string `json:"password,omitempty"`
}

func (c *Client) ListAdminUsers() ([]AdminUser, error) {
	resp, err := c.sendRequest("GET", "/admin-users", nil)
	if err != nil {
		return nil, err
	}
	var admins []AdminUser
	if err := decodeOK(resp, http.StatusOK, &admins); err != nil {
		return nil, err
	}
	return admins, nil
}

func (c *Client) CreateAdminUser(payload CreateAdminPayload) (*AdminUser, error) {
	resp, err := c.sendRequest("POST", "/admin-users", payload)
	if err != nil {
		return nil, err
	}
	var admin AdminUser
	if err := decodeOK(resp, http.StatusCreated, &admin); err != nil {
		return nil, err
	}
	return &admin, nil
}

func (c *Client) UpdateAdminUser(adminID int, payload UpdateAdminPayload) error {
	resp, err := c.sendRequest("PATCH", fmt.Sprintf("/admin-users/%d", adminID), payload)
	if err != nil {
		return err
	}
	return decodeOK(resp, http.StatusNoContent, nil)
}

func (c *Client) DeleteAdminUser(adminID int) error {
	resp, err := c.sendRequest("DELETE", fmt.Sprintf("/admin-users/%d", adminID), nil)
	if err != nil {
		return err
	}
	return decodeOK(resp, http.StatusNoContent, nil)
}

// --- Erişim talepleri (onay akışı) ---

type AccessRequest struct {
	ID             int       `json:"id"`
	ServerID       int       `json:"server_id"`
	PublicKeyID    int       `json:"public_key_id"`
	SystemUserID   int       `json:"system_user_id"`
	ValidFrom      time.Time `json:"valid_from"`
	ValidUntil     time.Time `json:"valid_until"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	ServerHostname string    `json:"server_hostname"`
	Username       string    `json:"username"`
	KeyName        string    `json:"key_name"`
	RequestReason  string    `json:"request_reason"`
	RejectReason   string    `json:"reject_reason"`
	RequestedBy    string    `json:"requested_by"`
	ApprovedBy     string    `json:"approved_by"`
}

func (c *Client) ListAccessRequests(status string) ([]AccessRequest, error) {
	endpoint := "/access-requests"
	if status != "" {
		endpoint += "?status=" + url.QueryEscape(status)
	}
	resp, err := c.sendRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	var reqs []AccessRequest
	if err := decodeOK(resp, http.StatusOK, &reqs); err != nil {
		return nil, err
	}
	return reqs, nil
}

func (c *Client) ApproveAccessRequest(requestID int) error {
	resp, err := c.sendRequest("POST", fmt.Sprintf("/access-requests/%d/approve", requestID), map[string]string{})
	if err != nil {
		return err
	}
	return decodeOK(resp, http.StatusNoContent, nil)
}

func (c *Client) RejectAccessRequest(requestID int, reason string) error {
	resp, err := c.sendRequest("POST", fmt.Sprintf("/access-requests/%d/reject", requestID), map[string]string{"reason": reason})
	if err != nil {
		return err
	}
	return decodeOK(resp, http.StatusNoContent, nil)
}

// --- Anahtar yasaklama ---

type KeyBan struct {
	ID          int       `json:"id"`
	PublicKeyID int       `json:"public_key_id"`
	Reason      string    `json:"reason"`
	BannedAt    time.Time `json:"banned_at"`
	BannedUntil time.Time `json:"banned_until"`
}

type KeyBanStatus struct {
	Banned bool    `json:"banned"`
	Ban    *KeyBan `json:"ban"`
}

func (c *Client) GetKeyBanStatus(keyID int) (*KeyBanStatus, error) {
	resp, err := c.sendRequest("GET", fmt.Sprintf("/keys/%d/ban", keyID), nil)
	if err != nil {
		return nil, err
	}
	var status KeyBanStatus
	if err := decodeOK(resp, http.StatusOK, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

func (c *Client) BanKey(keyID, durationMinutes int, reason string) (*KeyBan, error) {
	payload := map[string]interface{}{"duration_minutes": durationMinutes, "reason": reason}
	resp, err := c.sendRequest("POST", fmt.Sprintf("/keys/%d/ban", keyID), payload)
	if err != nil {
		return nil, err
	}
	var ban KeyBan
	if err := decodeOK(resp, http.StatusCreated, &ban); err != nil {
		return nil, err
	}
	return &ban, nil
}

func (c *Client) UnbanKey(keyID int) error {
	resp, err := c.sendRequest("DELETE", fmt.Sprintf("/keys/%d/ban", keyID), nil)
	if err != nil {
		return err
	}
	return decodeOK(resp, http.StatusNoContent, nil)
}

// --- Gösterge paneli ---

type DashboardStats struct {
	ActiveSessions int `json:"active_sessions"`
	ExpiredRules   int `json:"expired_rules"`
	TotalServers   int `json:"total_servers"`
	TotalUsers     int `json:"total_users"`
	PendingRules   int `json:"pending_rules"`
	TotalKeys      int `json:"total_keys"`
	TodaySessions  int `json:"today_sessions"`
	FailedSessions int `json:"failed_sessions"`
	BannedKeys     int `json:"banned_keys"`
}

func (c *Client) GetDashboardStats() (*DashboardStats, error) {
	resp, err := c.sendRequest("GET", "/dashboard/stats", nil)
	if err != nil {
		return nil, err
	}
	var stats DashboardStats
	if err := decodeOK(resp, http.StatusOK, &stats); err != nil {
		return nil, err
	}
	return &stats, nil
}

type Alert struct {
	ID             int       `json:"id"`
	SessionID      int       `json:"session_id"`
	Severity       string    `json:"severity"`
	RuleName       string    `json:"rule_name"`
	Command        string    `json:"command"`
	ActionTaken    string    `json:"action_taken"`
	CreatedAt      time.Time `json:"created_at"`
	Username       string    `json:"username"`
	ServerHostname string    `json:"server_hostname"`
}

func (c *Client) GetAlerts(limit int) ([]Alert, error) {
	resp, err := c.sendRequest("GET", fmt.Sprintf("/dashboard/alerts?limit=%d", limit), nil)
	if err != nil {
		return nil, err
	}
	var alerts []Alert
	if err := decodeOK(resp, http.StatusOK, &alerts); err != nil {
		return nil, err
	}
	return alerts, nil
}

// --- Bildirim/ayarlar ---

type NotificationSettings struct {
	WebhookURL           string `json:"webhook_url"`
	SMTPHost             string `json:"smtp_host"`
	SMTPPort             string `json:"smtp_port"`
	SMTPUser             string `json:"smtp_user"`
	SMTPPass             string `json:"smtp_pass,omitempty"`
	SMTPPassSet          bool   `json:"smtp_pass_set,omitempty"`
	SMTPFrom             string `json:"smtp_from"`
	AlertEmailTo         string `json:"alert_email_to"`
	RiskyAutoaction      string `json:"risky_autoaction"`
	RetentionDays        int    `json:"retention_days"`
	RetentionLastRun     string `json:"retention_last_run,omitempty"`
	RetentionLastDeleted int    `json:"retention_last_deleted,omitempty"`
}

func (c *Client) GetSettings() (*NotificationSettings, error) {
	resp, err := c.sendRequest("GET", "/settings", nil)
	if err != nil {
		return nil, err
	}
	var settings NotificationSettings
	if err := decodeOK(resp, http.StatusOK, &settings); err != nil {
		return nil, err
	}
	return &settings, nil
}

func (c *Client) UpdateSettings(payload NotificationSettings) error {
	resp, err := c.sendRequest("PUT", "/settings", payload)
	if err != nil {
		return err
	}
	return decodeOK(resp, http.StatusOK, nil)
}

func (c *Client) TestNotification() error {
	resp, err := c.sendRequest("POST", "/settings/test", map[string]string{})
	if err != nil {
		return err
	}
	return decodeOK(resp, http.StatusOK, nil)
}

// --- Denetim kaydı ---

type AuditLog struct {
	ID           int       `json:"id"`
	AdminRef     string    `json:"admin_ref"`
	Action       string    `json:"action"`
	TargetType   string    `json:"target_type"`
	TargetID     *int      `json:"target_id"`
	Status       string    `json:"status"`
	ErrorMessage string    `json:"error_message"`
	CreatedAt    time.Time `json:"created_at"`
}

type PaginatedAuditLogsResponse struct {
	TotalRecords int        `json:"total_records"`
	Page         int        `json:"page"`
	Limit        int        `json:"limit"`
	Data         []AuditLog `json:"data"`
}

func (c *Client) ListAuditLogs(page, limit int, search, action, status string) ([]AuditLog, error) {
	q := url.Values{}
	q.Set("page", strconv.Itoa(page))
	q.Set("limit", strconv.Itoa(limit))
	if search != "" {
		q.Set("search", search)
	}
	if action != "" {
		q.Set("action", action)
	}
	if status != "" {
		q.Set("status", status)
	}
	resp, err := c.sendRequest("GET", "/audit-logs?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var out PaginatedAuditLogsResponse
	if err := decodeOK(resp, http.StatusOK, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// --- Global komut arama ---

type CommandMatch struct {
	SessionID      int       `json:"session_id"`
	CommandIndex   int       `json:"command_index"`
	Command        string    `json:"command"`
	Username       string    `json:"username"`
	ServerHostname string    `json:"server_hostname"`
	StartTime      time.Time `json:"start_time"`
	Status         string    `json:"status"`
}

func (c *Client) SearchCommands(query string, limit int) ([]CommandMatch, error) {
	endpoint := fmt.Sprintf("/commands/search?q=%s&limit=%d", url.QueryEscape(query), limit)
	resp, err := c.sendRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	var matches []CommandMatch
	if err := decodeOK(resp, http.StatusOK, &matches); err != nil {
		return nil, err
	}
	return matches, nil
}

// --- Sertifikalar ---

type CertInfo struct {
	Subject   string `json:"subject"`
	Issuer    string `json:"issuer"`
	NotBefore string `json:"not_before"`
	NotAfter  string `json:"not_after"`
	DaysLeft  int    `json:"days_left"`
}

type AgentCertInfo struct {
	ServerID  int       `json:"server_id"`
	Hostname  string    `json:"hostname"`
	IPAddress string    `json:"ip_address"`
	Online    bool      `json:"online"`
	Cert      *CertInfo `json:"cert"`
}

type CertificatesResponse struct {
	CA          *CertInfo       `json:"ca"`
	CAError     string          `json:"ca_error"`
	Server      *CertInfo       `json:"server"`
	ServerError string          `json:"server_error"`
	Agents      []AgentCertInfo `json:"agents"`
}

func (c *Client) GetCertificates() (*CertificatesResponse, error) {
	resp, err := c.sendRequest("GET", "/certificates", nil)
	if err != nil {
		return nil, err
	}
	var certs CertificatesResponse
	if err := decodeOK(resp, http.StatusOK, &certs); err != nil {
		return nil, err
	}
	return &certs, nil
}

func (c *Client) RenewServerCert(validityDays int) (*CertInfo, bool, error) {
	payload := map[string]int{"validity_days": validityDays}
	resp, err := c.sendRequest("POST", "/certificates/server/renew", payload)
	if err != nil {
		return nil, false, err
	}
	var out struct {
		Cert            CertInfo `json:"cert"`
		RestartRequired bool     `json:"restart_required"`
	}
	if err := decodeOK(resp, http.StatusOK, &out); err != nil {
		return nil, false, err
	}
	return &out.Cert, out.RestartRequired, nil
}

// --- Sunucu sağlık durumu ---

type ServerHealth struct {
	ServerID  int     `json:"server_id"`
	Hostname  string  `json:"hostname"`
	IPAddress string  `json:"ip_address"`
	Online    bool    `json:"online"`
	LatencyMS float64 `json:"latency_ms"`
}

func (c *Client) GetServersHealth() ([]ServerHealth, error) {
	resp, err := c.sendRequest("GET", "/servers/health", nil)
	if err != nil {
		return nil, err
	}
	var health []ServerHealth
	if err := decodeOK(resp, http.StatusOK, &health); err != nil {
		return nil, err
	}
	return health, nil
}

// --- Agent kaydı (enroll) ---

type EnrollTokenResponse struct {
	Token           string    `json:"token"`
	ExpiresAt       time.Time `json:"expires_at"`
	ServerID        int       `json:"server_id"`
	ServerHostname  string    `json:"server_hostname"`
	ServerIP        string    `json:"server_ip"`
	BaseURL         string    `json:"base_url"`
	OS              string    `json:"os"`
	InstallCommand  string    `json:"install_command"`
	BinaryAvailable bool      `json:"binary_available"`
}

func (c *Client) GenerateEnrollToken(serverID, validityDays int, osName string) (*EnrollTokenResponse, error) {
	payload := map[string]interface{}{"validity_days": validityDays, "os": osName}
	resp, err := c.sendRequest("POST", fmt.Sprintf("/servers/%d/enroll-token", serverID), payload)
	if err != nil {
		return nil, err
	}
	var out EnrollTokenResponse
	if err := decodeOK(resp, http.StatusOK, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// --- Oturum asciicast dışa aktarımı ---

// ExportSessionAsciicast, oturumu asciinema .cast formatında ham bayt olarak
// indirir; UI'daki exportSessionAsciicast ile aynı uca gider.
func (c *Client) ExportSessionAsciicast(sessionID int) ([]byte, error) {
	resp, err := c.sendRequest("GET", fmt.Sprintf("/sessions/%d/asciicast", sessionID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API hatası %s: %s", resp.Status, string(bodyBytes))
	}
	return io.ReadAll(resp.Body)
}

// --- Hesap (whoami / parola) ---

type AuthIdentity struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	Role        string `json:"role"`
	DisplayName string `json:"display_name"`
	TotpEnabled bool   `json:"totp_enabled"`
}

func (c *Client) GetMe() (*AuthIdentity, error) {
	resp, err := c.sendRequest("GET", "/auth/me", nil)
	if err != nil {
		return nil, err
	}
	var me AuthIdentity
	if err := decodeOK(resp, http.StatusOK, &me); err != nil {
		return nil, err
	}
	return &me, nil
}

func (c *Client) ChangePassword(currentPassword, newPassword string) error {
	payload := map[string]string{"current_password": currentPassword, "new_password": newPassword}
	resp, err := c.sendRequest("POST", "/auth/change-password", payload)
	if err != nil {
		return err
	}
	return decodeOK(resp, http.StatusNoContent, nil)
}
