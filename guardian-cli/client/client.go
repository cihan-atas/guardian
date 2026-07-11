package client

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
