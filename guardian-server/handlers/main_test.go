// guardian/guardian-server/handlers/main_test.go

package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// testApp, test süresince kullanılacak veritabanı bağlantısı gibi
// global nesneleri tutar.
type testApp struct {
	Router *chi.Mux
	DB     *sql.DB
}

var app testApp

// TestMain, 'handlers' paketindeki TÜM testler çalışmadan önce bir kez çalışır.
// Görevi, test veritabanı bağlantısını kurmak ve tüm testler bittikten sonra kapatmaktır.
func TestMain(m *testing.M) {
	// Ortam değişkenlerinden test veritabanı bilgilerini oku, yoksa varsayılanları kullan.
	dbUser := os.Getenv("POSTGRES_USER")
	dbPassword := os.Getenv("POSTGRES_PASSWORD")
	dbHost := os.Getenv("POSTGRES_HOST")
	dbPort := os.Getenv("POSTGRES_PORT")
	dbName := os.Getenv("POSTGRES_DB")

	if dbUser == "" {
		dbUser = "guardian_user"
	}
	if dbPassword == "" {
		dbPassword = "guardian_password"
	}
	if dbHost == "" {
		dbHost = "localhost"
	}
	if dbPort == "" {
		dbPort = "5432"
	}
	if dbName == "" {
		dbName = "guardian_db_test"
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", dbUser, dbPassword, dbHost, dbPort, dbName)

	var err error
	app.DB, err = sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("Test veritabanı sürücüsü açılamadı: %v", err)
	}
	if err = app.DB.Ping(); err != nil {
		log.Fatalf("Test veritabanına bağlanılamadı: %v", err)
	}

	app.Router = chi.NewRouter()

	// Tüm testleri çalıştır.
	code := m.Run()

	// Testler bittikten sonra veritabanı bağlantısını kapat.
	app.DB.Close()

	os.Exit(code)
}

// --- ORTAK TEST HELPER'LARI ---
// Bu fonksiyonlar, paketteki tüm *_test.go dosyaları tarafından kullanılabilir.

// executeRequest, bir HTTP isteğini alır ve sahte bir cevap döndürür.
func executeRequest(req *http.Request, router http.Handler) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// checkResponseCode, beklenen ve gelen HTTP durum kodlarını karşılaştırır.
func checkResponseCode(t *testing.T, expected, actual int) {
	if expected != actual {
		t.Errorf("Beklenen durum kodu %d. Gelen: %d", expected, actual)
	}
}

// resetTables, testlerin birbirini etkilememesi için tüm tabloları temizler.
func resetTables(t *testing.T, db *sql.DB) {
	// audit_logs'u da truncate listesine ekleyelim.
	_, err := db.Exec("TRUNCATE TABLE audit_logs, access_rules, servers, public_keys, system_users, sessions, session_events RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatalf("Tablolar sıfırlanamadı: %v", err)
	}
}

// insertTestServer, teste özel bir sunucu oluşturur ve ID'sini döndürür.
func insertTestServer(t *testing.T, hostname, ip string) int {
	var id int
	err := app.DB.QueryRow("INSERT INTO servers (hostname, ip_address) VALUES ($1, $2) RETURNING id", hostname, ip).Scan(&id)
	if err != nil {
		t.Fatalf("Failed to insert test server: %v", err)
	}
	return id
}

// insertTestUser, teste özel bir kullanıcı oluşturur ve ID'sini döndürür.
func insertTestUser(t *testing.T, username string) int {
	var id int
	err := app.DB.QueryRow("INSERT INTO system_users (username) VALUES ($1) RETURNING id", username).Scan(&id)
	if err != nil {
		t.Fatalf("Failed to insert test user: %v", err)
	}
	return id
}

// insertTestKey, teste özel bir anahtar oluşturur ve ID'sini döndürür.
func insertTestKey(t *testing.T, keyName, publicKey string) int {
	// Her test için farklı bir parmak izi kullanmak, unique constraint hatalarını önler.
	dummyFingerprint := fmt.Sprintf("dummy-fingerprint-%s-%d", keyName, time.Now().UnixNano())
	var id int
	err := app.DB.QueryRow("INSERT INTO public_keys (key_name, ssh_public_key, fingerprint_sha256) VALUES ($1, $2, $3) RETURNING id", keyName, publicKey, dummyFingerprint).Scan(&id)
	if err != nil {
		t.Fatalf("Failed to insert test key: %v", err)
	}
	return id
}

// insertTestRule, teste özel bir kural oluşturur ve ID'sini döndürür.
func insertTestRule(t *testing.T, serverID, userID, keyID int, status string) int {
	var id int
	validFrom := time.Now()
	validUntil := validFrom.Add(1 * time.Hour)
	err := app.DB.QueryRow(
		"INSERT INTO access_rules (server_id, system_user_id, public_key_id, status, valid_from, valid_until) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id",
		serverID, userID, keyID, status, validFrom, validUntil,
	).Scan(&id)
	if err != nil {
		t.Fatalf("Failed to insert test rule: %v", err)
	}
	return id
}

// insertTestSession, teste özel bir oturum oluşturur ve ID'sini döndürür.
func insertTestSession(t *testing.T, serverID int, username, status string) int {
	var id int
	err := app.DB.QueryRow(
		"INSERT INTO sessions (server_id, username, status) VALUES ($1, $2, $3) RETURNING id",
		serverID, username, status,
	).Scan(&id)
	if err != nil {
		t.Fatalf("Failed to insert test session: %v", err)
	}
	return id
}
