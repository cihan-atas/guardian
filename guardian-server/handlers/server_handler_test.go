// guardian/guardian-server/handlers/server_handler_test.go

package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert" // assert kütüphanesini import ediyoruz
	"guardian.com/server/models"
)

func TestServerHandlers(t *testing.T) {
	// Her testin başında veritabanını sıfırlamak iyi bir pratiktir.
	resetTables(t, app.DB)

	testRouter := chi.NewRouter()
	testRouter.Get("/api/servers", ListServers(app.DB))
	testRouter.Post("/api/servers", CreateServer(app.DB))
	testRouter.Get("/api/servers/{serverID}", GetServer(app.DB))
	testRouter.Delete("/api/servers/{serverID}", DeleteServer(app.DB))

	t.Run("CreateServer: Yeni bir sunucu başarıyla oluşturulmalı", func(t *testing.T) {
		// Bu teste özel olarak DB'yi tekrar temizleyelim.
		resetTables(t, app.DB)

		serverJSON := `{"hostname": "prod-db-1", "ip_address": "10.0.1.5", "description": "Production DB"}`
		req, _ := http.NewRequest("POST", "/api/servers", bytes.NewBufferString(serverJSON))
		req.Header.Set("Content-Type", "application/json")

		response := executeRequest(req, testRouter)
		checkResponseCode(t, http.StatusCreated, response.Code)

		var newServer models.Server
		err := json.Unmarshal(response.Body.Bytes(), &newServer)
		assert.NoError(t, err, "JSON parse edilemedi")

		assert.Equal(t, "prod-db-1", newServer.Hostname)
		assert.NotZero(t, newServer.ID, "Sunucu ID'si 0 olmamalı")
	})

	t.Run("GetServer: Var olan bir sunucu getirilmeli", func(t *testing.T) {
		resetTables(t, app.DB)

		var serverID int
		err := app.DB.QueryRow("INSERT INTO servers (hostname, ip_address, description) VALUES ($1, $2, $3) RETURNING id",
			"staging-web", "192.168.1.100", "Staging Web Server").Scan(&serverID)
		assert.NoError(t, err, "Test verisi eklenemedi")

		url := fmt.Sprintf("/api/servers/%d", serverID)
		req, _ := http.NewRequest("GET", url, nil)
		response := executeRequest(req, testRouter)

		checkResponseCode(t, http.StatusOK, response.Code)

		var foundServer models.Server
		err = json.Unmarshal(response.Body.Bytes(), &foundServer)
		assert.NoError(t, err, "JSON parse edilemedi")

		assert.Equal(t, serverID, foundServer.ID)
		assert.Equal(t, "staging-web", foundServer.Hostname)
	})

	t.Run("GetServer: Var olmayan bir sunucu için 404 dönmeli", func(t *testing.T) {
		resetTables(t, app.DB)

		req, _ := http.NewRequest("GET", "/api/servers/9999", nil)
		response := executeRequest(req, testRouter)
		checkResponseCode(t, http.StatusNotFound, response.Code)
	})

	t.Run("DeleteServer: Var olan bir sunucu silinmeli", func(t *testing.T) {
		resetTables(t, app.DB)

		var serverID int
		err := app.DB.QueryRow("INSERT INTO servers (hostname, ip_address, description) VALUES ($1, $2, $3) RETURNING id",
			"to-be-deleted", "1.2.3.4", "Silinecek sunucu").Scan(&serverID)
		assert.NoError(t, err, "Test verisi eklenemedi")

		url := fmt.Sprintf("/api/servers/%d", serverID)
		req, _ := http.NewRequest("DELETE", url, nil)
		response := executeRequest(req, testRouter)
		checkResponseCode(t, http.StatusNoContent, response.Code)

		var count int
		err = app.DB.QueryRow("SELECT COUNT(*) FROM servers WHERE id = $1", serverID).Scan(&count)
		assert.NoError(t, err, "Veritabanı kontrol sorgusu başarısız")
		assert.Zero(t, count, "Sunucunun silinmesi bekleniyordu ama hala mevcut.")
	})

	t.Run("ListServers: Sunucular varken doğru listeyi dönmeli", func(t *testing.T) {
		resetTables(t, app.DB)

		_, err := app.DB.Exec("INSERT INTO servers (hostname, ip_address, description) VALUES ($1, $2, $3), ($4, $5, $6)",
			"list-server-1", "1.1.1.1", "desc1",
			"list-server-2", "2.2.2.2", "desc2")
		assert.NoError(t, err, "Test verisi eklenemedi")

		req, _ := http.NewRequest("GET", "/api/servers", nil)
		response := executeRequest(req, testRouter)

		checkResponseCode(t, http.StatusOK, response.Code)

		// DÜZELTME: Paginasyonlu response'u bekleyen bir struct tanımla
		var paginatedResponse struct {
			Data []models.Server `json:"data"`
			// Diğer paginasyon alanlarını da test etmek istersen ekleyebilirsin
			// TotalRecords int `json:"total_records"`
			// Page         int `json:"page"`
			// Limit        int `json:"limit"`
		}

		err = json.Unmarshal(response.Body.Bytes(), &paginatedResponse)
		assert.NoError(t, err, "JSON parse edilemedi: %v", err)

		// DÜZELTME: Gelen verinin (paginatedResponse.Data) uzunluğunu kontrol et
		assert.Len(t, paginatedResponse.Data, 2, "2 adet sunucu bekleniyordu")
	})
}
