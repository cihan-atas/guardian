// guardian/guardian-server/services/parser_service_test.go

package services

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("sqlmock oluşturulurken hata: %s", err)
	}
	return db, mock
}

func TestParseSessionEvents(t *testing.T) {
	sessionID := 1
	now := time.Now()

	// Sorgu metinleri burada tanımlı, bu sorun değil.
	metaQuery := `SELECT s.id, s.username, s.start_time, s.end_time, s.status, sv.hostname, sv.ip_address FROM sessions s JOIN servers sv ON s.server_id = sv.id WHERE s.id = $1`
	eventsQuery := `SELECT event_type, data, event_time FROM session_events WHERE session_id = $1 ORDER BY event_time ASC, id ASC`

	testCases := []struct {
		name          string
		setupMock     func(mock sqlmock.Sqlmock)
		expectedCmds  int
		expectedError string
		checkResult   func(t *testing.T, details *SessionDetails)
	}{
		{
			name: "Başarılı - Tek bir komut ve çıktısı",
			setupMock: func(mock sqlmock.Sqlmock) {
				// DÜZELTME: metaRows'u doğrudan bu testin içinde oluşturuyoruz.
				metaRows := sqlmock.NewRows([]string{"id", "username", "start_time", "end_time", "status", "hostname", "ip_address"}).
					AddRow(sessionID, "testuser", now, nil, "active", "test-server", "127.0.0.1")

				mock.ExpectQuery(metaQuery).
					WithArgs(sessionID).
					WillReturnRows(metaRows)

				eventRows := sqlmock.NewRows([]string{"event_type", "data", "event_time"}).
					AddRow("input", []byte("ls -la\r"), now.Add(1*time.Second)).
					AddRow("output", []byte("total 0\r\n"), now.Add(2*time.Second)).
					AddRow("output", []byte("drwxr-xr-x .\r\n"), now.Add(3*time.Second))

				mock.ExpectQuery(eventsQuery).
					WithArgs(sessionID).
					WillReturnRows(eventRows)
			},
			expectedCmds: 1,
			checkResult: func(t *testing.T, details *SessionDetails) {
				assert.Equal(t, "ls -la", details.Commands[0].Command)
				assert.Contains(t, details.Commands[0].Output, "total 0")
			},
		},
		{
			name: "Başarılı - ANSI escape kodları içeren çıktı",
			setupMock: func(mock sqlmock.Sqlmock) {
				// DÜZELTME: metaRows'u doğrudan bu testin içinde oluşturuyoruz.
				metaRows := sqlmock.NewRows([]string{"id", "username", "start_time", "end_time", "status", "hostname", "ip_address"}).
					AddRow(sessionID, "testuser", now, nil, "active", "test-server", "127.0.0.1")

				mock.ExpectQuery(metaQuery).WithArgs(sessionID).WillReturnRows(metaRows)

				eventRows := sqlmock.NewRows([]string{"event_type", "data", "event_time"}).
					AddRow("input", []byte("grep test\r"), now.Add(1*time.Second)).
					AddRow("output", []byte("this is a \x1b[31mtest\x1b[0m string\r\n"), now.Add(2*time.Second))

				mock.ExpectQuery(eventsQuery).WithArgs(sessionID).WillReturnRows(eventRows)
			},
			expectedCmds: 1,
			checkResult: func(t *testing.T, details *SessionDetails) {
				assert.Equal(t, "grep test", details.Commands[0].Command)
				assert.Equal(t, "this is a test string", details.Commands[0].Output)
			},
		},
		{
			name: "Başarılı - Backspace ile düzeltilen komut doğru yeniden oluşturulur",
			setupMock: func(mock sqlmock.Sqlmock) {
				metaRows := sqlmock.NewRows([]string{"id", "username", "start_time", "end_time", "status", "hostname", "ip_address"}).
					AddRow(sessionID, "testuser", now, nil, "active", "test-server", "127.0.0.1")

				mock.ExpectQuery(metaQuery).WithArgs(sessionID).WillReturnRows(metaRows)

				// Kullanıcı "lss" yazıp iki kez backspace yapıyor, sonra "s -la" yazıp Enter'a basıyor.
				eventRows := sqlmock.NewRows([]string{"event_type", "data", "event_time"}).
					AddRow("input", []byte("lss\x7f\x7fs -la\r"), now.Add(1*time.Second)).
					AddRow("output", []byte("total 0\r\n"), now.Add(2*time.Second))

				mock.ExpectQuery(eventsQuery).WithArgs(sessionID).WillReturnRows(eventRows)
			},
			expectedCmds: 1,
			checkResult: func(t *testing.T, details *SessionDetails) {
				assert.Equal(t, "ls -la", details.Commands[0].Command)
			},
		},
		{
			name: "Hata - Meta veri sorgusu başarısız",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(metaQuery).
					WithArgs(sessionID).
					WillReturnError(fmt.Errorf("veritabanı hatası"))
			},
			expectedError: "oturum metaverisi alınamadı: veritabanı hatası",
		},
		{
			name: "Başarılı - Hiç event yok",
			setupMock: func(mock sqlmock.Sqlmock) {
				// DÜZELTME: metaRows'u doğrudan bu testin içinde oluşturuyoruz.
				metaRows := sqlmock.NewRows([]string{"id", "username", "start_time", "end_time", "status", "hostname", "ip_address"}).
					AddRow(sessionID, "testuser", now, nil, "active", "test-server", "127.0.0.1")

				mock.ExpectQuery(metaQuery).WithArgs(sessionID).WillReturnRows(metaRows)
				eventRows := sqlmock.NewRows([]string{"event_type", "data", "event_time"})
				mock.ExpectQuery(eventsQuery).WithArgs(sessionID).WillReturnRows(eventRows)
			},
			expectedCmds: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := newMockDB(t)
			defer db.Close()

			tc.setupMock(mock)

			details, err := ParseSessionEvents(db, sessionID)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, details)
				assert.Len(t, details.Commands, tc.expectedCmds)
				if tc.checkResult != nil && details != nil {
					tc.checkResult(t, details)
				}
			}

			assert.NoError(t, mock.ExpectationsWereMet(), "Veritabanı beklentileri karşılanmadı")
		})
	}
}
