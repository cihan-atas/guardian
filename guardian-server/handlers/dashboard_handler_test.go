// guardian/guardian-server/handlers/dashboard_handler_test.go

package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

func TestDashboardHandlers(t *testing.T) {
	requireDB(t)
	testRouter := chi.NewRouter()
	testRouter.Get("/api/dashboard/stats", GetDashboardStats(app.DB))
	testRouter.Get("/api/dashboard/session-activity", GetSessionActivity(app.DB))
	testRouter.Get("/api/dashboard/top-servers", GetTopServers(app.DB))
	testRouter.Get("/api/dashboard/active-sessions", GetActiveSessionsList(app.DB))
	testRouter.Get("/api/dashboard/audit-stream", GetAuditLogStream(app.DB))

	endpoints := []string{
		"/api/dashboard/stats",
		"/api/dashboard/session-activity",
		"/api/dashboard/top-servers",
		"/api/dashboard/active-sessions",
		"/api/dashboard/audit-stream",
	}

	for _, endpoint := range endpoints {
		// Her endpoint için ayrı bir alt test çalıştır
		t.Run(endpoint, func(t *testing.T) {
			// Veritabanının boş olması önemli değil, handler'lar boş sonuç döndürmeli.
			resetTables(t, app.DB)

			req, _ := http.NewRequest("GET", endpoint, nil)
			response := executeRequest(req, testRouter)

			// Tek beklentimiz, endpoint'in çökmemesi ve 200 OK dönmesi.
			assert.Equal(t, http.StatusOK, response.Code)

			// Dönen cevabın geçerli bir JSON olup olmadığını da kontrol edebiliriz.
			var js json.RawMessage
			err := json.Unmarshal(response.Body.Bytes(), &js)
			assert.NoError(t, err, "Response should be valid JSON")
		})
	}
}
