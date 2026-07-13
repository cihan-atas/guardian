package hub

import (
	"testing"
	"time"
)

// waitForRegister, verilen client oturuma kaydedilene kadar (kısa timeout'la)
// bekler. Register kanalı işlendikten sonra map güncellemesi kilit altında
// yapıldığından küçük bir yarış penceresi vardır; bu yardımcı onu kapatır.
func waitForRegister(t *testing.T, h *Hub, sessionID int, c *Client) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		h.mu.Lock()
		_, ok := h.sessions[sessionID][c]
		h.mu.Unlock()
		if ok {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("client zamanında kaydedilmedi")
}

func newTestClient(sessionID int) *Client {
	return &Client{sessionID: sessionID, send: make(chan []byte, 4)}
}

func TestHub_BroadcastToMatchingSession(t *testing.T) {
	h := NewHub()
	go h.Run()

	c := newTestClient(42)
	h.Register <- c
	waitForRegister(t, h, 42, c)

	h.Broadcast <- &BroadcastMessage{SessionID: 42, Data: []byte("merhaba")}

	select {
	case msg := <-c.send:
		if string(msg) != "merhaba" {
			t.Fatalf("beklenen 'merhaba', alınan: %q", string(msg))
		}
	case <-time.After(time.Second):
		t.Fatal("eşleşen oturuma yayın alınamadı")
	}
}

func TestHub_BroadcastIgnoresOtherSession(t *testing.T) {
	h := NewHub()
	go h.Run()

	c := newTestClient(1)
	h.Register <- c
	waitForRegister(t, h, 1, c)

	// Farklı bir oturuma yayın → bu client almamalı.
	h.Broadcast <- &BroadcastMessage{SessionID: 999, Data: []byte("baska")}

	select {
	case msg := <-c.send:
		t.Fatalf("başka oturumun yayını alınmamalıydı, alınan: %q", string(msg))
	case <-time.After(150 * time.Millisecond):
		// beklenen: mesaj gelmedi.
	}
}

func TestHub_UnregisterStopsDelivery(t *testing.T) {
	h := NewHub()
	go h.Run()

	c := newTestClient(7)
	h.Register <- c
	waitForRegister(t, h, 7, c)

	h.Unregister <- c
	// Kayıt düşene kadar bekle.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		h.mu.Lock()
		_, stillThere := h.sessions[7][c]
		h.mu.Unlock()
		if !stillThere {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}

	h.Broadcast <- &BroadcastMessage{SessionID: 7, Data: []byte("sonra")}
	select {
	case <-c.send:
		t.Fatal("kayıttan düşen client'a yayın gitmemeliydi")
	case <-time.After(150 * time.Millisecond):
		// beklenen: teslimat yok.
	}
}
