# Guardian — İlerleme Durumu

> Bu dosya, projede yapılan çalışmaları ve bilinen eksikleri takip etmek için tutulur. Son güncelleme: 2026-07-11.

## Proje Özeti

Guardian, geleneksel kalıcı `authorized_keys` yerine **Just-in-Time (JIT) ve denetlenebilir SSH erişimi** sağlayan açık kaynaklı bir PAM (Privileged Access Management) aracı. 4 bileşenden oluşuyor: **guardian-server** (Go API/scheduler/DB), **guardian-agent** (hedef sunucularda çalışan Go ajanı), **guardian-cli** (Go CLI), **guardian-ui** (Angular web arayüzü).

---

## ✅ Yapılanlar

### 1. Güvenlik sertleştirmesi (auth)
- **Canlı oturum WebSocket endpoint'i auth'suzdu** (`/ws/sessions/{id}`, `role=viewer` query param'ıyla herkes izleyebiliyor/sahte event enjekte edebiliyordu). İki ayrı route'a bölündü:
  - `/api/agent/ws/sessions/{id}` → `AgentAuth` (header Bearer token)
  - `/api/ws/sessions/{id}` → yeni `AdminWSAuth` middleware'i (query token, sadece tarayıcı WS handshake kısıtlaması nedeniyle istisna)
- Token karşılaştırmaları `!=` yerine `crypto/subtle.ConstantTimeCompare` ile yapılıyor (timing attack koruması) — hem `AgentAuth` hem `AdminAuth` hem `AdminWSAuth`.
- Admin REST endpoint'lerinde URL query param üzerinden token kabulü kaldırıldı; sadece `Authorization: Bearer` header'ı kabul ediliyor.
- Token'ları açık metin loglayan debug satırları (`agent_auth_middleware.go`, `guardian-agent/main.go`) kaldırıldı.

### 2. Terminal/oturum kaydı düzeltmeleri
- **Komut ayrıştırma mantığı** (`parser_service.go`) düzeltildi: komut sınırı artık kullanıcının gerçek Enter'ına (**input** akışındaki `\r`/`\n`) göre belirleniyor, önceden **output**'ta herhangi bir `\r` görülünce (satır sarma, prompt redraw, tab-completion, history gezinme dahil) yanlışlıkla komut kesiliyordu. Backspace/delete artık gerçekten önceki karakteri siliyor (önceden sessizce atılıyordu, komutları bozuyordu).
- **Terminal boyutu uyuşmazlığı** (canlı izleme + replay'de "bir ekrandan fazla olunca imleç en üste sıçrıyor, ekran kayıyor" sorunu) düzeltildi: agent artık PTY boyutunu (`cols`/`rows`) sunucuya bildiriyor, `sessions` tablosunda saklanıyor, UI xterm.js terminalini kayıtlı boyuta göre oluşturuyor (konteynerine göre dinamik "fit" etmek yerine).

### 3. Docker Compose desteği
- `guardian-server` + `guardian-ui` (nginx) + `db` için Dockerfile'lar ve `docker-compose.yml` eklendi. `guardian-agent` bilinçli olarak systemd'de bırakıldı (host'un SSH/kullanıcı alanına geniş erişim gerektirdiği için container'a alınması güvenlik modeliyle çelişir).
- `docs/docker-setup.md` yeni kurulum rehberi eklendi.
- Test sırasında bulunup düzeltilen 2 gerçek hata:
  - `schema.sql`'deki PG17-only `SET transaction_timeout = 0;` satırı Docker'ın init script'inde (`ON_ERROR_STOP=1`) tüm şema oluşturmayı sessizce durduruyordu → kaldırıldı.
  - nginx `proxy_pass`'i literal hostname kullanınca `guardian-server` restart olup IP değişince 502 veriyordu → Docker DNS resolver + değişken kullanılarak çalışma-anında çözüm yapılacak şekilde düzeltildi, restart testiyle doğrulandı.
- Tüm stack gerçekten build edilip self-signed test sertifikalarıyla uçtan uca test edildi (şema init, seed data, auth, UI statik + `/api/` proxy).

### 4. Canlı izleme + replay yeniden tasarımı (2026-07-09/11)
- **"Ekran kararıyor" hatasının kök nedeni bulundu:** xterm.css component `styleUrls` ile yüklenince Angular scope'luyor, xterm'in runtime'da oluşturduğu DOM'a stiller uygulanmıyordu (`.xterm-viewport` `position:static` kalıyordu). Çözüm: xterm.css global `styles.scss`'e taşındı.
- Paylaşılan `TerminalViewerComponent` (arama, yazı boyutu, tam ekran, log indirme, "CANLIYA DÖN" takip modu).
- Replay medya oynatıcı değil **komut gezgini**: ←/→ ile komuttan komuta atlama, komut akışı paneli, riskli komut vurgusu.
- Canlı ekranda çift karakter (`ll`→`llll`) ve base64 çifte kodlama düzeltildi; `restrict,pty` ile PTY tahsisi (ok tuşları/tab) düzeltildi.

### 5. "Oturum aktif takılı kalıyor" düzeltmesi (agent)
- Kullanıcı terminali `exit` demeden kapatınca oturum sonsuza dek `active` kalıyordu. Agent artık `SIGHUP` yakalıyor ve stdin EOF'ta oturumu kapatıyor → `endSessionOnServer("ended")` çalışıyor. Sunucudaki zombi dedektörü (15 sn heartbeat'siz → `lost_contact`) yedek katman olarak duruyor.

### 6. Dashboard modernizasyonu + komut analitiği
- KPI kartları, envanter şeridi, **Aktif Erişim Pencereleri** (JIT geri sayım + ilerleme çubuğu), Son Oturumlar paneli.
- **Komut Analitiği**: tam komut satırları, baz komuta göre gruplu drill-down, sunucu filtresi, arama, riskli komut vurgusu (`/api/dashboard/command-stats`).
- Parser'da SS3 (`ESC O A/B/C/D`) escape temizliği — komut istatistiklerindeki "OAOA" çöpleri bitti (birim testli).

### 7. Anahtar yasaklama (key ban)
- `key_bans` tablosu, `BanKey/UnbanKey` servisi, yasaklı anahtarın aktif kuralları otomatik iptal, scheduler yasaklı anahtarın bekleyen kurallarını aktifleştirmiyor.
- UI: süre ön ayarlı (30dk/1s/24s/7g/özel) + gerekçeli yasaklama modalı; keys/sessions/live ekranlarından erişilebilir.

### 8. Liste ekranları yenilemesi + arama altyapısı (2026-07-11)
- 5 liste endpoint'ine `?search=` (ILIKE), sessions'a `?status=` filtresi; `ListPublicKeys`'e LATERAL JOIN ile aktif yasak bilgisi (UI'daki N+1 kaldırıldı).
- Paylaşılan UI kiti (`shared/ui/`): pagination, status-badge (Türkçe durumlar), copy-button, confirm-dialog, ban-dialog — tüm `window.confirm`/`prompt` kullanımları kaldırıldı.
- Oturumlar (durum çipleri + süre kolonu), Kurallar (kalan süre ilerleme çubuğu), Sunucular (IP kopyala), Kullanıcılar (NullString sızıntısı temizliği), Anahtarlar (public key görüntüleme modalı + parmak izi kopyala).

---

## 🗺️ Yol Haritası / Planlanan Özellikler

### Devam eden (şu an çalışılıyor)
1. **Gerçek zamanlı riskli komut alarmı** — sunucu tarafı canlı oturumda riskli kalıp tespiti (`rm -rf`, `curl | sh`, `passwd`…), dashboard'a canlı uyarı, opsiyonel otomatik aksiyon (oturumu kes / anahtarı yasakla).
2. **Bildirimler (Slack / e-posta / webhook)** — yeni oturum, bağlantı kopması, riskli komut, ban olaylarında dışa bildirim.

### Sırada
3. **Onay akışı (approval workflow)** — kullanıcı erişim talep eder, admin tek tıkla onaylar/reddeder; self-service JIT.
4. **Admin hesapları + roller (RBAC)** — tek statik token yerine çoklu admin (TOTP/parola), `viewer/operator/admin` rolleri; audit'te gerçek kimlik.
5. **Agent sağlık göstergesi** — Sunucular sayfasında agent çevrimiçi/çevrimdışı durumu (heartbeat/ping tabanlı).
6. **Tam Audit Log sayfası** — filtreli/aranabilir denetim kaydı ekranı (menüye 6. kalem; veri ve servis zaten mevcut).
7. **Global komut arama** — tüm oturumlarda komut arama ("kim `chmod 777` çalıştırdı?") + sonuçtan replay'e ilgili komuta derin link.
8. **Windows agent desteği** — Windows OpenSSH `authorized_keys` + ConPTY uyarlaması (projenin başındaki çapraz platform hedefi).
9. **Kayıt saklama politikası** — `session_events` için otomatik temizlik/arşiv zamanlayıcısı (örn. 90 gün).
10. **Replay'i asciicast dışa aktarma** — kayıtları asciinema formatında indirme (paylaşım/delil).

---

## ⚠️ Kalan Eksikler / Bilinen Riskler

### Güvenlik (öncelikli)
1. **Secret token, hedef sunuculardaki `authorized_keys` dosyasına açık metin yazılıyor** (`environment="GUARDIAN_SECRET_TOKEN=..."` direktifiyle, `file_manager.go`). Dosyaya erişen biri agent API'sini taklit edebilir. Kalıcı çözüm: agent'ın proxy modunda sunucuya kimlik doğrulamasını paylaşımlı token yerine zaten var olan mTLS sertifikasıyla (`agent.crt`/`agent.key`) yapması — orta ölçekli bir refactor, henüz yapılmadı.
2. **Geniş CORS + credentials** (`AllowedOrigins: []string{"https://*","http://*"}` + `AllowCredentials: true`) — tarayıcı güvenlik modelini büyük ölçüde etkisizleştiriyor. Gerçek UI origin'ine daraltılmalı.
3. `dashboard_handler.go`'daki elle set edilmiş `Access-Control-Allow-Origin: *` satırları global CORS middleware ile çakışıyor, temizlenmeli.

### Dayanıklılık
4. **Agent tarafında oturum süresi zorlaması hiç çalışmıyor** — `getRuleValidity()` her zaman `"not implemented"` hatası döndürüyor, client-side timeout goroutine'i hiç başlamıyor. Süre kontrolü tamamen sunucudaki scheduler'ın "terminate" komutuna bağlı (tek katmanlı savunma).
5. Scheduler'ın agent'lara gönderdiği komutlar "best-effort" — agent geçici ulaşılamazsa retry/backoff yok, DB durumu ile gerçek `authorized_keys` durumu kalıcı olarak senkron dışı kalabilir.
6. Audit logları asenkron yazılıyor (`go func`) — sunucu çökerse bazı audit kayıtları kaybolabilir.
7. Admin'in gerçek terminal boyutu değiştiğinde (SIGWINCH) bu, aktif SSH oturumuna forward edilmiyor (`session.WindowChange` kullanılmıyor) — oturum başında sabitlenen boyut oturum boyunca sabit kalıyor.

### Test kapsamı
8. `guardian-cli`: hiç test yok.
9. `guardian-agent`: sadece `file_manager_test.go` var; SSH/proxy/WebSocket akışları test edilmemiş.
10. `guardian-ui`: 13 `.spec.ts` dosyası var ama hepsi Angular'ın varsayılan iskelet testleri, gerçek mantık test edilmiyor.
11. `guardian-server`: `auth_middleware.go`, `agent_auth_middleware.go`, `websocket_handler.go`, `hub/hub.go` için test yok — özellikle bu oturumda yapılan auth değişikliklerinin regresyona uğramadığını garanti edecek testler eksik.

### Diğer
12. `generate-certs.sh` betiğinde interaktif akışta prompt sırasıyla ilgili şüpheli bir davranış gözlemlendi (test sırasında non-interactive girdi verilince CA/Server alanları beklenmedik şekilde karıştı) — kök neden araştırılmadı, betik elle/interaktif kullanıldığında sorun olmayabilir ama doğrulanmadı.
13. Mevcut (bu değişiklikten önce) deploy edilmiş sunucularda `sessions` tablosuna `cols`/`rows` kolonlarının manuel eklenmesi gerekiyor:
    ```sql
    ALTER TABLE sessions ADD COLUMN cols integer, ADD COLUMN rows integer;
    ```
