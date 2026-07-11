## 🐳 Guardian Docker Kurulumu

Bu rehber, Guardian'ın merkezi bileşenlerini (**guardian-server**, **guardian-ui**, **PostgreSQL**) Docker Compose ile tek komutla ayağa kaldırmayı anlatır. Bu, [Sunucu Kurulum Rehberi](./server-setup.md)'ndeki systemd tabanlı manuel kuruluma bir alternatiftir; ikisi aynı anda kullanılmaz.

> ℹ️ **`guardian-agent` bu stack'e dahil değildir.** Agent, yönetilen her hedef sunucuda `authorized_keys` dosyalarını değiştirmek, sistem kullanıcılarının home dizinlerine erişmek ve `localhost:22`'deki gerçek `sshd`'ye bağlanmak zorundadır. Bunu bir container içine almak host'un kullanıcı/SSH alanına geniş erişim (bind mount `/etc/passwd`, `/home`, host network vb.) gerektirir ve PAM aracının kendi güvenlik modeliyle çelişir. Bu yüzden agent, [Agent Kurulum Rehberi](./agent-setup.md)'ndeki gibi systemd ile, doğrudan hedef sunucuda çalıştırılmaya devam eder — Guardian Server Docker'da çalışsa bile.

### Ön Gereksinimler

* Docker ve Docker Compose (v2)
* `openssl` (sertifika oluşturma betiği için)

### Adım 1: TLS Sertifikalarını Oluşturma

Sertifikalar host üzerinde oluşturulur ve container'lara salt-okunur olarak bağlanır (mount edilir). Proje ana dizininde:

```bash
./generate-certs.sh
```

Detaylar için [Sertifika Oluşturma Rehberi](./generate-certs-usage.md)'ne bakın. Bu, proje ana dizininde bir `certs/` klasörü oluşturur (`ca.crt`, `server.crt`, `server.key`, ...) — `docker-compose.yml` bu klasörü doğrudan kullanır, başka bir yere kopyalamanıza gerek yoktur.

> 🔐 `certs/ca.key` (kök özel anahtar) container'lara **mount edilmez** ve hiçbir zaman `certs/` dışına, güvenli/çevrimdışı bir yere taşınmalıdır.

### Adım 2: Ortam Değişkenlerini Yapılandırma

```bash
cp .env.example .env
nano .env   # POSTGRES_PASSWORD, GUARDIAN_SECRET_TOKEN, GUARDIAN_ADMIN_USERNAME/PASSWORD değerlerini değiştirin
```

`.env` dosyası `.gitignore` ile hariç tutulur; sırlarınızı asla commit etmeyin.

### Adım 3: Stack'i Başlatma

```bash
docker compose build
docker compose up -d
```

İlk başlangıçta PostgreSQL, `schema.sql`'i otomatik olarak içe aktarır (yalnızca veritabanı volume'u boşsa çalışır). Durumu kontrol edin:

```bash
docker compose ps
docker compose logs -f guardian-server
```

### Adım 4: Erişim

* **Web Arayüzü:** `http://<docker-host-ip>` (nginx container'ı 80 portunda yayınlar)
* **Guardian CLI:**
  ```bash
  export GUARDIAN_SERVER_HOST="https://<docker-host-ip>"
  export GUARDIAN_SERVER_PORT="5555"
  export GUARDIAN_ADMIN_USERNAME="<.env dosyanızdaki GUARDIAN_ADMIN_USERNAME>"
  export GUARDIAN_ADMIN_PASSWORD="<.env dosyanızdaki GUARDIAN_ADMIN_PASSWORD>"
  export TLS_CA_FILE="./certs/ca.crt"
  ```

### Adım 5: Agent'ları Bağlama

Her hedef sunucuda [Agent Kurulum Rehberi](./agent-setup.md)'ni takip edin. `agent.conf` içinde sunucu adresi olarak Docker host'unun IP'sini ve **agent portunu** (6666, `docker-compose.yml`'de yayınlanır) kullanın:

```ini
GUARDIAN_SERVER_HOST=https://<docker-host-ip>
GUARDIAN_SERVER_PORT=6666
```

Sertifika olarak yine `generate-certs.sh` ile oluşturduğunuz `agentN.crt`/`agentN.key` çiftini (adını `agent.crt`/`agent.key` olarak değiştirerek) ilgili agent sunucusuna kopyalayın.

### Notlar

* `docker-compose.yml`, PostgreSQL portunu (`5432`) yalnızca `127.0.0.1`'e açar — aynı bare-metal kurulumdaki güvenlik varsayılanı korunur.
* `guardian-ui` container'ı, `/api/` isteklerini iç Docker ağı üzerinden `guardian-server:5555`'e proxy'ler; tarayıcıdan `guardian-server`'a doğrudan erişim yoktur (nginx sertifika doğrulamasını devre dışı bırakır, tıpkı bare-metal Nginx yapılandırmasında olduğu gibi — güven zinciri Docker'ın kendi iç ağına dayanır).
* Servisleri güncellemek için: `docker compose build && docker compose up -d`.
* Verileri tamamen sıfırlamak için (⚠️ geri alınamaz): `docker compose down -v`.
