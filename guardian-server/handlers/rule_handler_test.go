// guardian/guardian-server/handlers/rule_handler_test.go

package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"guardian.com/server/agentclient"
	"guardian.com/server/models"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Mock Agent Client ---
// Bu mock, SADECE bu test paketi (handlers) için geçerlidir.
type MockAgentClient struct {
	mock.Mock
}

func (m *MockAgentClient) SendKeyCommand(ip, action string, payload models.KeyPayload) error {
	args := m.Called(ip, action, payload)
	return args.Error(0)
}

func (m *MockAgentClient) TerminateSession(ip string, sessionID int) error {
	args := m.Called(ip, sessionID)
	return args.Error(0)
}

func (m *MockAgentClient) ValidateUser(ip, username string) error {
	args := m.Called(ip, username)
	return args.Error(0)
}

var _ agentclient.AgentCommunicator = (*MockAgentClient)(nil)

// --- Test Fonksiyonları ---

func TestRuleHandlers(t *testing.T) {
	// Router'ı test fonksiyonunun dışında, bir kez tanımlamak yeterli.
	testRouter := chi.NewRouter()

	// Her t.Run bloğu, kendi handler'ını ve mock'unu oluşturacak
	// ve router'a o anki test için ekleyecek.

	t.Run("CreateRule: should create a new rule successfully", func(t *testing.T) {
		// Teste başlamadan önce veritabanını temizle.
		resetTables(t, app.DB)
		mockAgent := new(MockAgentClient)

		// Bu teste özel handler'ı router'a bağla.
		testRouter.Post("/api/rules", CreateRule(app.DB, mockAgent))

		serverID := insertTestServer(t, "test-server", "1.1.1.1")
		userID := insertTestUser(t, "test-user")
		keyID := insertTestKey(t, "test-key-1", "ssh-rsa AAAA...")

		mockAgent.On("ValidateUser", "1.1.1.1", "test-user").Return(nil).Once()

		now := time.Now()
		validUntil := now.Add(1 * time.Hour)
		rulePayload := map[string]interface{}{
			"server_id":      serverID,
			"system_user_id": userID,
			"public_key_id":  keyID,
			"valid_from":     now.Format(time.RFC3339),
			"valid_until":    validUntil.Format(time.RFC3339),
		}
		jsonBody, _ := json.Marshal(rulePayload)

		req, _ := http.NewRequest("POST", "/api/rules", bytes.NewBuffer(jsonBody))
		response := executeRequest(req, testRouter)

		assert.Equal(t, http.StatusCreated, response.Code, "Expected status code 201")

		var newRule models.AccessRule
		json.Unmarshal(response.Body.Bytes(), &newRule)
		assert.Equal(t, serverID, newRule.ServerID)
		assert.NotZero(t, newRule.ID)

		mockAgent.AssertExpectations(t)
	})

	t.Run("CreateRule: should fail if user validation on agent fails", func(t *testing.T) {
		// Teste başlamadan önce veritabanını temizle.
		resetTables(t, app.DB)
		mockAgent := new(MockAgentClient)
		testRouter.Post("/api/rules", CreateRule(app.DB, mockAgent))

		serverID := insertTestServer(t, "another-server", "2.2.2.2")
		userID := insertTestUser(t, "invalid-user")
		keyID := insertTestKey(t, "another-key-2", "ssh-rsa BBBB...")

		mockAgent.On("ValidateUser", "2.2.2.2", "invalid-user").
			Return(fmt.Errorf("user not found on system")).Once()

		rulePayload := map[string]interface{}{
			"server_id": serverID, "system_user_id": userID, "public_key_id": keyID,
			"valid_from": time.Now(), "valid_until": time.Now().Add(1 * time.Hour),
		}
		jsonBody, _ := json.Marshal(rulePayload)

		req, _ := http.NewRequest("POST", "/api/rules", bytes.NewBuffer(jsonBody))
		response := executeRequest(req, testRouter)

		assert.Equal(t, http.StatusBadRequest, response.Code, "Expected status code 400")
		assert.Contains(t, response.Body.String(), "Kullanıcı doğrulanamadı")

		mockAgent.AssertExpectations(t)
	})

	t.Run("DeleteRule: should delete an existing rule", func(t *testing.T) {
		// Teste başlamadan önce veritabanını temizle.
		resetTables(t, app.DB)
		mockAgent := new(MockAgentClient)
		testRouter.Delete("/api/rules/{ruleID}", DeleteRule(app.DB, mockAgent))

		serverID := insertTestServer(t, "deletable-server", "3.3.3.3")
		userID := insertTestUser(t, "deletable-user")
		keyID := insertTestKey(t, "deletable-key-3", "ssh-rsa CCCC...")
		ruleID := insertTestRule(t, serverID, userID, keyID, "active")

		mockAgent.On("SendKeyCommand", "3.3.3.3", "remove", mock.Anything).Return(nil).Once()

		req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/rules/%d", ruleID), nil)
		response := executeRequest(req, testRouter)

		assert.Equal(t, http.StatusNoContent, response.Code, "Expected status code 204")

		var count int
		app.DB.QueryRow("SELECT COUNT(*) FROM access_rules WHERE id = $1", ruleID).Scan(&count)
		assert.Zero(t, count, "Rule should be deleted from the database")

		mockAgent.AssertExpectations(t)
	})
}
