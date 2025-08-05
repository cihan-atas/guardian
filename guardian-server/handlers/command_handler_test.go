// guardian/guardian-server/handlers/command_handler_test.go

package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"guardian.com/server/services"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

func TestCommandHandlers(t *testing.T) {
	t.Run("GetSessionCommands: should retrieve commands for a session", func(t *testing.T) {
		resetTables(t, app.DB)
		testRouter := chi.NewRouter()
		testRouter.Get("/api/sessions/{sessionID}/commands", GetSessionCommands(app.DB))

		serverID := insertTestServer(t, "replay-server", "4.4.4.4")
		sessionID := insertTestSession(t, serverID, "replay-user", "ended")

		_, err := app.DB.Exec(`
			INSERT INTO session_events (session_id, event_type, data) VALUES 
			($1, 'input', $2), 
			($1, 'output', $3),
			($1, 'output', $4)`,
			sessionID,
			[]byte("ls -l"),
			[]byte("total 0\r\n"),
			[]byte("drwxr-xr-x file.txt\r\n"),
		)
		assert.NoError(t, err)

		req, _ := http.NewRequest("GET", fmt.Sprintf("/api/sessions/%d/commands", sessionID), nil)
		response := executeRequest(req, testRouter)
		assert.Equal(t, http.StatusOK, response.Code)

		var details services.SessionDetails
		err = json.Unmarshal(response.Body.Bytes(), &details)
		assert.NoError(t, err)
		assert.Equal(t, "replay-user", details.SessionInfo.Username)
		assert.Len(t, details.Commands, 1)
		if len(details.Commands) > 0 {
			assert.Equal(t, "ls -l", details.Commands[0].Command)
			assert.Contains(t, details.Commands[0].Output, "file.txt")
		}
	})
}
