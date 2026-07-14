package services

import (
	"context"
	"database/sql"
	"errors"
	"net/http/httptest"
	"testing"
)

// fakeExecer, RecordTx'in verilen execer (tx/db) üzerinden yazdığını ve
// argümanları doğru ilettiğini DB olmadan doğrulamak için sahte bir auditExecer.
type fakeExecer struct {
	query   string
	args    []interface{}
	calls   int
	execErr error
}

func (f *fakeExecer) Exec(query string, args ...interface{}) (sql.Result, error) {
	f.calls++
	f.query = query
	f.args = args
	return nil, f.execErr
}

func TestRecordTx_ForwardsArgsAndAdminRef(t *testing.T) {
	fe := &fakeExecer{}
	// Context'e kimlik yerleştir → admin_ref bu kullanıcı olmalı.
	ident := &AdminIdentity{ID: 7, Username: "alice", Role: RoleAdmin}
	req := httptest.NewRequest("POST", "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), AdminIdentityContextKey, ident))

	err := RecordTx(fe, req, AuditLog{
		Action:     ActionDeleteServer,
		TargetType: "server",
		TargetID:   42,
		Status:     "SUCCESS",
	})
	if err != nil {
		t.Fatalf("RecordTx beklenmedik hata: %v", err)
	}
	if fe.calls != 1 {
		t.Fatalf("Exec 1 kez çağrılmalıydı, alınan: %d", fe.calls)
	}
	if len(fe.args) != 6 {
		t.Fatalf("6 argüman bekleniyordu, alınan: %d", len(fe.args))
	}
	if fe.args[0] != "alice" {
		t.Errorf("admin_ref 'alice' olmalıydı, alınan: %v", fe.args[0])
	}
	// target_id sql.NullInt64{42, valid}
	if ni, ok := fe.args[3].(sql.NullInt64); !ok || !ni.Valid || ni.Int64 != 42 {
		t.Errorf("target_id 42/valid olmalıydı, alınan: %v", fe.args[3])
	}
}

func TestRecordTx_NoIdentityIsSystem(t *testing.T) {
	fe := &fakeExecer{}
	req := httptest.NewRequest("GET", "/", nil)
	if err := RecordTx(fe, req, AuditLog{Action: ActionLogin, Status: "SUCCESS"}); err != nil {
		t.Fatalf("beklenmedik hata: %v", err)
	}
	if fe.args[0] != "system" {
		t.Errorf("kimliksiz istekte admin_ref 'system' olmalıydı, alınan: %v", fe.args[0])
	}
}

func TestRecordTx_PropagatesExecError(t *testing.T) {
	// Audit yazımı başarısızsa hata dönmeli ki çağıran transaction'ı rollback etsin
	// (aynı-transaction bütünlüğü: mutasyon audit'siz commit edilmemeli).
	sentinel := errors.New("insert patladı")
	fe := &fakeExecer{execErr: sentinel}
	err := RecordTx(fe, httptest.NewRequest("GET", "/", nil), AuditLog{Action: ActionCreateKey})
	if !errors.Is(err, sentinel) {
		t.Fatalf("Exec hatası iletilmemeli, alınan: %v", err)
	}
}

// target_id sıfır olduğunda NULL yazılmalı (Valid=false).
func TestRecordTx_ZeroTargetIsNull(t *testing.T) {
	fe := &fakeExecer{}
	if err := RecordTx(fe, httptest.NewRequest("GET", "/", nil), AuditLog{Action: ActionUpdateSetting}); err != nil {
		t.Fatalf("beklenmedik hata: %v", err)
	}
	if ni, ok := fe.args[3].(sql.NullInt64); !ok || ni.Valid {
		t.Errorf("target_id 0 iken NULL (Valid=false) olmalıydı, alınan: %v", fe.args[3])
	}
}
