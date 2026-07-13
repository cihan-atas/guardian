# Guardian — İlerleme Durumu

> Bu dosya, projede yapılan çalışmaları ve bilinen eksikleri takip etmek için tutulur. Son güncelleme: 2026-07-13.

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

### 18. Kayıt saklama politikası (2026-07-12)
- Bitmiş oturumların hacimli olay/replay akışı (`session_events`) yapılandırılabilir süre sonra otomatik silinir; oturum özet satırları (kullanıcı/sunucu/süre/durum) audit için **korunur**. `0 = sınırsız` (temizlik kapalı).
- **Servis:** `services/retention_service.go` — `RetentionDays`, `CountPurgeableEvents` (önizleme), `PurgeOldSessionEvents` (bitmiş + `end_time` süresi geçmiş oturumların olayları), `RunRetention` (temizlik + `retention_last_run`/`retention_last_deleted` ayarlarına yazma). Süre `settings.retention_days`'te.
- **Scheduler:** ayrı seyrek döngü (`runRetentionLoop`, 12 saatte bir + açılışta bir kez).
- **API:** `GET /api/settings/retention-preview?days=N` (silinecek kayıt sayısı önizlemesi); `GET/PUT /api/settings` retention alanlarını taşır. Ayarlar kaydedilince temizlik hemen tetiklenir (best-effort goroutine).
- **UI:** Ayarlar'da "Kayıt Saklama Politikası" bölümü — süre ön ayarları (Sınırsız/30/90/180/365 gün) + özel gün girişi, canlı "kaç kayıt etkilenecek" önizlemesi (400ms debounce), son-temizlik bilgisi.

### 19. Replay'i asciicast dışa aktarma (2026-07-12)
- Oturum kayıtları asciinema **asciicast v2** (`.cast`) formatında indirilebilir (paylaşım/delil; asciinema player ile oynatılır).
- **API:** `GET /api/sessions/{id}/asciicast` — yalnızca `output` olayları, satır-satır **akış** (streaming; uzun oturumlar bellekte tutulmaz). İlk satır JSON başlık (version/width/height/timestamp/title=`kullanıcı@host`), sonraki satırlar `[zaman_ofseti, "o", "veri"]`. `Content-Disposition: attachment`.
- **UI:** Replay ekranı başlığında "Dışa aktar" butonu — Bearer auth'lu blob indirme (`guardian-session-<id>.cast`).

### 20. İki adımlı doğrulama — 2FA / TOTP (2026-07-12)
- Yönetici hesapları için opsiyonel TOTP (RFC 6238, SHA1/6 hane/30 sn) 2FA. Google Authenticator, Authy, 1Password vb. uyumlu.
- **TOTP servisi** (`services/totp_service.go`): yalnızca stdlib (crypto/hmac+sha1, base32) — yeni Go bağımlılığı yok. RFC 6238 test vektörleriyle doğrulandı (T=59→287082, T=1111111109→081804). ±1 pencere saat kayması toleransı, sabit zamanlı karşılaştırma.
- **Şema:** `admin_users`'a `totp_secret`/`totp_enabled` kolonları (EnsureAuthTables içinde `ADD COLUMN IF NOT EXISTS` migration).
- **Login akışı:** `Authenticate(..., totpCode)` — parola doğru + 2FA açıksa kod boşsa `ErrTOTPRequired` (login yanıtı `{totp_required:true}`), hatalıysa `ErrInvalidTOTP`. UI iki adımlı: parola → (gerekirse) 6 haneli kod ekranı.
- **API:** `POST /api/auth/2fa/setup` (gizli anahtar + otpauth URI), `/enable` (kod doğrula → etkinleştir), `/disable` (parola doğrula → kapat). `/auth/me` ve login yanıtı `totp_enabled` taşır. ENABLE_2FA/DISABLE_2FA audit'lenir.
- **UI:** yeni **Hesabım** sayfası (`features/account`, tüm rollere açık) — parola değiştirme + 2FA yönetimi. Kurulumda QR (istemcide `qrcode` ile üretilir, harici servis yok) + manuel anahtar. Sidebar kullanıcı kutusu bu sayfaya linklendi.
- Not: parola değiştirmede `VerifyPassword` doğrudan bcrypt kontrolü yapar (Authenticate'in 2FA yan etkisini tetiklemeden).

### 21. Ajan kimlik doğrulaması: paylaşımlı token → mTLS (2026-07-13)
- **Sorun:** `GUARDIAN_SECRET_TOKEN` hedef sunucunun `authorized_keys` dosyasına açık metin yazılıyordu (`environment=` direktifi); dosyayı okuyabilen biri ajan API'sini taklit edebiliyordu.
- **Çözüm:** Her iki yönde de kimlik doğrulaması **mTLS** (mevcut CA imzalı `agent.crt`/`server.crt`) ile yapılıyor; paylaşımlı token yalnızca eski kurulumlar için **geriye dönük yedek**.
  - **Ajan→sunucu:** Sunucu TLS dinleyicisi `ClientAuth: VerifyClientCertIfGiven` + `ClientCAs` (tarayıcıları bozmadan; admin arayüzü sertifika sunmaz). `AgentAuth` middleware'i geçerli istemci sertifikası varsa kabul eder, yoksa token yedeğine düşer. Ajan `createApiClient`/WS dialer artık istemci sertifikası (`agent.crt`/`agent.key`) sunuyor.
  - **Sunucu→ajan:** Ajan dinleyicisi de `VerifyClientCertIfGiven` + `ClientCAs` (sağlık `/status` sertifikasız erişilebilir kalır). `agentclient` sunucu sertifikasını (`server.crt`/`server.key`) istemci sertifikası olarak sunuyor; `agentclient.New(...)` imzasına `certFile`/`keyFile` eklendi.
- **En önemli düzeltme:** `file_manager.go` artık `GUARDIAN_SECRET_TOKEN`'ı `authorized_keys`'e **yazmıyor**; yalnızca gizli olmayan dosya yolları (cert/key/CA) geçiyor.
- **Uyumluluk:** `GUARDIAN_SECRET_TOKEN` hem sunucuda hem ajanda artık **opsiyonel** (yoksa yalnızca mTLS). Sertifikalar zaten hem `serverAuth` hem `clientAuth` EKU'ya sahip (Go ile üretilenler) veya EKU kısıtı yok (bootstrap) → mTLS iki yönde de çalışır. Not: `agent.conf` içindeki token (root-only 0600) zararsız yedek olarak kalır; tüm ajanlar yenilendikten sonra kaldırılabilir.
- **Test:** `handlers/agent_auth_middleware_test.go` — gerçek TLS el sıkışmasıyla: (1) mTLS istemci sertifikası kabul, (2) sertifikasız+token'sız 401, (3) token yedeği (doğru→200, yanlış→403). Ayrıca `TestMain` DB yokken artık paketi öldürmüyor (DB testleri `requireDB` ile atlanıyor), böylece DB'siz ortamda saf mantık testleri çalışabiliyor.

---

## 🗺️ Yol Haritası / Planlanan Özellikler

### Sırada — sertleştirme & dayanıklılık & test (öncelik sırası, 2026-07-13)
Ana yol haritası maddeleri tamamlandı. Sıradaki iş, "Kalan Eksikler"deki 11 başlığı sırayla kapatmak:

**Güvenlik:**
1. ~~**Shared token → mTLS.**~~ ✅ **Tamam (2026-07-13, madde #21).** İki yönde mTLS; token yalnızca yedek; `authorized_keys`'e token yazılmıyor.
2. ~~**CORS daraltma.**~~ ✅ **Tamam (2026-07-13).** `GUARDIAN_CORS_ORIGINS` (virgüllü) ile origin daraltılabiliyor; boşsa joker + uyarı.
3. ~~**Elle CORS başlığı temizliği.**~~ ✅ **Tamam (2026-07-13).** `command_handler.go`'daki elle `Access-Control-Allow-Origin: *` kaldırıldı.

**Dayanıklılık:**
4. ~~**Agent süre zorlaması.**~~ ✅ **Tamam (2026-07-13).** Kırık `getRuleValidity` kaldırıldı; oturum başlarken sunucudan dönen `valid_until` `enforceSessionTimeout`'a taşınıyor → client-side ikinci savunma katmanı çalışıyor.
5. ~~**Scheduler retry/backoff.**~~ ✅ **Tamam (2026-07-13).** `agentclient.sendCommand` taşıma hatalarında üstel backoff'la 3 kez deniyor (yalnızca yanıt alınamayan = uygulanmamış komutlar; çift uygulama riski yok). Testli.
6. ~~**Audit dayanıklılığı.**~~ ✅ **Tamam (2026-07-13).** Audit yazımı senkron hale getirildi (handler dönmeden kayıt kalıcı) + geçici hatalara karşı kısa retry.
7. ~~**SIGWINCH forward.**~~ ✅ **Tamam (2026-07-13).** SIGWINCH yakalanıp `session.WindowChange` ile aktif oturuma iletiliyor (Unix; Windows'ta işlemsiz, build-tag'li).

**Test kapsamı:**
8. ~~`guardian-cli` testleri (şu an 0).~~ ✅ **Tamam (2026-07-13).** `client/client_test.go` — Login (token saklama/boş token/HTTP hata), Bearer auth başlığı, ListServers ayrıştırma, CreateServer hata, DeleteServer 204.
9. ~~`guardian-agent` SSH/proxy/WebSocket testleri.~~ ✅ **Tamam (2026-07-13).** `auth_middleware_test.go` (mTLS/token fallback, gerçek TLS el sıkışması) + `config_file_test.go` (agent.conf ayrıştırma, getEnv). SSH/proxy uçtan uca kısmı canlı ortam gerektirdiğinden kapsam dışı.
10. `guardian-ui` gerçek mantık testleri (iskelet spec'ler yerine).
11. `guardian-server` auth/websocket/hub middleware testleri.

---

## ⚠️ Kalan Eksikler / Bilinen Riskler

### Güvenlik (öncelikli)
1. ~~**Secret token, `authorized_keys`'e açık metin yazılıyor**~~ — ✅ **ÇÖZÜLDÜ (2026-07-13, madde #21).** Ajan↔sunucu kimlik doğrulaması mTLS'e taşındı; token `authorized_keys`'e artık yazılmıyor, yalnızca eski kurulumlar için opsiyonel yedek.
2. ~~**Geniş CORS + credentials**~~ — ✅ **ÇÖZÜLDÜ (2026-07-13).** `GUARDIAN_CORS_ORIGINS` env'i ile origin listesi daraltılabiliyor (boşsa geriye dönük joker + üretim uyarısı).
3. ~~Elle set edilmiş `Access-Control-Allow-Origin: *`~~ — ✅ **ÇÖZÜLDÜ (2026-07-13).** `command_handler.go`'daki satır kaldırıldı (global CORS middleware yeterli). Not: eski notta `dashboard_handler.go` yazıyordu; gerçekte `command_handler.go`'daydı.

### Dayanıklılık
4. ~~**Agent tarafında oturum süresi zorlaması hiç çalışmıyor**~~ — ✅ **ÇÖZÜLDÜ (2026-07-13).** `getRuleValidity` stub'ı kaldırıldı; oturum başlatılırken sunucudan dönen `valid_until` `parseFlagsAndStartSession` üzerinden `enforceSessionTimeout`'a aktarılıyor. Artık scheduler'ın "terminate"ine ek olarak ajan da kendi zamanlayıcısıyla süreyi zorluyor (iki katmanlı savunma).
5. ~~Scheduler'ın agent'lara gönderdiği komutlar "best-effort"~~ — ✅ **ÇÖZÜLDÜ (2026-07-13).** `agentclient.sendCommand` taşıma hatalarında üstel backoff'la (300ms/600ms/1.2s) 3 kez deniyor. Yalnızca yanıt alınamayan (= ajana ulaşmamış, uygulanmamış) komutlar tekrarlanır; HTTP yanıtı (200 dışı dahil) alınırsa tekrar edilmez → add/remove-key çift uygulama riski yok. `client_test.go` ile testli.
6. ~~Audit logları asenkron yazılıyor (`go func`)~~ — ✅ **ÇÖZÜLDÜ (2026-07-13).** `services.Record` artık senkron yazıyor (denetlenen işlemi yapan handler dönmeden kayıt kalıcı olur) ve geçici DB hatalarına karşı 3 kez deniyor. Not: aynı-transaction bütünlüğü (mutasyon + audit tek tx) ayrı ve daha büyük bir iyileştirme olarak bırakıldı.
7. ~~Admin'in gerçek terminal boyutu değiştiğinde (SIGWINCH) forward edilmiyor~~ — ✅ **ÇÖZÜLDÜ (2026-07-13).** `watchWindowSize` (build-tag'li: `resize_unix.go`) SIGWINCH'i yakalayıp `session.WindowChange(h, w)` ile aktif oturuma iletiyor; Windows'ta işlemsiz (`resize_windows.go`). Not: DB'ye kaydedilen `cols`/`rows` replay tutarlılığı için oturum başındaki değer kalır; canlı boyut değişimi yalnızca etkileşimli oturuma uygulanır.

### Test kapsamı
8. ~~`guardian-cli`: hiç test yok.~~ — ✅ **ÇÖZÜLDÜ (2026-07-13).** `client/client_test.go` eklendi (httptest ile: Login token akışı, `sendRequest` Bearer başlığı, liste ayrıştırma, oluştur/sil durum kodları).
9. ~~`guardian-agent`: sadece `file_manager_test.go` var~~ — ✅ **KISMEN ÇÖZÜLDÜ (2026-07-13).** `auth_middleware_test.go` (mTLS kabul / sertifikasız+token'sız 401 / token yedeği 200-403, gerçek TLS handshake) ve `config_file_test.go` (agent.conf KEY=VALUE ayrıştırma, mevcut env koruması, `getEnv` fallback) eklendi. Not: SSH/proxy/WebSocket uçtan uca akışı canlı sshd + sunucu gerektirdiğinden hâlâ birim testsiz.
10. `guardian-ui`: 13 `.spec.ts` dosyası var ama hepsi Angular'ın varsayılan iskelet testleri, gerçek mantık test edilmiyor.
11. `guardian-server`: `auth_middleware.go`, `agent_auth_middleware.go`, `websocket_handler.go`, `hub/hub.go` için test yok — özellikle bu oturumda yapılan auth değişikliklerinin regresyona uğramadığını garanti edecek testler eksik.

### Diğer
12. `generate-certs.sh` betiğinde interaktif akışta prompt sırasıyla ilgili şüpheli bir davranış gözlemlendi (test sırasında non-interactive girdi verilince CA/Server alanları beklenmedik şekilde karıştı) — kök neden araştırılmadı, betik elle/interaktif kullanıldığında sorun olmayabilir ama doğrulanmadı.
13. Mevcut (bu değişiklikten önce) deploy edilmiş sunucularda `sessions` tablosuna `cols`/`rows` kolonlarının manuel eklenmesi gerekiyor:
    ```sql
    ALTER TABLE sessions ADD COLUMN cols integer, ADD COLUMN rows integer;
    ```

### Windows'ta guardian-server (sunucu) çalıştırma — durum ve eksikler (2026-07-13)

**Özet:** guardian-server binary'si Windows'a **temiz cross-compile oluyor** (`GOOS=windows GOARCH=amd64 go build` sorunsuz, ~19 MB `guardian-server.exe`). Kodun kendisi büyük ölçüde OS-bağımsız: tüm dosya yolları env değişkenlerinden okunuyor (`TLS_CA_FILE`, `TLS_CERT_FILE`, `TLS_KEY_FILE`, `TLS_CA_KEY_FILE`, `GUARDIAN_AGENT_BINARY_PATH[_WINDOWS]` …), runtime'da Linux'a özgü `syscall`/sinyal/sabit yol kullanımı **yok**. Yani teknik olarak Windows'ta çalışabilir; **ama şu an resmi kurulum/işletim desteği yok** — aşağıdaki boşluklar elle doldurulmalı. (Agent tarafı #8'de zaten Windows'a hazırlandı; buradaki eksikler yalnızca **sunucu**yu Windows'ta koşmakla ilgilidir.)

Eksikler / elle yapılması gerekenler:

1. **Sertifika bootstrap (en önemli boşluk).** İlk kurulumdaki CA anahtarı + CA sertifikası + ilk sunucu sertifikası hâlâ `generate-certs.sh` (bash + `openssl`) ile üretiliyor. Bu betik Windows'ta doğrudan çalışmaz; şu seçenekler gerekir:
   - **Git Bash / WSL / MSYS2** altında `openssl` ile aynı betiği koşmak, ya da
   - Windows için `openssl.exe` ile aynı `openssl` komutlarını (CA `genpkey`+`req -x509`, server `genpkey`+`req`+`x509 -req` SAN `server.ext` ile) elle çalıştırmak, ya da
   - Betiği PowerShell'e veya bir Go alt-komutuna (`guardian-server gen-certs`) port etmek — **henüz yapılmadı.**
   - Not: Bootstrap sonrası **agent sertifikaları ve sunucu-cert yenileme openssl'siz** (server-side `IssueAgentCert`/`SignAgentCSR`/`RenewServerCert`, madde #15–#16); yani openssl ihtiyacı yalnızca **ilk** CA+server cert üretimiyle sınırlı. CA yenileme de UI dışı (dağıtım gerektirir).

2. **Windows servisi yok.** Sunucu için yalnızca Linux systemd unit'i var (deploy `/usr/local/bin/guardian-server`, kullanıcı `guardian:guardian`). Windows'ta `guardian-server.exe`'yi servis olarak koşmak için `New-Service` / `sc.exe` / `nssm` ile bir servis tanımı + otomatik başlatma **elle** kurulmalı (agent'ta yaptığımız `New-Service` deseni örnek alınabilir, ama sunucu için hazır script yok).

3. **Config yükleme farkı.** Sunucu binary'sinin **config-dosyası okuyucusu yok** — tüm ayarları yalnızca ortam değişkenlerinden alır. Linux'ta bunları systemd `EnvironmentFile=/etc/guardian/server.conf` sağlıyor. Windows'ta systemd olmadığından değişkenlerin **makine/servis düzeyinde env** olarak (veya servis sarmalayıcı üzerinden) verilmesi gerekir. Agent'a #8'de eklenen `config_file.go` benzeri bir yükleyici sunucuya **eklenmedi**; istenirse `GUARDIAN_SERVER_CONFIG` ile dosyadan okuma kolayca portlanabilir.

4. **Veritabanı.** PostgreSQL Windows'ta Docker Desktop veya yerel PostgreSQL ile çalışır; sorun kodda değil, sadece **kurulumu script'lenmedi** (Linux tarafında docker-compose / harici Postgres var).

5. **UI / reverse proxy.** UI dağıtımı Linux nginx'e göre (`/var/www/guardian-ui`, `systemctl reload nginx`). Windows'ta nginx-for-Windows veya IIS reverse-proxy ile aynısı yapılabilir ama **script/doküman yok**.

6. **Deploy dokümanı yok.** Tüm deploy adımları (systemd, `/usr/local/bin`, `/var/www`, `/etc/guardian`) Linux varsayıyor; Windows sunucu için ayrı bir kurulum rehberi yazılmadı.

**Sonuç:** "Windows'ta hem agent hem server" → **agent: destekli**; **server: derlenir ve çalışır ama kurulum/işletim tarafı Linux'a göre; Windows'ta koşmak için yukarıdaki 6 boşluk elle doldurulmalı.** Üretim önerisi şimdilik: **server Linux'ta, agent'lar Linux + Windows karışık.** Windows'ta sunucu resmi desteği istenirse; öncelik sırası (1) cert bootstrap'ı openssl'siz Go alt-komutuna taşımak, (2) sunucu için Windows servis kurulumu, (3) `GUARDIAN_SERVER_CONFIG` dosya yükleyici.
