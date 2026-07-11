package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"guardian.com/server/agentclient"
	"guardian.com/server/handlers"
	"guardian.com/server/hub"
	"guardian.com/server/scheduler"
	"guardian.com/server/services"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func printRoutes(r chi.Router) {
	chi.Walk(r, func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		log.Printf("[ROUTE] %s %s", method, route)
		return nil
	})
}
func main() {
	dbUser := os.Getenv("POSTGRES_USER")
	dbPassword := os.Getenv("POSTGRES_PASSWORD")
	dbHost := getEnv("POSTGRES_HOST", "db")
	dbPort := getEnv("POSTGRES_PORT", "5432")
	dbName := os.Getenv("POSTGRES_DB")
	serverPort := getEnv("GUARDIAN_SERVER_PORT", "5555")
	certFile := getEnv("TLS_CERT_FILE", "../certs/server.crt")
	keyFile := getEnv("TLS_KEY_FILE", "../certs/server.key")
	agentPort := getEnv("GUARDIAN_AGENT_PORT", "6666")
	secretToken := os.Getenv("GUARDIAN_SECRET_TOKEN")
	caCertFile := getEnv("TLS_CA_FILE", "../certs/ca.crt")

	// Bildirim/alarm ayarları artık DB'de tutulur ve UI'dan yönetilir; env
	// değerleri yalnızca ilk açılışta (settings tablosu boşsa) tohum olur.
	settingsEnvDefaults := map[string]string{
		services.SettingWebhookURL:      os.Getenv("GUARDIAN_WEBHOOK_URL"),
		services.SettingSMTPHost:        os.Getenv("GUARDIAN_SMTP_HOST"),
		services.SettingSMTPPort:        getEnv("GUARDIAN_SMTP_PORT", "587"),
		services.SettingSMTPUser:        os.Getenv("GUARDIAN_SMTP_USER"),
		services.SettingSMTPPass:        os.Getenv("GUARDIAN_SMTP_PASS"),
		services.SettingSMTPFrom:        os.Getenv("GUARDIAN_SMTP_FROM"),
		services.SettingAlertEmailTo:    os.Getenv("GUARDIAN_ALERT_EMAIL_TO"),
		services.SettingRiskyAutoAction: getEnv("GUARDIAN_RISKY_AUTOACTION", "none"),
	}

	if dbUser == "" || dbPassword == "" || dbName == "" {
		log.Fatal("FATAL: Veritabanı için gerekli ortam değişkenleri eksik.")
	}
	if secretToken == "" {
		log.Fatal("FATAL: Güvenlik için GUARDIAN_SECRET_TOKEN ortam değişkeni ayarlanmamış!")
	}
	if os.Getenv("GUARDIAN_ADMIN_TOKEN") == "" {
		log.Fatal("FATAL: Güvenlik için GUARDIAN_ADMIN_TOKEN ortam değişkeni ayarlanmamış!")
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	log.Println("Veritabanına bağlanılıyor...")
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("Veritabanı sürücüsü açılamadı: %v", err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		log.Fatalf("Veritabanına bağlanılamadı: %v", err)
	}
	fmt.Println("✅ Başarıyla PostgreSQL veritabanına bağlanıldı!")

	if err := seedExampleData(db); err != nil {
		log.Fatalf("Örnek veriler oluşturulurken hata oluştu: %v", err)
	}

	// alerts tablosunu (yoksa) oluştur + ayarları (DB + env tohumu) yükleyip
	// bildirim/alarm katmanını yapılandır.
	if err := services.EnsureAlertsTable(db); err != nil {
		log.Fatalf("alerts tablosu oluşturulamadı: %v", err)
	}
	if err := services.InitSettings(db, settingsEnvDefaults); err != nil {
		log.Fatalf("ayarlar yüklenemedi: %v", err)
	}

	ac := agentclient.New(agentPort, secretToken, caCertFile)
	wsHub := hub.NewHub()

	go wsHub.Run()
	go scheduler.Start(db, ac)

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Guardian Server API'sine (Chi Router ile) hoş geldiniz!")
	})

	r.Route("/api", func(r chi.Router) {

		r.Group(func(r chi.Router) {
			r.Use(handlers.AgentAuth)
			r.Post("/agent/sessions", handlers.StartSession(db))
			r.Patch("/agent/sessions/{sessionID}", handlers.EndSession(db))
			r.Get("/agent/ws/sessions/{sessionID}", handlers.AgentSessionStreamHandler(db, wsHub, ac))
		})

		r.Group(func(r chi.Router) {
			r.Use(handlers.AdminWSAuth)
			r.Get("/ws/sessions/{sessionID}", handlers.ViewerSessionStreamHandler(wsHub))
		})

		r.Group(func(r chi.Router) {
			r.Use(handlers.AdminAuth)

			r.Get("/auth/check", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"status": "ok"}`))
			})

			r.Route("/dashboard", func(r chi.Router) {
				r.Get("/stats", handlers.GetDashboardStats(db))
				r.Get("/session-activity", handlers.GetSessionActivity(db))
				r.Get("/top-servers", handlers.GetTopServers(db))
				r.Get("/session-status", handlers.GetSessionStatusBreakdown(db))
				r.Get("/rule-status", handlers.GetRuleStatusBreakdown(db))
				r.Get("/top-commands", handlers.GetTopCommands(db))
				r.Get("/command-stats", handlers.GetCommandStats(db))
				r.Get("/user-activity", handlers.GetUserActivity(db))
				r.Get("/hourly-activity", handlers.GetHourlyActivity(db))
				r.Get("/active-sessions", handlers.GetActiveSessionsList(db))
				r.Get("/audit-stream", handlers.GetAuditLogStream(db))
				r.Get("/alerts", handlers.GetAlerts(db))
			})
			r.Route("/servers", func(r chi.Router) {
				r.Get("/", handlers.ListServers(db))
				r.Post("/", handlers.CreateServer(db))
				r.Route("/{serverID}", func(r chi.Router) {
					r.Get("/", handlers.GetServer(db))
					r.Put("/", handlers.UpdateServer(db))
					r.Patch("/", handlers.PatchServer(db))
					r.Delete("/", handlers.DeleteServer(db))
				})
			})

			r.Route("/rules", func(r chi.Router) {
				r.Get("/", handlers.ListRules(db))
				// ac'yi (agent client) CreateRule'a geçir
				r.Post("/", handlers.CreateRule(db, ac))
				r.Route("/{ruleID}", func(r chi.Router) {
					r.Get("/", handlers.GetRule(db))
					r.Put("/", handlers.UpdateRule(db))
					// ac'yi PatchRule ve DeleteRule'a geçir
					r.Patch("/", handlers.PatchRule(db, ac))
					r.Delete("/", handlers.DeleteRule(db, ac))
				})
			})

			r.Route("/keys", func(r chi.Router) {
				r.Get("/", handlers.ListPublicKeys(db))
				r.Post("/", handlers.CreatePublicKey(db))
				r.Route("/{keyID}", func(r chi.Router) {
					r.Get("/", handlers.GetPublicKey(db))
					r.Patch("/", handlers.PatchPublicKey(db))
					r.Delete("/", handlers.DeletePublicKey(db))
					r.Get("/ban", handlers.GetKeyBanStatus(db))
					r.Post("/ban", handlers.BanPublicKey(db, ac))
					r.Delete("/ban", handlers.UnbanPublicKey(db))
				})
			})

			r.Route("/users", func(r chi.Router) {
				r.Get("/", handlers.ListSystemUsers(db))
				r.Post("/", handlers.CreateSystemUser(db))
				r.Route("/{userID}", func(r chi.Router) {
					r.Get("/", handlers.GetSystemUser(db))
					r.Patch("/", handlers.PatchSystemUser(db))
					r.Delete("/", handlers.DeleteSystemUser(db))
				})
			})

			r.Route("/settings", func(r chi.Router) {
				r.Get("/", handlers.GetSettings(db))
				r.Put("/", handlers.UpdateSettings(db))
				r.Post("/test", handlers.TestNotification(db))
			})

			r.Route("/sessions", func(r chi.Router) {
				r.Get("/", handlers.ListSessions(db))

				r.Route("/{sessionID}", func(r chi.Router) {
					// ac'yi (agent client) TerminateSession'a geçir
					r.Delete("/", handlers.TerminateSession(db, ac))
					r.Get("/replay", handlers.GetSessionReplay(db))
					r.Get("/commands", handlers.GetSessionCommands(db))
					r.Get("/meta", handlers.GetSessionMeta(db))
				})
			})
		})
	})

	fmt.Printf("🚀 Guardian Server (Chi) https://localhost:%s adresinde GÜVENLİ modda başlatılıyor...\n", serverPort)
	err = http.ListenAndServeTLS(":"+serverPort, certFile, keyFile, r)
	if err != nil {
		log.Fatalf("Güvenli (TLS) sunucu başlatılamadı: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
