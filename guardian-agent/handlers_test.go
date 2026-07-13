package main

import (
	"net/http"
	"net/http/httptest"
	"os/user"
	"runtime"
	"strings"
	"testing"
)

// doHandler, verilen handler'ı body ile çağırıp yanıt kaydını döndüren yardımcı.
func doHandler(t *testing.T, h http.HandlerFunc, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func TestHandleStatus(t *testing.T) {
	rec := doHandler(t, handleStatus, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("200 bekleniyordu, alınan: %d", rec.Code)
	}
	if got := rec.Body.String(); got != "OK" {
		t.Errorf("gövde 'OK' olmalıydı, alınan: %q", got)
	}
}

func TestHandleValidateUser_InvalidBody(t *testing.T) {
	rec := doHandler(t, handleValidateUser, "{bozuk json")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("geçersiz JSON'da 400 bekleniyordu, alınan: %d", rec.Code)
	}
}

func TestHandleValidateUser_UnknownUser(t *testing.T) {
	rec := doHandler(t, handleValidateUser, `{"username":"guardian_yok_kullanici_zzz"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("bilinmeyen kullanıcıda 404 bekleniyordu, alınan: %d", rec.Code)
	}
}

func TestHandleValidateUser_ExistingUser(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix kullanıcı veritabanı varsayımı")
	}
	cur, err := user.Current()
	if err != nil {
		t.Skipf("mevcut kullanıcı okunamadı: %v", err)
	}
	rec := doHandler(t, handleValidateUser, `{"username":"`+cur.Username+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("mevcut kullanıcıda 200 bekleniyordu, alınan: %d", rec.Code)
	}
}

func TestHandleTerminateSession_InvalidBody(t *testing.T) {
	rec := doHandler(t, handleTerminateSession, "{bozuk")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("geçersiz JSON'da 400 bekleniyordu, alınan: %d", rec.Code)
	}
}

func TestHandleTerminateSession_MissingID(t *testing.T) {
	rec := doHandler(t, handleTerminateSession, `{"session_id":0}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("eksik session_id'de 400 bekleniyordu, alınan: %d", rec.Code)
	}
}

func TestHandleAddKey_InvalidBody(t *testing.T) {
	rec := doHandler(t, handleAddKey, "bozuk")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("geçersiz JSON'da 400 bekleniyordu, alınan: %d", rec.Code)
	}
}

func TestHandleRemoveKey_InvalidBody(t *testing.T) {
	rec := doHandler(t, handleRemoveKey, "bozuk")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("geçersiz JSON'da 400 bekleniyordu, alınan: %d", rec.Code)
	}
}
