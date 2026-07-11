// guardian/guardian-server/scheduler/scheduler_test.go

package scheduler

import (
	"database/sql"
	"sync"
	"testing"

	"guardian.com/server/agentclient"
	"guardian.com/server/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAgentClient, scheduler testleri için AgentCommunicator interface'ini taklit eder.
type MockAgentClient struct {
	mock.Mock
	wg *sync.WaitGroup
}

// NewMockAgentClient, WaitGroup'u alarak yeni bir mock client oluşturur.
func NewMockAgentClient(wg *sync.WaitGroup) *MockAgentClient {
	return &MockAgentClient{wg: wg}
}

func (m *MockAgentClient) SendKeyCommand(ip, action string, payload models.KeyPayload) error {
	args := m.Called(ip, action, payload)
	return args.Error(0)
}

func (m *MockAgentClient) TerminateSession(ip string, sessionID int) error {
	// Bu metod asenkron çağrıldığı için, bittiğinde WaitGroup'a haber verir.
	if m.wg != nil {
		defer m.wg.Done()
	}
	args := m.Called(ip, sessionID)
	return args.Error(0)
}

// ValidateUser, handler'lar için gerekli olsa da, bu mock'un interface'i tam
// karşılaması için burada da tanımlanmalıdır.
func (m *MockAgentClient) ValidateUser(ip, username string) error {
	args := m.Called(ip, username)
	return args.Error(0)
}

// Bu satır, MockAgentClient'ın AgentCommunicator interface'ini tam olarak
// uyguladığını derleme zamanında garanti eder.
var _ agentclient.AgentCommunicator = (*MockAgentClient)(nil)

// newMockDB, testler için sahte bir veritabanı bağlantısı oluşturur.
func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("sqlmock oluşturulurken hata: %s", err)
	}
	return db, mock
}

// TestProcessPendingRules, zamanı gelmiş 'pending' kuralların 'active' durumuna geçip
// agent'a 'add' komutu gönderilip gönderilmediğini test eder.
func TestProcessPendingRules(t *testing.T) {
	db, mockDB := newMockDB(t)
	defer db.Close()

	mockAgent := NewMockAgentClient(nil) // Bu testte asenkron çağrı yok.
	ruleID := 101

	// Yasaklı anahtarların kuralları aktifleştirilmez (LEFT JOIN key_bans).
	mockDB.ExpectQuery("SELECT ar.id FROM access_rules ar LEFT JOIN key_bans kb ON kb.public_key_id = ar.public_key_id AND kb.banned_until > NOW() AT TIME ZONE 'utc' WHERE ar.status = $1 AND ar.valid_from <= NOW() AT TIME ZONE 'utc' AND kb.id IS NULL").
		WithArgs(statusPending).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(ruleID))

	mockDB.ExpectExec("UPDATE access_rules SET status = $1 WHERE id = ANY($2)").
		WithArgs(statusActive, pq.Array([]int{ruleID})).
		WillReturnResult(sqlmock.NewResult(1, 1))

	ruleDetailsRows := sqlmock.NewRows([]string{"id", "username", "ssh_public_key", "ip_address"}).
		AddRow(ruleID, "testuser", "ssh-rsa ...", "1.2.3.4")
	mockDB.ExpectQuery("SELECT r.id, su.username, pk.ssh_public_key, s.ip_address FROM access_rules r JOIN servers s ON r.server_id = s.id JOIN public_keys pk ON r.public_key_id = pk.id JOIN system_users su ON r.system_user_id = su.id WHERE r.id = ANY($1)").
		WithArgs(pq.Array([]int{ruleID})).
		WillReturnRows(ruleDetailsRows)

	mockAgent.On("SendKeyCommand", "1.2.3.4", "add", mock.AnythingOfType("models.KeyPayload")).Return(nil)

	err := processPendingRules(db, mockAgent)

	assert.NoError(t, err)
	mockAgent.AssertExpectations(t)
	assert.NoError(t, mockDB.ExpectationsWereMet())
}

// TestProcessActiveRules, süresi dolmuş 'active' kuralların 'expired' durumuna geçip
// ilgili oturumları sonlandırıp agent'a 'remove' komutu gönderip göndermediğini test eder.
func TestProcessActiveRules(t *testing.T) {
	db, mockDB := newMockDB(t)
	defer db.Close()

	var wg sync.WaitGroup
	mockAgent := NewMockAgentClient(&wg)

	ruleID := 202
	sessionID := 303
	agentIP := "4.5.6.7"

	mockDB.ExpectQuery("SELECT id FROM access_rules WHERE status = $1 AND valid_until <= NOW() AT TIME ZONE 'utc'").
		WithArgs(statusActive).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(ruleID))

	activeSessionRows := sqlmock.NewRows([]string{"id", "ip_address"}).AddRow(sessionID, agentIP)
	mockDB.ExpectQuery("SELECT s.id, sv.ip_address FROM sessions s JOIN servers sv ON s.server_id = sv.id WHERE s.rule_id = ANY($1) AND s.status = 'active'").
		WithArgs(pq.Array([]int{ruleID})).
		WillReturnRows(activeSessionRows)

	sessionStatusRows := sqlmock.NewRows([]string{"ip_address", "status"}).AddRow(agentIP, "active")
	mockDB.ExpectQuery("SELECT sv.ip_address, s.status FROM sessions s JOIN servers sv ON s.server_id = sv.id WHERE s.id = $1").
		WithArgs(sessionID).
		WillReturnRows(sessionStatusRows)

	mockDB.ExpectExec("UPDATE sessions SET status = $1, end_time = NOW() AT TIME ZONE 'utc' WHERE id = $2").
		WithArgs("terminated_by_expiry", sessionID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	wg.Add(1) // 1 asenkron görevin (TerminateSession) tamamlanmasını bekliyoruz.
	mockAgent.On("TerminateSession", agentIP, sessionID).Return(nil)

	mockDB.ExpectExec("UPDATE access_rules SET status = $1 WHERE id = ANY($2)").
		WithArgs(statusExpired, pq.Array([]int{ruleID})).
		WillReturnResult(sqlmock.NewResult(1, 1))

	ruleDetailsRows := sqlmock.NewRows([]string{"id", "username", "ssh_public_key", "ip_address"}).
		AddRow(ruleID, "expireduser", "ssh-rsa ...", agentIP)
	mockDB.ExpectQuery("SELECT r.id, su.username, pk.ssh_public_key, s.ip_address FROM access_rules r JOIN servers s ON r.server_id = s.id JOIN public_keys pk ON r.public_key_id = pk.id JOIN system_users su ON r.system_user_id = su.id WHERE r.id = ANY($1)").
		WithArgs(pq.Array([]int{ruleID})).
		WillReturnRows(ruleDetailsRows)

	mockAgent.On("SendKeyCommand", agentIP, "remove", mock.AnythingOfType("models.KeyPayload")).Return(nil)

	err := processActiveRules(db, mockAgent)
	assert.NoError(t, err)

	wg.Wait() // Asenkron görevin bitmesini bekle.

	mockAgent.AssertExpectations(t)
	assert.NoError(t, mockDB.ExpectationsWereMet())
}

// TestProcessZombieSessions, uzun süre heartbeat göndermeyen oturumların durumunu güncelleyip
// güncellemediğini test eder.
func TestProcessZombieSessions(t *testing.T) {
	db, mockDB := newMockDB(t)
	defer db.Close()
	zombieSessionID := 404

	zombieRows := sqlmock.NewRows([]string{"id"}).AddRow(zombieSessionID)
	mockDB.ExpectQuery("SELECT id FROM sessions WHERE status = 'active' AND last_heartbeat < NOW() AT TIME ZONE 'utc' - INTERVAL '15 seconds'").
		WillReturnRows(zombieRows)

	mockDB.ExpectExec("UPDATE sessions SET status = 'lost_contact', end_time = NOW() AT TIME ZONE 'utc' WHERE id = ANY($1)").
		WithArgs(pq.Array([]int{zombieSessionID})).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := processZombieSessions(db)

	assert.NoError(t, err)
	assert.NoError(t, mockDB.ExpectationsWereMet())
}
