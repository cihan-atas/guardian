package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

// Bu dosya, ajanın SSH proxy KAYIT VERİ YOLUNU uçtan uca test eder:
// gerçek bir SSH oturumu (in-process x/crypto/ssh sunucusu) + gerçek bir
// WebSocket sunucusu üzerinden üretim `setupPipes` fonksiyonu çalıştırılır ve
// (1) uzak shell çıktısının "output" olayı olarak, (2) istemci girişinin
// "input" olayı olarak sunucuya doğru (base64 çift-kodlama olmadan) aktığı
// doğrulanır. Ayrıca stdin EOF'ta oturumun kapandığı test edilir.
//
// Canlı sshd/sunucu GEREKTİRMEZ; tüm bileşenler in-process'tir. Böylece
// "canlı ortam" ihtiyacı olmadan gerçek transport (SSH + WS) üzerinde çalışır.

// wsFrame, kayıt WebSocket'ine ajanın yazdığı çerçeve. websocketWriter, Data'yı
// []byte olarak gönderir; JSON bunu base64 kodlar, sunucu []byte'a decode eder.
type wsFrame struct {
	Type string `json:"type"`
	Data []byte `json:"data"`
}

// startRecordingWS, ajanın bağlanacağı kayıt WS sunucusunu başlatır ve alınan
// çerçeveleri frames kanalına iletir.
func startRecordingWS(t *testing.T) (*httptest.Server, <-chan wsFrame) {
	t.Helper()
	frames := make(chan wsFrame, 64)
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			var f wsFrame
			if err := conn.ReadJSON(&f); err != nil {
				return
			}
			frames <- f
		}
	}))
	return srv, frames
}

// startEchoSSHServer, "shell" isteğine hazır banner + gelen her girdiyi
// "ECHO:" ön ekiyle geri yazan basit bir in-process SSH sunucusu başlatır.
func startEchoSSHServer(t *testing.T) net.Listener {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("host key üretilemedi: %v", err)
	}
	signer, err := ssh.NewSignerFromSigner(priv)
	if err != nil {
		t.Fatalf("signer oluşturulamadı: %v", err)
	}
	cfg := &ssh.ServerConfig{NoClientAuth: true}
	cfg.AddHostKey(signer)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("dinleyici açılamadı: %v", err)
	}

	go func() {
		nConn, err := ln.Accept()
		if err != nil {
			return
		}
		_, chans, reqs, err := ssh.NewServerConn(nConn, cfg)
		if err != nil {
			return
		}
		go ssh.DiscardRequests(reqs)
		for newCh := range chans {
			if newCh.ChannelType() != "session" {
				newCh.Reject(ssh.UnknownChannelType, "yalnızca session")
				continue
			}
			ch, requests, err := newCh.Accept()
			if err != nil {
				return
			}
			go func() {
				for req := range requests {
					switch req.Type {
					case "pty-req", "shell":
						req.Reply(true, nil)
						if req.Type == "shell" {
							go func() {
								ch.Write([]byte("GUARDIAN_E2E_READY\n"))
								buf := make([]byte, 1024)
								for {
									n, rerr := ch.Read(buf)
									if n > 0 {
										ch.Write(append([]byte("ECHO:"), buf[:n]...))
									}
									if rerr != nil {
										break
									}
								}
								ch.Close()
							}()
						}
					default:
						req.Reply(false, nil)
					}
				}
			}()
		}
	}()
	return ln
}

// collectFrames, gelen çerçeveleri type'a göre birleştirir; beklenen içerik
// (banner + echo + input) geldiğinde erken döner, en geç timeout'ta.
func collectFrames(frames <-chan wsFrame, timeout time.Duration) map[string]string {
	out := map[string]string{}
	deadline := time.After(timeout)
	for {
		select {
		case f := <-frames:
			out[f.Type] += string(f.Data)
			if strings.Contains(out["output"], "ECHO:merhaba-guardian") &&
				strings.Contains(out["output"], "GUARDIAN_E2E_READY") &&
				strings.Contains(out["input"], "merhaba-guardian") {
				return out
			}
		case <-deadline:
			return out
		}
	}
}

func TestE2E_ProxyRecordingPipeline(t *testing.T) {
	// 1. Kayıt WS sunucusu + in-process SSH sunucusu.
	wsSrv, frames := startRecordingWS(t)
	defer wsSrv.Close()
	sshLn := startEchoSSHServer(t)
	defer sshLn.Close()

	// 2. Ajanın kayıt WS bağlantısı (setupPipes buna yazar). Üretimdeki gibi
	//    thread-safe wsConn ile sarmalanır (output/input yazıcıları aynı conn'u
	//    paylaşır).
	wsURL := "ws" + strings.TrimPrefix(wsSrv.URL, "http")
	rawWS, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("kayıt WS'ine bağlanılamadı: %v", err)
	}
	ws := newWSConn(rawWS)
	defer ws.Close()

	// 3. Gerçek SSH istemci oturumu (in-process sunucuya).
	clientCfg := &ssh.ClientConfig{
		User:            "e2e",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}
	client, err := ssh.Dial("tcp", sshLn.Addr().String(), clientCfg)
	if err != nil {
		t.Fatalf("SSH sunucusuna bağlanılamadı: %v", err)
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("SSH oturumu açılamadı: %v", err)
	}

	// 4. os.Stdin/os.Stdout'u geçici pipe/devnull ile değiştir: üretim
	//    setupPipes bunları çağrı anında yakalar.
	origStdin, origStdout, origStderr := os.Stdin, os.Stdout, os.Stderr
	stdinR, stdinW, _ := os.Pipe()
	devnull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	os.Stdin, os.Stdout, os.Stderr = stdinR, devnull, devnull
	defer func() {
		os.Stdin, os.Stdout, os.Stderr = origStdin, origStdout, origStderr
		devnull.Close()
	}()

	// 5. ÜRETİM KODU: I/O + WS köprüsünü kur ve uzak shell'i başlat.
	setupPipes(session, ws, 80, 24)
	if err := session.Shell(); err != nil {
		t.Fatalf("uzak shell başlatılamadı: %v", err)
	}

	// 6. İstemci girişi yaz → stdinPipe + wsInputWriter'a akar; sunucu ECHO'lar.
	//    output (banner + echo) ve input yazıcıları aynı conn'a EŞZAMANLI yazar;
	//    thread-safe wsConn olmadan bu, gorilla'da bozuk çerçevelere yol açar.
	if _, err := stdinW.Write([]byte("merhaba-guardian\n")); err != nil {
		t.Fatalf("stdin yazılamadı: %v", err)
	}

	got := collectFrames(frames, 3*time.Second)

	// output: hazır banner + sunucunun ECHO çıktısı (base64 doğru decode edilmeli).
	if !strings.Contains(got["output"], "GUARDIAN_E2E_READY") {
		t.Errorf("output olayında hazır banner beklen(iyor)di; alınan: %q", got["output"])
	}
	if !strings.Contains(got["output"], "ECHO:merhaba-guardian") {
		t.Errorf("output olayında ECHO çıktısı beklen(iyor)di; alınan: %q", got["output"])
	}
	// input: istemci girişi ayrı bir olay olarak kaydedilmeli.
	if !strings.Contains(got["input"], "merhaba-guardian") {
		t.Errorf("input olayında istemci girişi beklen(iyor)di; alınan: %q", got["input"])
	}

	// 7. stdin EOF → setupPipes goroutine'i oturumu kapatmalı (aktif takılı
	//    kalma regresyonuna karşı). Kapatınca session.Wait() dönmeli.
	stdinW.Close()
	done := make(chan struct{})
	go func() { session.Wait(); close(done) }()
	select {
	case <-done:
		// beklenen: stdin kapanınca oturum sonlandı.
	case <-time.After(3 * time.Second):
		t.Error("stdin EOF sonrası oturum kapanmadı (aktif takılı kalma riski)")
	}
}
