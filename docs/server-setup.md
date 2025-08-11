## 🛡️ Guardian Sunucu Kurulumu

Bu rehber, projenin merkezi bileşeni olan `guardian-server`'ı ve gerekli tüm bağımlılıklarını (Veritabanı, Web Arayüzü, Agent) bir Linux sunucusuna kurma adımlarını içerir. Tüm komutlar `root` veya `sudo` yetkileriyle çalıştırılmalıdır.

### Adım 1: `guardian` Sistem Kullanıcısını ve Grubunu Oluşturma

Sunucu uygulaması ve ilgili dosyalar için izole bir sistem kullanıcısı ve grubu oluşturmak, güvenlik için en iyi pratiktir.

1.  **Önce `guardian` grubunu oluşturun:**
    ```bash
    sudo groupadd --system guardian
    ```

2.  **Sonra `guardian` kullanıcısını oluşturun:**
    ```bash
    sudo useradd --system -g guardian --no-create-home -s /usr/sbin/nologin -c "Guardian Server Account" guardian
    ```

### Adım 2: Gerekli Dizinleri Oluşturma

Sunucunun çalışması için gerekli olan tüm dizin yapılarını oluşturalım.

```bash
sudo mkdir -p /opt/guardian/pids
sudo mkdir -p /etc/guardian/certs
sudo mkdir -p /var/www/guardian-ui
```

### Adım 3: Veritabanı Kurulumu (Docker Compose ile)

Guardian, verilerini saklamak için PostgreSQL kullanır. Kurulumu basitleştirmek için veritabanını bir Docker konteyneri içinde çalıştıracağız.

1.  **Repo'dan Gerekli Dosyaları Kopyalayın:**
    Projenin GitHub reposundan `docker-compose.yml` ve `schema.sql` dosyalarını sunucunuzdaki `/opt/guardian/` dizinine kopyalayın.


#### 2. `docker-compose.yml` Dosyasını Yapılandırın ve Güvenliğini Sağlayın

**Önemli:** Veritabanınızın güvenliği, sistemin genel güvenliği için hayati önem taşır. Bu adımda, veritabanına sadece sunucunun kendi içinden erişilebildiğinden emin olacağız.

Kopyaladığınız `/opt/guardian/docker-compose.yml` dosyasını bir metin editörü ile açın:

```bash
sudo nano /opt/guardian/docker-compose.yml
```

Dosyanın içeriğinin aşağıdaki gibi olduğundan emin olun. Özellikle `ports` bölümünü ve `environment` altındaki parolayı kontrol edin:

```yaml
services:
  db:
    image: postgres:14-alpine
    restart: always
    environment:
      POSTGRES_USER: guardian_user
      # DİKKAT: Production ortamında bu parolayı mutlaka daha güvenli bir parola ile değiştirin!
      POSTGRES_PASSWORD: guardian_password
      POSTGRES_DB: guardian_db
    ports:
      # SADECE localhost'tan (sunucunun kendisinden) erişime izin verilir.
      # Bu, en güvenli ve tavsiye edilen yapılandırmadır.
      - "127.0.0.1:5432:5432"
    volumes:
      - guardian_postgres_data:/var/lib/postgresql/data

volumes:
  guardian_postgres_data:
```

Bu yapılandırmadaki `"127.0.0.1:5432:5432"` satırı, PostgreSQL veritabanı portunun (5432) **yalnızca sunucunun kendisine** (localhost) açılmasını sağlar. Bu sayede dış ağdan veya yerel ağdaki başka bir bilgisayardan veritabanına doğrudan erişim engellenmiş olur. **Production (canlı) ortamlar için bu yapılandırmayı değiştirmemeniz şiddetle tavsiye edilir.**

---

#### İsteğe Bağlı: Veritabanını Yerel Ağa Açma (Tavsiye Edilmez)

> **⚠️ DİKKAT: CİDDİ GÜVENLİK RİSKİ ⚠️**
>
> Bu adımı yalnızca **geliştirme ortamınızda, güvenli ve tamamen kapalı bir yerel ağ içerisindeyseniz** ve veritabanına kendi bilgisayarınızdan bir araçla (örn: DBeaver) erişmeniz *kesinlikle* gerekiyorsa uygulayın.
>
> Veritabanı portunu yerel ağa açmak, kontrolsüz ağlarda veritabanınıza yetkisiz erişim sağlanmasına ve veri sızıntılarına yol açabilecek **kritik bir güvenlik açığı** oluşturur.
>
> **BU YAPILANDIRMAYI PRODUCTION (CANLI) SUNUCULARDA ASLA KULLANMAYIN!**
>
> Eğer riski anladıysanız ve devam etmek istiyorsanız, `docker-compose.yml` dosyasındaki `ports` bölümünü şu şekilde değiştirin:
>
> ```diff
> services:
>   db:
>     ...
>     ports:
> -     - "127.0.0.1:5432:5432"  # GÜVENLİ: Sadece localhost'tan erişim
> +     - "5432:5432"             # RİSKLİ: Tüm ağ arayüzlerinden erişim
>     ...
> ```
---

#### 3. Veritabanı için `systemd` Servisi Oluşturun

Sunucu yeniden başladığında veritabanı konteynerinin otomatik olarak başlaması için bir `systemd` servisi oluşturacağız.

```bash
sudo nano /etc/systemd/system/guardian-db.service
```

Açılan editörün içine aşağıdaki içeriği yapıştırın ve dosyayı kaydedin:

```ini
# ##################################################################
# #          Guardian Veritabanı için systemd Servis Dosyası       #
# #          Konum: /etc/systemd/system/guardian-db.service        #
# ##################################################################
#
# Bu servis, sunucu açıldığında Guardian'ın PostgreSQL veritabanını
# barındıran Docker konteynerini otomatik olarak başlatır ve
# sunucu kapandığında güvenli bir şekilde durdurur.

[Unit]
# --- Servis Tanımı ve Bağımlılıklar ---
# Bu bölüm, servisin ne olduğunu ve çalışmak için nelere ihtiyacı olduğunu tanımlar.

# systemctl status komutunda görünecek olan insan tarafından okunabilir açıklama.
Description=Guardian PostgreSQL Database (via Docker Compose)

# Bu servis başlamadan önce 'docker.service'in çalışıyor olmasını zorunlu kılar.
# Eğer Docker servisi başlamazsa, bu servis de başlamaz.
Requires=docker.service

# Bu servis, sadece 'docker.service' başarıyla başladıktan SONRA başlar.
# Bu, olası bir yarış durumunu (race condition) önler.
After=docker.service


[Service]
# --- Servis Davranışı ve Komutlar ---
# Bu bölüm, servisin nasıl başlayacağını, duracağını ve çalışacağını tanımlar.

# Servis tipini 'oneshot' olarak ayarlar. Bu, ExecStart'taki komutun
# çalışıp bitmesi beklenen tek seferlik bir işlem olduğu anlamına gelir.
# 'docker compose up -d' komutu konteynerleri arka plana atıp kendisi sonlandığı için bu tip uygundur.
Type=oneshot

# 'Type=oneshot' ile birlikte kullanılır. ExecStart komutu bittikten sonra bile
# systemd'nin bu servisi 'aktif' olarak kabul etmesini sağlar.
# Bu sayede, konteynerler arka planda çalışmaya devam ederken servis durumu doğru görünür.
RemainAfterExit=yes

# Komutların çalıştırılacağı dizini belirtir. /opt/guardian/docker-compose.yml dosyasının
# bulunduğu yer olduğu için bu dizin ayarlanmıştır.
WorkingDirectory=/opt/guardian

# Servis başlatıldığında ('systemctl start guardian-db') çalıştırılacak komut.
# Belirtilen docker-compose.yml dosyasını kullanarak konteynerleri arka planda (-d) başlatır.
ExecStart=/usr/bin/docker compose -f /opt/guardian/docker-compose.yml up -d

# Servis durdurulduğunda ('systemctl stop guardian-db') çalıştırılacak komut.
# Konteynerleri güvenli bir şekilde durdurur ve kaldırır.
ExecStop=/usr/bin/docker compose -f /opt/guardian/docker-compose.yml down


[Install]
# --- Servis Etkinleştirme Ayarları ---
# Bu bölüm, 'systemctl enable' komutu çalıştırıldığında ne olacağını tanımlar.

# Bu servis, sistem 'multi-user.target' seviyesine ulaştığında (yani normal sunucu
# çalışma moduna geçtiğinde) başlatılmak istenir. Bu satır, servisin açılışta
# otomatik olarak başlamasını sağlar.
WantedBy=multi-user.target
```

#### 4. Servisi Aktif Edin ve Başlatın

Yeni oluşturduğunuz servisi `systemd`'ye tanıtın, açılışta başlayacak şekilde ayarlayın ve hemen başlatın:

```bash
# systemd'ye yeni servis dosyasını okumasını söyleyin
sudo systemctl daemon-reload

# Servisi açılışta başlayacak şekilde etkinleştirin ve şimdi başlatın
sudo systemctl enable --now guardian-db.service
```

#### 5. Kurulumu Doğrulayın

Servisin ve veritabanı konteynerinin çalıştığını kontrol edin:

```bash
# systemd servis durumunu kontrol edin
sudo systemctl status guardian-db.service
```

Çıktıda `active (exited)` görmelisiniz. Bu, `ExecStart` komutunun başarıyla çalışıp sonlandığını, ancak servisin aktif kalmaya devam ettiğini gösterir (`RemainAfterExit=yes` sayesinde).

```bash
# Çalışan Docker konteynerlerini listeleyin
sudo docker ps
```
Çıktıda `postgres:14-alpine` imajını kullanan bir konteynerin çalıştığını ve `PORTS` kısmında `127.0.0.1:5432->5432/tcp` yazdığını görmelisiniz.

### Adım 4: Binary'leri Derleme ve Kopyalama

`guardian-server` ve (bu sunucuya da kurulacak olan) `guardian-agent` programlarını derleyip sunucuya kopyalayın.

1.  **Geliştirme Makinenizde Derleyin:**
    ```bash
    # Guardian Server için
    cd /path/to/project/guardian/guardian-server
    go build -o guardian-server .

    # Guardian Agent için
    cd /path/to/project/guardian/guardian-agent
    go build -o guardian-agent .
    ```

2.  **Hedef Sunucuya Kopyalayın ve İzinleri Ayarlayın:**
    ```bash
    scp ./guardian-server user@sunucu-ip:/tmp/guardian-server
    scp ../guardian-agent/guardian-agent user@sunucu-ip:/tmp/guardian-agent

    # Sunucuya bağlanıp dosyaları taşıyın ve çalıştırılabilir yapın
    # ssh user@sunucu-ip
    sudo mv /tmp/guardian-server /usr/local/bin/
    sudo mv /tmp/guardian-agent /usr/local/bin/
    sudo chmod +x /usr/local/bin/guardian-*
    ```

### Adım 5: Yapılandırma Dosyalarını Oluşturma

`/etc/guardian` altına hem sunucu hem de (opsiyonel) agent için yapılandırma dosyalarını oluşturalım.

1.  **`server.conf` Dosyasını Oluşturun:**
    ```bash
    sudo nano /etc/guardian/server.conf
    ```
```ini
#################################################################
#           Guardian Server Yapılandırma Dosyası                #
#             Konum: /etc/guardian/server.conf                  #
#################################################################

# --- Veritabanı Bağlantı Ayarları ------------------------------
# Bu bölümdeki ayarlar, Guardian Server'ın PostgreSQL veritabanına nasıl
# bağlanacağını belirler. Bu değerlerin, /opt/guardian/docker-compose.yml
# dosyasındaki 'environment' bölümü ile tutarlı olması gerekir.

POSTGRES_USER=guardian_user

# DİKKAT: Bu parolanın, docker-compose.yml dosyasındaki POSTGRES_PASSWORD ile aynı olması zorunludur.
POSTGRES_PASSWORD=guardian_password

POSTGRES_DB=guardian_db

# Veritabanı Docker konteyneri ile aynı sunucuda çalıştığı için 'localhost' kullanılır.
POSTGRES_HOST=localhost
POSTGRES_PORT=5432


# --- Sunucu Ağ Ayarları -----------------------------------------
# Guardian servislerinin dinleyeceği ağ portları.

# Guardian Server'ın CLI ve diğer bileşenlerden gelen yönetimsel istekleri dinleyeceği port.
GUARDIAN_SERVER_PORT=5555

# Guardian Agent'ların metrik göndermek için bağlanacağı port.
GUARDIAN_AGENT_PORT=6666


# --- Güvenlik ve Kimlik Doğrulama Ayarları ----------------------
# DİKKAT: Aşağıdaki token değerleri production (canlı) ortamı için
# mutlaka GÜVENLİ ve TAHMİN EDİLEMEZ değerlerle değiştirilmelidir!

# Agent'ların sunucuya bağlanırken kullandığı paylaşımlı gizli anahtar.
GUARDIAN_SECRET_TOKEN=bu-super-gizli-bir-token-DEGISTIR

# Guardian CLI aracılığıyla yönetimsel komutları çalıştırmak ve Web Arayüzüne (UI) bağlanmak için gereken yönetici token.
GUARDIAN_ADMIN_TOKEN=yonetici-icin-cok-gizli-bir-token-DEGISTIR


# --- TLS/SSL Sertifika Ayarları --------------------------------
# Bu yollar, 'generate-certs.sh' betiği ile oluşturulan ve sunucuya
# kopyalanan sertifika dosyalarını göstermelidir.

# Kök Sertifika Otoritesi (CA) dosyası.
TLS_CA_FILE=/etc/guardian/certs/ca.crt

# Sunucunun kendi sertifika dosyası.
TLS_CERT_FILE=/etc/guardian/certs/server.crt

# Sunucunun özel anahtar dosyası.
TLS_KEY_FILE=/etc/guardian/certs/server.key
```

2.  **`agent.conf` Dosyasını Oluşturun (Bu sunucunun kendi agent'ı için):**
    ```bash
    sudo nano /etc/guardian/agent.conf
    ```
    
```ini
#################################################################
#              Guardian Agent Yapılandırma Dosyası              #
#             Konum: /etc/guardian/agent.conf                   #
#################################################################

# --- Merkezi Sunucu Bağlantı Ayarları --------------------------
# Bu bölüm, Agent'ın hangi Guardian Server'a bağlanacağını ve
# nasıl iletişim kuracağını belirler.

# DİKKAT: Guardian Server'ın genel erişilebilir IP adresini veya DNS adını girin.
# 'https://' protokolü zorunludur.
GUARDIAN_SERVER_HOST=https://10.10.10.2

# Guardian Server'ın agent bağlantılarını dinlediği port.
# Bu değer, server.conf dosyasındaki 'GUARDIAN_AGENT_PORT' ile aynı olmalıdır.
GUARDIAN_SERVER_PORT=6666


# --- Agent Kimlik ve Doğrulama Ayarları -------------------------
# Bu bölüm, Agent'ın sunucuya kendini nasıl tanıtacağını belirler.

# DİKKAT: Bu Agent'a Guardian Server üzerinde atanan benzersiz kimlik (ID) numarasıdır.
# Bu değerin sunucu tarafındaki kayıtlarla eşleşmesi gerekir.
GUARDIAN_AGENT_SERVER_ID=2

# DİKKAT: Sunucu ile paylaşılan gizli anahtar.
# Bu değer, server.conf dosyasındaki 'GUARDIAN_SECRET_TOKEN' ile birebir aynı olmalıdır.
GUARDIAN_SECRET_TOKEN=bu-super-gizli-bir-token-DEGISTIR


# --- TLS/SSL Sertifika Ayarları --------------------------------
# Güvenli iletişim (mTLS) için gereken sertifika dosyalarının yolları.

# Guardian Server'ın kimliğini doğrulamak için kullanılan Kök Sertifika Otoritesi (CA) dosyası.
TLS_CA_FILE=/etc/guardian/certs/ca.crt

# Bu Agent'ın kendi kimliğini sunucuya kanıtlamak için kullandığı sertifika dosyası.
AGENT_TLS_CERT_FILE=/etc/guardian/certs/agent.crt

# Agent sertifikasına karşılık gelen özel anahtar dosyası. Bu dosya hassastır.
AGENT_TLS_KEY_FILE=/etc/guardian/certs/agent.key


# --- SSH Bağlantı Ayarları (İleri Düzey Görevler için) ---------
# Agent'ın görevleri yerine getirmek için SSH bağlantısı yapması gerektiğinde
# kullanacağı anahtar dosyaları.

# Agent'ın SSH bağlantısı kurarken kullanacağı özel anahtarın yolu.
GUARDIAN_AGENT_SSH_KEY_PATH=/etc/guardian/agent_service_key

# Agent'ın bağlanacağı SSH sunucusunun genel anahtarı. Bu, 'ortadaki adam'
# saldırılarını önlemek için SSH sunucusunun kimliğini doğrulamada kullanılır.
GUARDIAN_AGENT_TRUSTED_HOST_KEY=/etc/ssh/ssh_host_ed25519_key.pub
```

### Adım 6: Web Arayüzü (UI) Kurulumu ve Nginx Yapılandırması

1.  **UI Dosyalarını Derleyip Kopyalayın:**
    Geliştirme makinenizde `guardian-ui` dizininde `ng build` komutunu çalıştırın. `dist/guardian-ui/browser/` dizini içindeki tüm dosyaları sunucudaki `/var/www/guardian-ui/` dizinine kopyalayın.

2.  **Nginx Site Yapılandırması Oluşturun:**
    ```bash
    sudo nano /etc/nginx/sites-available/guardian
    ```
    `server_name` ve `proxy_pass` direktiflerini sunucunuzun IP'si ile güncelleyerek yapıştırın:
    
```nginx
# ##################################################################
# #          Guardian Web Arayüzü (UI) için Nginx Yapılandırması    #
# ##################################################################
#
# Bu yapılandırma, iki temel görevi yerine getirir:
# 1. /var/www/guardian-ui dizinindeki statik dosyaları (HTML, CSS, JS) sunar.
# 2. /api/ ile başlayan tüm istekleri, arka planda çalışan Guardian Server'a
#    (port 5555) yönlendirir (Reverse Proxy).

server {
    # --- Temel Sunucu Ayarları ---
    # Nginx'in 80 numaralı porttan gelen HTTP isteklerini dinlemesini sağlar.
    # DİKKAT: Production (canlı) ortamı için 443 portu (HTTPS) ile
    # SSL/TLS sertifikası kullanmanız şiddetle tavsiye edilir.
    listen 80;

    # Bu yapılandırmanın hangi alan adı veya IP adresi için geçerli olduğunu belirtir.
    # 'SUNUCU_IP_ADRESINIZ' kısmını kendi sunucunuzun IP adresi veya alan adı ile değiştirin.
    server_name 10.10.10.2; # örn: 10.10.10.2 veya guardian.sirketim.com

    # --- Web Arayüzü (UI) Dosyalarını Sunma ---
    # Statik dosyaların bulunduğu ana dizin.
    root /var/www/guardian-ui;
    # Bir dizin istendiğinde varsayılan olarak sunulacak dosya.
    index index.html;

    # Gelen tüm istekler için bu blok çalışır.
    location / {
        # Bu satır, React, Vue, Angular gibi Modern JavaScript (SPA)
        # uygulamaları için hayati önem taşır.
        # 1. Önce istenen URI'nin bir dosya olup olmadığını kontrol eder ($uri).
        # 2. Dosya değilse, bir dizin olup olmadığını kontrol eder ($uri/).
        # 3. İkisi de değilse, isteği /index.html dosyasına yönlendirir.
        # Bu, tarayıcıda /dashboard gibi sanal yolların yenilendiğinde
        # 404 hatası vermemesini ve uygulamanın JavaScript'inin yönlendirmeyi
        # devralmasını sağlar.
        try_files $uri $uri/ /index.html;
    }

    # --- API İsteklerini Arka Uç Sunucusuna Yönlendirme (Reverse Proxy) ---
    # Sadece '/api/' ile başlayan istekler bu bloğa girer.
    location /api/ {
        # İsteği, arka planda çalışan Guardian Server uygulamasına yönlendirir.
        # 'SUNUCU_IP_ADRESINIZ' kısmını kendi sunucunuzun IP adresi ile değiştirin.
        # Port (5555), server.conf dosyasındaki GUARDIAN_SERVER_PORT ile eşleşmelidir.
        proxy_pass https://10.10.10.2:5555; # örn: https://10.10.10.2:5555

        # --- İstemci Bilgilerini Arka Uca İletme ---
        # Bu başlıklar, Guardian Server'ın isteğin doğrudan Nginx'ten değil,
        # gerçek kullanıcıdan geldiğini anlamasını sağlar.

        # İsteğin yapıldığı orijinal 'Host' başlığını korur.
        proxy_set_header Host $host;
        # Kullanıcının gerçek IP adresini arka uca iletir.
        proxy_set_header X-Real-IP $remote_addr;
        # Proxy zincirindeki tüm IP'leri içeren standart başlık.
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        # Orijinal isteğin protokolünü (http veya https) arka uca bildirir.
        proxy_set_header X-Forwarded-Proto $scheme;

        # --- WebSocket Desteği için Ayarlar ---
        # Bu üç satır, WebSocket bağlantılarının (canlı veri akışı vb. için)
        # proxy üzerinden sorunsuz bir şekilde çalışmasını sağlar.
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

3.  **Siteyi Aktifleştirin:**
    ```bash
    sudo ln -s /etc/nginx/sites-available/guardian /etc/nginx/sites-enabled/
    sudo nginx -t # Yapılandırmayı test et
    ```

### Adım 7: Sunucuya Özel Agent Adımlarını ve `sshd`'yi Yapılandırma

Bu sunucu aynı zamanda bir agent barındıracağı için, **[Guardian Agent Kurulumu](./agent-setup.md)** rehberindeki bazı adımları bu sunucuya da uygulamanız gerekmektedir.

Lütfen aşağıdaki bağlantılara tıklayarak ilgili adımları takip edin:

*   [`Adım 2: Agent için SSH Anahtar Çifti Oluşturma`](./agent-setup.md#adım-2-agent-için-ssh-anahtar-çifti-oluşturma)
*   [`Adım 4: Agent Public Anahtarını Yetkilendirme`](./agent-setup.md#adım-4-agent-public-anahtarını-yetkilendirme)
*   [`Adım 7: sshd_config Dosyasını Güvenli Hale Getirme`](./agent-setup.md#adım-7-sshd_config-dosyasını-güvenli-hale-getirme)

---

### Adım 8: Sertifikaları Yerleştirme ve Güvenliği Sağlama

Bu adımda, oluşturulan sertifikaları sunucuya kopyalayıp tüm Guardian dosyalarının izinlerini güvenli hale getireceğiz.

1.  **Sertifikaları Kopyalama:**
    Bu sunucu için gerekli olan TLS sertifikalarının oluşturulması ve dağıtımı, ana `README.md` dosyasındaki **"TLS Sertifikalarını Oluşturma ve Dağıtma"** bölümünde detaylı olarak anlatılmıştır. Lütfen o bölümdeki adımları takip ederek aşağıdaki dosyaları bu sunucudaki `/etc/guardian/certs/` dizinine kopyalayın:
    *   `ca.crt`
    *   `server.crt`
    *   `server.key`
    *   Bu sunucuya özel `agent` sertifikası (örn: `agent0.crt`, kopyalarken adını `agent.crt` olarak değiştirin)
    *   Bu sunucuya özel `agent` anahtarı (örn: `agent0.key`, kopyalarken adını `agent.key` olarak değiştirin)

2.  **`ca.key` (Kök Anahtar) Güvenliği:**
    > 🔐 **ÇOK ÖNEMLİ:** `ca.key` dosyası, tüm TLS güvenliğinizin temelidir. Bu anahtar ele geçirilirse, saldırganlar tüm sunucu ve agent'larınız için geçerli sertifikalar üretebilir ve iletişimi dinleyebilir. **Bu dosyayı ASLA bir sunucuda bırakmayın.** Sertifikaları oluşturduktan sonra `ca.key` dosyasını güvenli, çevrimdışı bir yerde (örneğin şifreli bir USB bellek) saklayın. `/etc/guardian/certs/` dizinine **kopyalamayın**.

3.  **Sahiplik ve İzinleri Ayarlama:**
    Tüm dosyalar yerleştirildikten sonra, `guardian` ile ilgili tüm dizinlerin sahipliğini ve izinlerini güvenli hale getirin:
    ```bash
    sudo chown -R guardian:guardian /opt/guardian /etc/guardian /var/www/guardian-ui
    # Agent kurulumundan gelen chmod komutlarını da buraya uygulayarak tüm izinleri tek seferde ayarlayın
    sudo chmod 770 /etc/guardian
    sudo chmod 640 /etc/guardian/agent_service_key
    sudo chmod 640 /etc/guardian/server.conf
    sudo chmod 640 /etc/guardian/agent.conf
    sudo chmod 750 /etc/guardian/certs
    sudo chmod 640 /etc/guardian/certs/*
    ```

### Adım 9: `systemd` Servislerini Oluşturma

1.  **`guardian-server.service`:**
    ```bash
    sudo nano /etc/systemd/system/guardian-server.service
    ```
    ```ini
    [Unit]
    Description=Guardian Main Server Application
    Requires=guardian-db.service
    After=guardian-db.service network-online.target

    [Service]
    Type=simple
    User=guardian
    Group=guardian
    WorkingDirectory=/opt/guardian
    EnvironmentFile=/etc/guardian/server.conf
    ExecStart=/usr/local/bin/guardian-server
    Restart=on-failure
    RestartSec=5s
    StandardOutput=journal
    StandardError=journal
    SyslogIdentifier=guardian-server
    PrivateTmp=true
    ProtectSystem=full
    ProtectHome=true

    [Install]
    WantedBy=multi-user.target
    ```

2.  **`guardian-agent.service` (Bu sunucunun kendisi için):**
    Bu dosyanın içeriği, Agent Kurulumu rehberindeki ile aynıdır.

### Adım 10: Servisleri Aktifleştirme

Tüm yapılandırmalar tamamlandıktan sonra, servisleri doğru sırada başlatın.
```bash
# Değişiklikleri sisteme tanıt
sudo systemctl daemon-reload

# Servisleri etkinleştir ve başlat
sudo systemctl enable --now guardian-db.service
sudo systemctl enable --now guardian-server.service
sudo systemctl enable --now guardian-agent.service
sudo systemctl enable --now nginx

# Durumlarını kontrol et
sudo systemctl status guardian-db.service guardian-server.service guardian-agent.service nginx
```
