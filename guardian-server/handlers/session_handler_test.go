// guardian/guardian-server/handlers/session_handler_test.go

package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"guardian.com/server/agentclient"
	"guardian.com/server/models"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAgentClient, SADECE bu test paketi için geçerlidir.
type SessionMockAgentClient struct {
	mock.Mock
	wg *sync.WaitGroup
}

func (m *SessionMockAgentClient) SendKeyCommand(ip, action string, payload models.KeyPayload) error {
	args := m.Called(ip, action, payload)
	return args.Error(0)
}

func (m *SessionMockAgentClient) TerminateSession(ip string, sessionID int) error {
	if m.wg != nil {
		defer m.wg.Done()
	}
	args := m.Called(ip, sessionID)
	return args.Error(0)
}

func (m *SessionMockAgentClient) ValidateUser(ip, username string) error {
	args := m.Called(ip, username)
	return args.Error(0)
}

// Bu satır mock'un interface'i tam olarak karşıladığını garanti eder.
var _ agentclient.AgentCommunicator = (*SessionMockAgentClient)(nil)

func TestSessionHandlers(t *testing.T) {

	t.Run("ListSessions: should return a paginated list of sessions", func(t *testing.T) {
		resetTables(t, app.DB)
		testRouter := chi.NewRouter()
		testRouter.Get("/api/sessions", ListSessions(app.DB))

		serverID := insertTestServer(t, "session-server", "5.5.5.5")
		insertTestSession(t, serverID, "user1", "active")
		insertTestSession(t, serverID, "user2", "ended")

		req, _ := http.NewRequest("GET", "/api/sessions", nil)
		response := executeRequest(req, testRouter)

		assert.Equal(t, http.StatusOK, response.Code)

		var paginatedResponse struct {
			Data []models.SessionDetailsAPI `json:"data"`
		}
		err := json.Unmarshal(response.Body.Bytes(), &paginatedResponse)
		assert.NoError(t, err)
		assert.Len(t, paginatedResponse.Data, 2)
		assert.Equal(t, "session-server", paginatedResponse.Data[0].ServerHostname)
	})

	t.Run("TerminateSession: should terminate an active session", func(t *testing.T) {
		resetTables(t, app.DB)
		testRouter := chi.NewRouter()
		var wg sync.WaitGroup
		mockAgent := &SessionMockAgentClient{wg: &wg}
		testRouter.Delete("/api/sessions/{sessionID}", TerminateSession(app.DB, mockAgent))

		serverID := insertTestServer(t, "live-server", "6.6.6.6")
		sessionID := insertTestSession(t, serverID, "live-user", "active")

		wg.Add(1)
		mockAgent.On("TerminateSession", "6.6.6.6", sessionID).Return(nil).Once()

		req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/sessions/%d", sessionID), nil)
		response := executeRequest(req, testRouter)
		assert.Equal(t, http.StatusOK, response.Code)

		wg.Wait()
		mockAgent.AssertExpectations(t)

		var status string
		app.DB.QueryRow("SELECT status FROM sessions WHERE id = $1", sessionID).Scan(&status)
		assert.Equal(t, "terminated_by_admin", status)
	})

	t.Run("TerminateSession: should not call agent for an already ended session", func(t *testing.T) {
		resetTables(t, app.DB)
		testRouter := chi.NewRouter()
		mockAgent := new(SessionMockAgentClient)
		testRouter.Delete("/api/sessions/{sessionID}", TerminateSession(app.DB, mockAgent))

		serverID := insertTestServer(t, "ended-server", "7.7.7.7")
		sessionID := insertTestSession(t, serverID, "ended-user", "ended")

		req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/sessions/%d", sessionID), nil)
		response := executeRequest(req, testRouter)
		assert.Equal(t, http.StatusOK, response.Code)
		mockAgent.AssertExpectations(t)
	})

	// GetSessionCommands testi bu dosyadan kaldırıldı ve command_handler_test.go'ya taşındı.
}
