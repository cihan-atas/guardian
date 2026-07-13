// guardian/guardian-server/handlers/key_handler_test.go

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

func TestKeyHandlers(t *testing.T) {
	requireDB(t)
	testRouter := chi.NewRouter()
	testRouter.Post("/api/keys", CreatePublicKey(app.DB))
	testRouter.Delete("/api/keys/{keyID}", DeletePublicKey(app.DB))
	testRouter.Get("/api/keys", ListPublicKeys(app.DB))

	validPublicKey := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCzH6dGjfsXA33t8CjlplE6OsEzoliynScv4m+Hx+l6LphnSI5WUO292U6KSj5vfCmzfu6OKHTbV/IBZwxq9tv5IJRSKIvyUwlXMaCYAVxIEJ9gE0jU4O9cnQ9xWLeQNPIshP2tm7389ZJAFz2YKW3u4UPTNo4Mdm4Bu7TpiQC4AlPCeYsc6+vbv3MNomIWlObxGpnFw8uoiMiOPIDqTnj0QsEueKgqG24XrGqVYvMDXt/BYlQEhzHuzucYpk3eBDSzqORZnIiA7zmDPWxBJKAikzaSstskJTYSoizx0k0F8XQBpLbwQpsDebafGBfo82myyFrb7xaJZw7egWTePWOX example@guardian"
	t.Run("CreateKey: should create a new public key successfully", func(t *testing.T) {
		resetTables(t, app.DB)

		// JSON string'i oluştururken anahtarı doğru şekilde yerleştir
		keyPayload := fmt.Sprintf(`{"key_name": "dev-team-key", "ssh_public_key": "%s"}`, validPublicKey)
		req, _ := http.NewRequest("POST", "/api/keys", bytes.NewBufferString(keyPayload))
		response := executeRequest(req, testRouter)

		assert.Equal(t, http.StatusCreated, response.Code)

		var newKey models.PublicKey
		err := json.Unmarshal(response.Body.Bytes(), &newKey)
		assert.NoError(t, err)
		assert.Equal(t, "dev-team-key", newKey.KeyName)
		assert.NotEmpty(t, newKey.FingerprintSHA256)
	})

	t.Run("CreateKey: should fail with an invalid public key", func(t *testing.T) {
		resetTables(t, app.DB)

		keyPayload := `{"key_name": "invalid-key", "ssh_public_key": "this-is-not-a-key"}`
		req, _ := http.NewRequest("POST", "/api/keys", bytes.NewBufferString(keyPayload))
		response := executeRequest(req, testRouter)

		assert.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), "Geçersiz public anahtar formatı")
	})

	t.Run("DeleteKey: should delete an existing key", func(t *testing.T) {
		resetTables(t, app.DB)

		keyID := insertTestKey(t, "key-to-delete", validPublicKey)

		req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/keys/%d", keyID), nil)
		response := executeRequest(req, testRouter)

		assert.Equal(t, http.StatusNoContent, response.Code)

		var count int
		app.DB.QueryRow("SELECT COUNT(*) FROM public_keys WHERE id = $1", keyID).Scan(&count)
		assert.Zero(t, count, "Key should have been deleted")
	})

	t.Run("ListKeys: should return a paginated list of keys", func(t *testing.T) {
		resetTables(t, app.DB)

		insertTestKey(t, "key1", "ssh-rsa KEY1...")
		insertTestKey(t, "key2", "ssh-rsa KEY2...")

		req, _ := http.NewRequest("GET", "/api/keys", nil)
		response := executeRequest(req, testRouter)

		assert.Equal(t, http.StatusOK, response.Code)

		var paginatedResponse struct {
			Data []models.PublicKey `json:"data"`
		}
		err := json.Unmarshal(response.Body.Bytes(), &paginatedResponse)
		assert.NoError(t, err)
		assert.Len(t, paginatedResponse.Data, 2)
	})
}
