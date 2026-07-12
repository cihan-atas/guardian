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

### 9. Gerçek zamanlı riskli komut alarmı + bildirimler (2026-07-11)
- **Sunucu tarafı canlı tespit:** agent WS'inden gelen `input` olayları komut sınırına (Enter) kadar biriktirilip `services.DetectRisky` ile denetleniyor (`risky_command_service.go`). Kurallar `high`/`critical` ciddiyetli (sudo, chmod 777, curl|sh, /etc/shadow… → high; rm -rf /, mkfs, dd of=/dev/, fork bomb, shutdown… → critical). Birim testli.
- **Alert akışı** (`alert_service.go`): eşleşme → `alerts` tablosuna kayıt (başlangıçta `CREATE TABLE IF NOT EXISTS` ile otomatik migration) + o oturumu canlı izleyen tarayıcılara WS `{type:"alert"}` yayını + dış bildirim + kritikte opsiyonel oto-aksiyon.
- **Oto-aksiyon:** `GUARDIAN_RISKY_AUTOACTION` = `none` (varsayılan) | `terminate` | `ban`; yalnızca `critical` eşleşmelerde uygulanır.
- **Bildirimler** (`notification_service.go`): webhook (Slack/Discord/Teams uyumlu JSON) ve/veya SMTP e-posta, fire-and-forget. Tetikleyiciler: riskli komut, yeni oturum (`StartSession`), anahtar yasağı (`BanPublicKey`). Hepsi opsiyonel env ile açılır; hiçbiri yoksa yalnızca uygulama içi uyarı.
- **UI:** canlı oturum ekranında yüzen uyarı paneli + toast; dashboard'da "Son Riskli Komut Uyarıları" paneli (`/api/dashboard/alerts`).
- **Yeni env değişkenleri** (server.conf): `GUARDIAN_WEBHOOK_URL`, `GUARDIAN_SMTP_HOST/PORT/USER/PASS/FROM`, `GUARDIAN_ALERT_EMAIL_TO`, `GUARDIAN_RISKY_AUTOACTION`.
- **UI'dan yönetim (Ayarlar sayfası):** Bu ayarlar artık `settings` (key-value) tablosunda tutulur ve `/settings` ekranından düzenlenir; env değerleri yalnızca ilk açılışta (tablo boşsa) tohumlanır. Kayıttan sonra notifier + alerting canlı yeniden yüklenir (restart yok). Endpoint'ler: `GET/PUT /api/settings`, `POST /api/settings/test`. SMTP parolası write-only (GET'te dönmez, boş bırakılırsa korunur). "Test Bildirimi Gönder" butonuyla kanal denenebilir.

### 10. RBAC — Yönetici hesapları + roller (2026-07-11)
- **Statik `GUARDIAN_ADMIN_TOKEN` tamamen kaldırıldı.** Giriş artık kullanıcı adı + parola (bcrypt) ile; başarılı girişte DB'de (`admin_sessions`) sunucu-taraflı, revoke edilebilir bir oturum token'ı üretilir (12 saat TTL). Tüm istekler `Authorization: Bearer <oturum-token>` kullanır (`auth_service.go`, `auth_handler.go`).
- **Roller:** `viewer` (salt-okunur) < `operator` (izleme + erişim talebi + oturum sonlandırma) < `admin` (tam yetki). `RequireRole` middleware'i rota bazında rütbe kontrolü yapar; mutasyon uçları admin'e, oturum sonlandırma + talep açma operatöre kapılıdır.
- **İlk yönetici bootstrap:** `admin_users` boşsa `GUARDIAN_ADMIN_USERNAME`/`GUARDIAN_ADMIN_PASSWORD`'dan oluşturulur; parola boşsa rastgele geçici parola üretilip log'a yazılır. Tablolar açılışta otomatik oluşturulur (auto-migration).
- **Audit gerçek kimliğe bağlandı:** kayıtlar artık token prefix'i yerine gerçek kullanıcı adını yazar (önceki cross-package context anahtarı uyuşmazlığı da giderildi). LOGIN dahil yeni audit aksiyonları eklendi.
- **Kendini-kilitleme koruması:** son aktif admin'in rolü düşürülemez/silinemez/devre dışı bırakılamaz; kullanıcı kendini silemez/devre dışı bırakamaz. Parola değişimi ve devre dışı bırakma ilgili oturumları düşürür. Süresi dolmuş oturumlar scheduler'da temizlenir.
- **UI:** kullanıcı adı+parola giriş formu; auth.service rol+oturum saklar (`hasRole`); `roleGuard` ve rol kapılı sidebar; **Yöneticiler** ekranı (CRUD + rol + devre dışı + parola). 403 (yetersiz rol) artık kullanıcıyı çıkışa zorlamaz (yalnızca 401).
- **CLI:** statik token yerine `GUARDIAN_ADMIN_USERNAME`/`GUARDIAN_ADMIN_PASSWORD` ile `/auth/login` üzerinden oturum token'ı alır.

### 11. Onay akışı (approval workflow) (2026-07-11)
- Operatör erişim talebi açar (sunucu + anahtar + sistem kullanıcısı + zaman aralığı + gerekçe) → `access_rules`'ta `awaiting_approval` durumunda kaydedilir; scheduler etkinleştirmez.
- Admin **onaylar** → durum `pending` olur, `approved_by`/`decided_at` işlenir; agent'ta kullanıcı doğrulanır + yasak kontrol edilir; scheduler `valid_from` geldiğinde kuralı otomatik etkinleştirir. Admin **reddeder** → `rejected` + gerekçe.
- `access_rules`'a `requested_by`/`approved_by`/`request_reason`/`reject_reason`/`decided_at` kolonları eklendi (auto-migration). Kurallar listesi `awaiting_approval`/`rejected` kayıtlarını gizler (onlar Erişim Talepleri ekranında).
- Endpoint'ler: `GET /api/access-requests` (viewer+), `POST /api/access-requests` (operator+), `POST /api/access-requests/{id}/approve|reject` (admin). UI: **Erişim Talepleri** ekranı (durum sekmeleri, talep formu, onay/red).

---

### 12. Agent sağlık göstergesi (2026-07-11)
- `agentclient.Ping` agent'ın kimlik doğrulamasız `/status` ucuna kısa (3 sn) timeout'lu GET atar. `GET /api/servers/health` tüm sunucuların agent'larını paralel pingler; her biri için `online` + `latency_ms` döner (`AgentPinger` arayüzü, `AgentCommunicator`'ı ve test mock'larını bozmadan).
- UI: Sunucular sayfasına "Agent Durumu" kolonu (çevrimiçi nabız + gecikme / çevrimdışı / kontrol ediliyor); 20 sn'de bir otomatik tazelenir.

### 13. Tam Audit Log sayfası (2026-07-11)
- `GET /api/audit-logs` filtreli + sayfalı denetim kaydı (arama: admin_ref/target_type ILIKE; `action`, `status` filtreleri); yalnızca admin.
- UI: **Denetim Kaydı** ekranı (arama kutusu, aksiyon + sonuç filtreleri, sayfalama, Türkçe aksiyon etiketleri, hata mesajı gösterimi). Sidebar'a admin'e özel "Denetim Kaydı" kalemi. RBAC ile audit artık gerçek kullanıcı adını kaydettiği için ekran anlamlı.

### 14. Global komut arama (2026-07-11)
- `GET /api/commands/search?q=&limit=` en son 1000 oturumu tarayıp komut metninde alt dize araması yapar (`services.SearchCommands`). Her eşleşme oturum metadata'sı (kullanıcı, sunucu, zaman, durum) + oturum içi **komut indeksi** döner.
- Komut indeksi `ParseSessionEvents`'in komut dizisiyle birebir hizalı (aynı ayrıştırma/temizleme mantığı) → sonuçtan **replay'e derin link**: `/replay/:id?cmd=<index>` replay'i doğrudan o komutta açar.
- UI: **Komut Arama** ekranı (debounce'lu arama, riskli komut vurgusu, durum rozeti, "Replay" derin linki). Sidebar'a "Komut Arama" kalemi (tüm roller). Replay `?cmd=` query param'ını okuyup başlangıç karesini ilgili komuta ayarlar.

### 15. UI'dan agent kurulumu (manuel script + SSH oto-kurulum) (2026-07-12)
- **PKI/enrollment:** Sunucu, CA özel anahtarını (`TLS_CA_KEY_FILE`, docker'da `/etc/guardian/certs/ca.key`) yükleyip agent CSR'larını `crypto/x509` ile imzalar (`pki_service.go`). CA anahtarı yoksa oto-kurulum devre dışı kalır, sunucu normal çalışır. Kayıt token'ları `agent_enroll_tokens` (30 dk, tek kullanımlık; auto-migration + scheduler temizliği).
- **Uçlar (kayıt token'ıyla, admin oturumu gerekmez):** `GET /api/agent/install.sh`, `POST /api/agent/enroll` (CSR imzala), `GET /api/agent/ca.crt`, `GET /api/agent/binary`. Admin: `POST /api/servers/{id}/enroll-token`, `POST /api/servers/{id}/ssh-install`.
- **Manuel kurulum:** UI'da sunucu seçilir → tek satırlık `curl … | sudo bash` komutu üretilir. Script hedefte kullanıcı/grup, dizin, agent SSH anahtarı, TLS key+CSR üretir; sertifikayı sunucuya imzalatır; `ca.crt` + binary indirir; `agent.conf` + systemd yazar ve servisi başlatır. sshd ve kullanıcı-`authorized_keys` adımları güvenlik gereği otomatik yapılmaz, çıktıda listelenir.
- **SSH oto-kurulum:** Sunucu, verilen SSH bilgisiyle (parola/özel anahtar) hedefe `x/crypto/ssh` ile bağlanıp aynı komutu uzaktan çalıştırır; çıktı UI'ya döner. Kimlik bilgileri saklanmaz.
- **UI:** admin'e özel **Agent Kurulumu** ekranı (sunucu seç, Manuel/SSH sekmeleri, komut kopyalama, çıktı). Yeni env: `TLS_CA_KEY_FILE`, `GUARDIAN_AGENT_BINARY_PATH`, `GUARDIAN_PUBLIC_URL` (.env.example + docker-compose belgelendi).

### 16. Sertifika süre-sonu göstergesi (2026-07-12)
- `GET /api/certificates` (admin): CA (`ca.crt`) ve Guardian sunucu sertifikasını (`server.crt`) yerel dosyalardan; her sunucunun **agent sertifikasını** TLS el sıkışmasıyla (`agentclient.PeerCertificate`, `InsecureSkipVerify` ile süresi dolmuş olsa da okunur) — konu/bitiş/kalan-gün bilgisiyle döndürür (`pki_service.ReadCertInfo`).
- UI: admin'e özel **Sertifikalar** ekranı — CA + server kartları, agent cert tablosu; kalan güne göre renk (>30 yeşil, ≤30 sarı, ≤7/negatif kırmızı).
- **Yenileme + süre seçimi:** İmzalama artık süre parametreli (`SignAgentCSR(..., validityDays)`); enroll token'ına `validity_days` kolonu eklendi. Süre seçenekleri 1/2/5/10 yıl. **Server cert yenileme:** `POST /api/certificates/server/renew` mevcut anahtar + SAN'ları koruyarak seçilen süreyle yeniden imzalar (eski cert `.bak`'lanır), etkin olması için sunucu yeniden başlatılır. **Agent cert yenileme:** Sertifikalar/Agent Kurulumu ekranlarından seçilen süreyle enroll komutu üretilir; hedefte çalıştırılınca çalışan agent kesintisiz güncellenir. CA yenileme bilinçli olarak UI dışında (dağıtım gerektirir; ekranda not).

---

### 17. Windows agent desteği (2026-07-12)
- Agent zaten cross-platform (yerel PTY yok; `localhost:22`'ye SSH ile bağlanıp PTY'yi OpenSSH sshd'den ister → ConPTY'yi sshd sağlar). Windows'a temiz cross-compile oluyor.
- **Agent kodu:** config'i dosyadan okuma (`GUARDIAN_AGENT_CONFIG` / OS varsayılanı — Windows'ta systemd `EnvironmentFile` yok); `getAuthorizedKeysPath` Windows yönetici hesapları için `%ProgramData%\ssh\administrators_authorized_keys` (kimlerin admin olduğu `GUARDIAN_WINDOWS_ADMIN_USERS` ile).
- **Sunucu:** `CA.IssueAgentCert` server-side anahtar+sertifika üretir (openssl'siz enroll — Windows için); `POST /api/agent/enroll-bundle` (key+crt+ca JSON); `GET /api/agent/install.ps1` (PowerShell kurulum); `GET /api/agent/binary?os=windows` (`GUARDIAN_AGENT_BINARY_PATH_WINDOWS`). enroll-token `os` parametresiyle Linux/Windows kurulum komutu üretir.
- **PowerShell kurulumu:** ProgramData dizinleri, SSH servis anahtarı, enroll-bundle ile sertifika, `agent.conf`, `guardian-agent.exe` indirme, `New-Service` ile Windows servisi + `GUARDIAN_AGENT_CONFIG` makine env'i.
- **UI:** Agent Kurulumu'na OS seçici; Windows'ta PowerShell tek satırlık komut; SSH oto-kurulum Linux'a özel (Windows'ta manuel not). `docs/agent-setup-windows.md` eklendi.
- Not: Linux uçtan uca test edildi; Windows tarafı gerçek bir Windows host'ta doğrulanmalı (bu ortamda runtime test yapılamadı).

---

## 🗺️ Yol Haritası / Planlanan Özellikler

### Sırada
9. **Kayıt saklama politikası** — `session_events` için otomatik temizlik/arşiv zamanlayıcısı (örn. 90 gün).
10. **Replay'i asciicast dışa aktarma** — kayıtları asciinema formatında indirme (paylaşım/delil).
11. **2FA (TOTP)** — RBAC üzerine opsiyonel iki adımlı doğrulama (login'de TOTP kodu).

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
