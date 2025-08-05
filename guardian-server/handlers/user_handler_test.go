// guardian/guardian-server/handlers/user_handler_test.go

package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"guardian.com/server/models"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

func TestUserHandlers(t *testing.T) {
	testRouter := chi.NewRouter()
	testRouter.Post("/api/users", CreateSystemUser(app.DB))
	testRouter.Delete("/api/users/{userID}", DeleteSystemUser(app.DB))
	testRouter.Get("/api/users", ListSystemUsers(app.DB))

	t.Run("CreateUser: should create a new system user successfully", func(t *testing.T) {
		resetTables(t, app.DB)

		// DÜZELTME: Payload'ı sql.NullString'in JSON formatına uygun hale getir.
		userPayload := `{"username": "test-admin", "description": {"String": "Test Admin User", "Valid": true}}`

		req, _ := http.NewRequest("POST", "/api/users", bytes.NewBufferString(userPayload))
		response := executeRequest(req, testRouter)

		assert.Equal(t, http.StatusCreated, response.Code)

		var newUser models.SystemUser
		err := json.Unmarshal(response.Body.Bytes(), &newUser)
		assert.NoError(t, err)
		assert.Equal(t, "test-admin", newUser.Username)
		assert.True(t, newUser.Description.Valid)
		assert.Equal(t, "Test Admin User", newUser.Description.String)
		assert.NotZero(t, newUser.ID)
	})

	// DÜZELTME: Description olmadan da kullanıcı oluşturabildiğimizi test edelim.
	t.Run("CreateUser: should create a user without a description", func(t *testing.T) {
		resetTables(t, app.DB)

		// Payload'da description alanı hiç yok.
		userPayload := `{"username": "no-desc-user"}`

		req, _ := http.NewRequest("POST", "/api/users", bytes.NewBufferString(userPayload))
		response := executeRequest(req, testRouter)

		assert.Equal(t, http.StatusCreated, response.Code)

		var newUser models.SystemUser
		err := json.Unmarshal(response.Body.Bytes(), &newUser)
		assert.NoError(t, err)
		assert.Equal(t, "no-desc-user", newUser.Username)
		// Description'ın geçersiz (NULL) geldiğini kontrol et.
		assert.False(t, newUser.Description.Valid)
	})

	t.Run("DeleteUser: should delete an existing user", func(t *testing.T) {
		resetTables(t, app.DB)

		userID := insertTestUser(t, "user-to-delete")

		req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/users/%d", userID), nil)
		response := executeRequest(req, testRouter)

		assert.Equal(t, http.StatusNoContent, response.Code)

		var count int
		app.DB.QueryRow("SELECT COUNT(*) FROM system_users WHERE id = $1", userID).Scan(&count)
		assert.Zero(t, count, "User should have been deleted from the database")
	})

	t.Run("DeleteUser: should fail to delete a user that is in use by a rule", func(t *testing.T) {
		resetTables(t, app.DB)

		serverID := insertTestServer(t, "server-for-user-test", "8.8.8.8")
		userID := insertTestUser(t, "user-in-use")
		keyID := insertTestKey(t, "key-for-user-test", "ssh-rsa IN_USE")
		insertTestRule(t, serverID, userID, keyID, "active")

		req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/users/%d", userID), nil)
		response := executeRequest(req, testRouter)

		assert.Equal(t, http.StatusConflict, response.Code)
		assert.Contains(t, response.Body.String(), "kural tarafından kullanılıyor")

		var count int
		app.DB.QueryRow("SELECT COUNT(*) FROM system_users WHERE id = $1", userID).Scan(&count)
		assert.Equal(t, 1, count, "User should NOT have been deleted")
	})

	t.Run("ListUsers: should return a paginated list of users", func(t *testing.T) {
		resetTables(t, app.DB)

		insertTestUser(t, "user1")
		insertTestUser(t, "user2")

		req, _ := http.NewRequest("GET", "/api/users", nil)
		response := executeRequest(req, testRouter)

		assert.Equal(t, http.StatusOK, response.Code)

		var paginatedResponse struct {
			Data []models.SystemUser `json:"data"`
		}
		err := json.Unmarshal(response.Body.Bytes(), &paginatedResponse)
		assert.NoError(t, err)
		assert.Len(t, paginatedResponse.Data, 2)
	})
}
