// guardian/guardian-server/scheduler/scheduler.go (GÜNCELLENMİŞ HALİ)

package scheduler

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/lib/pq"

	"guardian.com/server/agentclient"
	"guardian.com/server/models"
	"guardian.com/server/services"
)

const (
	checkInterval = 10 * time.Second
	statusPending = "pending"
	statusActive  = "active"
	statusExpired = "expired"
)

// DEĞİŞİKLİK: *agentclient.Client yerine agentclient.AgentCommunicator kullanıyoruz.
func Start(db *sql.DB, agentClient agentclient.AgentCommunicator) {
	log.Println("⏰ Zamanlayıcı başlatılıyor...")
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	go runChecks(db, agentClient)

	for range ticker.C {
		go runChecks(db, agentClient) // go yu kaldırabilirsin !!!
	}
}

// DEĞİŞİKLİK: *agentclient.Client yerine agentclient.AgentCommunicator kullanıyoruz.
func runChecks(db *sql.DB, agentClient agentclient.AgentCommunicator) {
	log.Println("⏳ Zamanlayıcı tetiklendi, kural durumları kontrol ediliyor...")

	if err := processPendingRules(db, agentClient); err != nil {
		log.Printf("[ERROR] Bekleyen kurallar işlenirken hata oluştu: %v", err)
	}

	if err := processActiveRules(db, agentClient); err != nil {
		log.Printf("[ERROR] Aktif kurallar işlenirken hata oluştu: %v", err)
	}

	if err := processZombieSessions(db); err != nil {
		log.Printf("[ERROR] Zombi oturumlar temizlenirken hata oluştu: %v", err)
	}

	log.Println("✅ Kural kontrolü tamamlandı.")
}

// DEĞİŞİKLİK: *agentclient.Client yerine agentclient.AgentCommunicator kullanıyoruz.
func processPendingRules(db *sql.DB, agentClient agentclient.AgentCommunicator) error {
	// key_bans'ta hâlâ geçerli bir yasağı olan anahtarlara bağlı kurallar
	// etkinleştirilmez; yasak süresi dolunca bir sonraki taramada normal
	// şekilde etkinleşirler.
	query := `
		SELECT ar.id FROM access_rules ar
		LEFT JOIN key_bans kb ON kb.public_key_id = ar.public_key_id AND kb.banned_until > NOW() AT TIME ZONE 'utc'
		WHERE ar.status = $1 AND ar.valid_from <= NOW() AT TIME ZONE 'utc' AND kb.id IS NULL`
	ruleIDs, err := getRuleIDsByQuery(db, query, statusPending)
	if err != nil {
		return fmt.Errorf("bekleyen kural ID'leri alınamadı: %w", err)
	}

	if len(ruleIDs) > 0 {
		log.Printf("    -> %d kural '%s' durumuna geçiriliyor: %v", len(ruleIDs), statusActive, ruleIDs)
		if err := updateRuleStatus(db, ruleIDs, statusActive); err != nil {
			return fmt.Errorf("kurallar '%s' olarak güncellenemedi: %w", statusActive, err)
		}
		if err := notifyAgents(db, ruleIDs, "add", agentClient); err != nil {
			log.Printf("[WARN] Agent'lar bilgilendirilirken hata oluştu (add): %v", err)
		}
	}
	return nil
}

// DEĞİŞİKLİK: *agentclient.Client yerine agentclient.AgentCommunicator kullanıyoruz.
func processActiveRules(db *sql.DB, agentClient agentclient.AgentCommunicator) error {
	query := "SELECT id FROM access_rules WHERE status = $1 AND valid_until <= NOW() AT TIME ZONE 'utc'"
	ruleIDs, err := getRuleIDsByQuery(db, query, statusActive)
	if err != nil {
		return fmt.Errorf("aktif kural ID'leri alınamadı: %w", err)
	}

	if len(ruleIDs) > 0 {
		log.Printf("    -> %d kuralın süresi doldu, ilişkili aktif oturumlar sonlandırılıyor: %v", len(ruleIDs), ruleIDs)
		terminateActiveSessionsForExpiredRules(db, agentClient, ruleIDs)

		log.Printf("    -> %d kural '%s' durumuna geçiriliyor: %v", len(ruleIDs), statusExpired, ruleIDs)
		if err := updateRuleStatus(db, ruleIDs, statusExpired); err != nil {
			return fmt.Errorf("kurallar '%s' olarak güncellenemedi: %w", statusExpired, err)
		}

		if err := notifyAgents(db, ruleIDs, "remove", agentClient); err != nil {
			log.Printf("[WARN] Agent'lar bilgilendirilirken hata oluştu (remove): %v", err)
		}
	}
	return nil
}

func processZombieSessions(db *sql.DB) error {
	query := `
		SELECT id FROM sessions
		WHERE status = 'active' 
		  AND last_heartbeat < NOW() AT TIME ZONE 'utc' - INTERVAL '15 seconds'
	`
	rows, err := db.Query(query)
	if err != nil {
		return fmt.Errorf("zombi oturumlar sorgulanamadı: %w", err)
	}
	defer rows.Close()

	var zombieSessionIDs []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			log.Printf("[WARN] Zombi oturum ID'si okunurken hata: %v", err)
			continue
		}
		zombieSessionIDs = append(zombieSessionIDs, id)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("zombi oturum satırları okunurken hata: %w", err)
	}

	if len(zombieSessionIDs) > 0 {
		log.Printf("🧟‍♂ %d adet zombi oturum tespit edildi, durumları güncelleniyor: %v", len(zombieSessionIDs), zombieSessionIDs)
		updateQuery := `
			UPDATE sessions 
			SET status = 'lost_contact', end_time = NOW() AT TIME ZONE 'utc'
			WHERE id = ANY($1)
		`
		if _, err := db.Exec(updateQuery, pq.Array(zombieSessionIDs)); err != nil {
			return fmt.Errorf("zombi oturum durumları güncellenemedi: %w", err)
		}
	}
	return nil
}

func getRuleIDsByQuery(db *sql.DB, query string, args ...interface{}) ([]int, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func updateRuleStatus(db *sql.DB, ids []int, newStatus string) error {
	_, err := db.Exec("UPDATE access_rules SET status = $1 WHERE id = ANY($2)", newStatus, pq.Array(ids))
	return err
}

// DEĞİŞİKLİK: *agentclient.Client yerine agentclient.AgentCommunicator kullanıyoruz.
func notifyAgents(db *sql.DB, ruleIDs []int, action string, agentClient agentclient.AgentCommunicator) error {
	query := `
			 SELECT
					 r.id,
					 su.username,
					 pk.ssh_public_key,
					 s.ip_address
			 FROM access_rules r
			 JOIN servers s ON r.server_id = s.id
			 JOIN public_keys pk ON r.public_key_id = pk.id
			 JOIN system_users su ON r.system_user_id = su.id
			 WHERE r.id = ANY($1)`

	rows, err := db.Query(query, pq.Array(ruleIDs))
	if err != nil {
		return fmt.Errorf("agent'a gönderilecek kural bilgileri alınamadı: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var payload models.KeyPayload
		var ipAddress string

		if err := rows.Scan(&payload.RuleID, &payload.Username, &payload.SshPublicKey, &ipAddress); err != nil {
			log.Printf("[WARN] Kural verisi okunurken hata, bu kural atlanıyor: %v", err)
			continue
		}

		if err := agentClient.SendKeyCommand(ipAddress, action, payload); err != nil {
			log.Printf("[ERROR] Scheduler: Agent'a komut gönderilirken hata oluştu (Host: %s, Kural ID: %d): %v", ipAddress, payload.RuleID, err)
		}
	}
	return rows.Err()
}

// DEĞİŞİKLİK: *agentclient.Client yerine agentclient.AgentCommunicator kullanıyoruz.
func terminateActiveSessionsForExpiredRules(db *sql.DB, agentClient agentclient.AgentCommunicator, ruleIDs []int) {
	query := `
			 SELECT s.id, sv.ip_address
			 FROM sessions s
			 JOIN servers sv ON s.server_id = sv.id
			 WHERE s.rule_id = ANY($1) AND s.status = 'active'`

	rows, err := db.Query(query, pq.Array(ruleIDs))
	if err != nil {
		log.Printf("[ERROR] Süresi dolan kurallar için aktif oturumlar sorgulanırken hata: %v", err)
		return
	}
	defer rows.Close()
	var sessionsToTerminate []int
	for rows.Next() {
		var sessionID int
		var agentIP string // Bu değişkene artık ihtiyacımız yok ama Scan için gerekli.
		if err := rows.Scan(&sessionID, &agentIP); err != nil {
			log.Printf("[WARN] Aktif oturum verisi okunurken hata, bu oturum atlanıyor: %v", err)
			continue
		}
		sessionsToTerminate = append(sessionsToTerminate, sessionID)
	}

	if len(sessionsToTerminate) > 0 {
		log.Printf("Zamanlayıcı: Süresi dolan %d kurala ait %d oturum sonlandırılıyor...", len(ruleIDs), len(sessionsToTerminate))
		for _, sessionID := range sessionsToTerminate {
			if err := services.UpdateAndTerminateSession(db, agentClient, sessionID, "terminated_by_expiry", nil); err != nil {
				log.Printf("[ERROR] Zamanlayıcı oturum %d sonlandırılamadı: %v", sessionID, err)
			}
		}
	}
}
