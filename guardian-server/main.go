package main

import (
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

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

	// İlk yönetici hesabı (yalnızca admin_users tablosu boşsa kullanılır).
	bootstrapAdminUser := getEnv("GUARDIAN_ADMIN_USERNAME", "admin")
	bootstrapAdminPass := os.Getenv("GUARDIAN_ADMIN_PASSWORD")

	// Agent oto-kurulum (enrollment) için CA anahtarı + binary + genel URL.
	caKeyFile := getEnv("TLS_CA_KEY_FILE", "../certs/ca.key")
	agentBinaryPath := os.Getenv("GUARDIAN_AGENT_BINARY_PATH")
	agentBinaryPathWin := os.Getenv("GUARDIAN_AGENT_BINARY_PATH_WINDOWS")
	publicURL := os.Getenv("GUARDIAN_PUBLIC_URL")

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
		// Ajan kimlik doğrulaması artık öncelikli olarak mTLS (istemci
		// sertifikası) ile yapılıyor; paylaşımlı token yalnızca eski
		// kurulumlarla uyumluluk için geriye dönük bir yedek. Token yoksa
		// yalnızca mTLS kabul edilir.
		log.Println("UYARI: GUARDIAN_SECRET_TOKEN ayarlı değil; ajan kimlik doğrulaması yalnızca mTLS ile yapılacak (token yedeği devre dışı).")
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

	// RBAC (yönetici hesapları/oturumlar) + onay akışı için tablo/kolonlar.
	if err := services.EnsureAuthTables(db); err != nil {
		log.Fatalf("auth tabloları oluşturulamadı: %v", err)
	}
	if err := services.EnsureAccessRequestColumns(db); err != nil {
		log.Fatalf("access_rules kolonları eklenemedi: %v", err)
	}
	if err := services.BootstrapAdmin(db, bootstrapAdminUser, bootstrapAdminPass); err != nil {
		log.Fatalf("ilk yönetici oluşturulamadı: %v", err)
	}
	if err := services.EnsureEnrollTable(db); err != nil {
		log.Fatalf("agent_enroll_tokens tablosu oluşturulamadı: %v", err)
	}

	// CA anahtarını (ca.key) yükle; yoksa agent oto-kurulum (enrollment) devre
	// dışı kalır ama sunucu normal çalışmaya devam eder.
	var agentCA *services.CA
	if loaded, caErr := services.LoadCA(caCertFile, caKeyFile); caErr == nil {
		agentCA = loaded
		log.Println("✅ CA anahtarı yüklendi; agent oto-kurulum (enrollment) etkin.")
	} else {
		log.Printf("[WARN] CA anahtarı yüklenemedi (%s); agent oto-kurulum devre dışı: %v", caKeyFile, caErr)
	}

	installer := &handlers.AgentInstaller{
		DB:          db,
		CA:          agentCA,
		BinaryPath:  agentBinaryPath,
		WinBinPath:  agentBinaryPathWin,
		SecretToken: secretToken,
		ServerPort:  serverPort,
		AgentPort:   agentPort,
		PublicURL:   publicURL,
	}

	ac := agentclient.New(agentPort, secretToken, caCertFile, certFile, keyFile)
	wsHub := hub.NewHub()

	go wsHub.Run()
	go scheduler.Start(db, ac)

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// CORS origin'leri GUARDIAN_CORS_ORIGINS (virgülle ayrılmış) ile daraltılır.
	// Ayarlanmazsa geriye dönük uyumluluk için izin verici joker kalır ama
	// üretimde daraltılması için uyarı loglanır.
	corsOrigins := parseCORSOrigins(os.Getenv("GUARDIAN_CORS_ORIGINS"))
	if len(corsOrigins) == 0 {
		log.Println("UYARI: GUARDIAN_CORS_ORIGINS ayarlı değil; tüm origin'lere izin veriliyor (joker). Üretimde arayüzün gerçek origin'ine daraltın.")
		corsOrigins = []string{"https://*", "http://*"}
	}

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   corsOrigins,
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
			r.Use(handlers.AdminWSAuth(db))
			r.Get("/ws/sessions/{sessionID}", handlers.ViewerSessionStreamHandler(wsHub))
		})

		// Herkese açık giriş.
		r.Post("/auth/login", handlers.Login(db))

		// Agent kurulum uçları — kayıt (enroll) token'ıyla doğrulanır
		// (hedef sunucunun admin oturumu yoktur). AgentAuth/AdminAuth uygulanmaz.
		r.Get("/agent/install.sh", installer.ServeInstallScript())
		r.Get("/agent/install.ps1", installer.ServeInstallScriptPS())
		r.Post("/agent/enroll", installer.EnrollAgent())
		r.Post("/agent/enroll-bundle", installer.EnrollBundle())
		r.Get("/agent/ca.crt", installer.ServeCACert())
		r.Get("/agent/binary", installer.ServeBinary())

		r.Group(func(r chi.Router) {
			r.Use(handlers.AdminAuth(db))

			// Rol kapıları.
			admin := handlers.RequireRole(services.RoleAdmin)
			operator := handlers.RequireRole(services.RoleOperator)

			// Kimlik doğrulanmış her kullanıcı.
			r.Get("/auth/me", handlers.Me())
			r.Post("/auth/logout", handlers.Logout(db))
			r.Post("/auth/change-password", handlers.ChangeOwnPassword(db))
			// İki adımlı doğrulama (2FA / TOTP) — kendi hesabı için.
			r.Post("/auth/2fa/setup", handlers.Setup2FA(db))
			r.Post("/auth/2fa/enable", handlers.Enable2FA(db))
			r.Post("/auth/2fa/disable", handlers.Disable2FA(db))

			// Yönetici hesapları (yalnızca admin).
			r.Route("/admin-users", func(r chi.Router) {
				r.With(admin).Get("/", handlers.ListAdminUsers(db))
				r.With(admin).Post("/", handlers.CreateAdminUser(db))
				r.With(admin).Patch("/{adminID}", handlers.UpdateAdminUser(db))
				r.With(admin).Delete("/{adminID}", handlers.DeleteAdminUser(db))
			})

			// Denetim kaydı ekranı (yalnızca admin).
			r.With(admin).Get("/audit-logs", handlers.ListAuditLogs(db))

			// Sertifika süre-sonu göstergesi + server cert yenileme (yalnızca admin).
			r.With(admin).Get("/certificates", handlers.Certificates(db, ac, caCertFile, certFile))
			r.With(admin).Post("/certificates/server/renew", handlers.RenewServerCert(db, agentCA, certFile, keyFile))

			// Global komut arama (viewer+).
			r.Get("/commands/search", handlers.SearchCommands(db))

			// Erişim talepleri (onay akışı).
			r.Route("/access-requests", func(r chi.Router) {
				r.Get("/", handlers.ListAccessRequests(db))
				r.With(operator).Post("/", handlers.CreateAccessRequest(db))
				r.With(admin).Post("/{requestID}/approve", handlers.ApproveAccessRequest(db, ac))
				r.With(admin).Post("/{requestID}/reject", handlers.RejectAccessRequest(db))
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
				r.Get("/health", handlers.GetServersHealth(db, ac))
				r.With(admin).Post("/", handlers.CreateServer(db))
				r.Route("/{serverID}", func(r chi.Router) {
					r.Get("/", handlers.GetServer(db))
					r.With(admin).Put("/", handlers.UpdateServer(db))
					r.With(admin).Patch("/", handlers.PatchServer(db))
					r.With(admin).Delete("/", handlers.DeleteServer(db))
					// Agent kurulumu (yalnızca admin).
					r.With(admin).Post("/enroll-token", installer.GenerateEnrollToken())
					r.With(admin).Post("/ssh-install", installer.SSHInstall())
				})
			})

			r.Route("/rules", func(r chi.Router) {
				r.Get("/", handlers.ListRules(db))
				// ac'yi (agent client) CreateRule'a geçir
				r.With(admin).Post("/", handlers.CreateRule(db, ac))
				r.Route("/{ruleID}", func(r chi.Router) {
					r.Get("/", handlers.GetRule(db))
					r.With(admin).Put("/", handlers.UpdateRule(db))
					// ac'yi PatchRule ve DeleteRule'a geçir
					r.With(admin).Patch("/", handlers.PatchRule(db, ac))
					r.With(admin).Delete("/", handlers.DeleteRule(db, ac))
				})
			})

			r.Route("/keys", func(r chi.Router) {
				r.Get("/", handlers.ListPublicKeys(db))
				r.With(admin).Post("/", handlers.CreatePublicKey(db))
				r.Route("/{keyID}", func(r chi.Router) {
					r.Get("/", handlers.GetPublicKey(db))
					r.With(admin).Patch("/", handlers.PatchPublicKey(db))
					r.With(admin).Delete("/", handlers.DeletePublicKey(db))
					r.Get("/ban", handlers.GetKeyBanStatus(db))
					r.With(admin).Post("/ban", handlers.BanPublicKey(db, ac))
					r.With(admin).Delete("/ban", handlers.UnbanPublicKey(db))
				})
			})

			r.Route("/users", func(r chi.Router) {
				r.Get("/", handlers.ListSystemUsers(db))
				r.With(admin).Post("/", handlers.CreateSystemUser(db))
				r.Route("/{userID}", func(r chi.Router) {
					r.Get("/", handlers.GetSystemUser(db))
					r.With(admin).Patch("/", handlers.PatchSystemUser(db))
					r.With(admin).Delete("/", handlers.DeleteSystemUser(db))
				})
			})

			r.Route("/settings", func(r chi.Router) {
				r.With(admin).Get("/", handlers.GetSettings(db))
				r.With(admin).Put("/", handlers.UpdateSettings(db))
				r.With(admin).Post("/test", handlers.TestNotification(db))
				r.With(admin).Get("/retention-preview", handlers.RetentionPreview(db))
			})

			r.Route("/sessions", func(r chi.Router) {
				r.Get("/", handlers.ListSessions(db))

				r.Route("/{sessionID}", func(r chi.Router) {
					// ac'yi (agent client) TerminateSession'a geçir
					r.With(operator).Delete("/", handlers.TerminateSession(db, ac))
					r.Get("/replay", handlers.GetSessionReplay(db))
					r.Get("/asciicast", handlers.ExportSessionAsciicast(db))
					r.Get("/commands", handlers.GetSessionCommands(db))
					r.Get("/meta", handlers.GetSessionMeta(db))
				})
			})
		})
	})

	// Ajan→sunucu mTLS: ajanlar TLS el sıkışmasında CA tarafından imzalı
	// istemci sertifikası sunar; sunduklarında zincir doğrulanır. Tarayıcılar
	// (admin arayüzü) istemci sertifikası sunmadığından VerifyClientCertIfGiven
	// kullanılır — sertifika varsa doğrulanır, yoksa el sıkışma devam eder ve
	// kimlik doğrulama katmanı (AdminAuth/AgentAuth) devreye girer.
	caPEM, caReadErr := os.ReadFile(caCertFile)
	if caReadErr != nil {
		log.Fatalf("FATAL: mTLS için CA sertifikası okunamadı (%s): %v", caCertFile, caReadErr)
	}
	clientCAPool := x509.NewCertPool()
	if !clientCAPool.AppendCertsFromPEM(caPEM) {
		log.Fatalf("FATAL: CA sertifikası ayrıştırılamadı (%s)", caCertFile)
	}

	srv := &http.Server{
		Addr:    ":" + serverPort,
		Handler: r,
		TLSConfig: &tls.Config{
			ClientAuth: tls.VerifyClientCertIfGiven,
			ClientCAs:  clientCAPool,
			MinVersion: tls.VersionTLS12,
		},
	}

	fmt.Printf("🚀 Guardian Server (Chi) https://localhost:%s adresinde GÜVENLİ modda başlatılıyor...\n", serverPort)
	err = srv.ListenAndServeTLS(certFile, keyFile)
	if err != nil {
		log.Fatalf("Güvenli (TLS) sunucu başlatılamadı: %v", err)
	}
}

// parseCORSOrigins, virgülle ayrılmış origin listesini ayrıştırır; boş
// girişleri atar. Boş dönerse çağıran taraf joker'e düşer.
func parseCORSOrigins(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if o := strings.TrimSpace(part); o != "" {
			out = append(out, o)
		}
	}
	return out
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
