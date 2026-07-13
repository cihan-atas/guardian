package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

// TestWebsocketWriter_Write, websocketWriter'ın yazdığı verinin karşı tarafa
// beklenen JSON yapısında ({type, data}) ulaştığını doğrular. Gerçek bir SSH
// oturumu gerektirmez; yalnızca bellek içi bir WebSocket bağlantısı kullanır.
func TestWebsocketWriter_Write(t *testing.T) {
	type wsMessage struct {
		Type string `json:"type"`
		Data []byte `json:"data"`
	}

	received := make(chan wsMessage, 1)
	upgrader := websocket.Upgrader{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade başarısız: %v", err)
			return
		}
		defer conn.Close()
		var msg wsMessage
		if err := conn.ReadJSON(&msg); err != nil {
			t.Errorf("mesaj okunamadı: %v", err)
			return
		}
		received <- msg
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket bağlantısı kurulamadı: %v", err)
	}
	defer conn.Close()

	writer := &websocketWriter{conn: conn, eventType: "output"}
	payload := []byte("merhaba dünya")
	n, err := writer.Write(payload)
	if err != nil {
		t.Fatalf("Write hata döndürdü: %v", err)
	}
	if n != len(payload) {
		t.Errorf("Write %d byte döndürmeliydi, alınan: %d", len(payload), n)
	}

	got := <-received
	if got.Type != "output" {
		t.Errorf("mesaj tipi 'output' olmalıydı, alınan: %q", got.Type)
	}
	if string(got.Data) != string(payload) {
		t.Errorf("veri eşleşmiyor: %q != %q", got.Data, payload)
	}
}
