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

2.  **`docker-compose.yml` Dosyasını Güvenli Hale Getirin:**
    Kopyaladığınız `/opt/guardian/docker-compose.yml` dosyasını açın ve `ports` bölümünü düzenleyerek veritabanının sadece sunucunun kendisinden erişilebilir olmasını sağlayın.
    ```bash
    sudo nano /opt/guardian/docker-compose.yml
    ```
    Dosyanın içeriği şu şekilde olmalıdır:
    ```yaml
    services:
      db:
        image: postgres:14-alpine
        restart: always
        environment:
          POSTGRS_USER: guardian_user
          POSTGRES_PASSWORD: guardian_password
          POSTGRES_DB: guardian_db
        ports:
          - "127.0.0.1:5432:5432" # Sadece localhost'tan erişim
        volumes:
          - guardian_postgres_data:/var/lib/postgresql/data
    
    volumes:
      guardian_postgres_data:
    ```
    
3.  **Veritabanı için `systemd` Servisi Oluşturun:**
    ```bash
    sudo nano /etc/systemd/system/guardian-db.service
    ```
    İçine aşağıdaki içeriği yapıştırın:
    ```ini
    [Unit]
    Description=Guardian PostgreSQL Database (via Docker Compose)
    Requires=docker.service
    After=docker.service

    [Service]
    Type=oneshot
    RemainAfterExit=yes
    WorkingDirectory=/opt/guardian
    ExecStart=/usr/bin/docker compose -f /opt/guardian/docker-compose.yml up -d
    ExecStop=/usr/bin/docker compose -f /opt/guardian/docker-compose.yml down

    [Install]
    WantedBy=multi-user.target
    ```

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
    # /etc/guardian/server.conf
    POSTGRES_USER=guardian_user
    POSTGRES_PASSWORD=guardian_password
    POSTGRES_DB=guardian_db
    POSTGRES_HOST=localhost
    POSTGRES_PORT=5432
    GUARDIAN_SERVER_PORT=5555
    GUARDIAN_AGENT_PORT=6666
    GUARDIAN_SECRET_TOKEN=bu-super-gizli-bir-token-DEGISTIR
    GUARDIAN_ADMIN_TOKEN=yonetici-icin-cok-gizli-bir-token-DEGISTIR
    TLS_CA_FILE=/etc/guardian/certs/ca.crt
    TLS_CERT_FILE=/etc/guardian/certs/server.crt
    TLS_KEY_FILE=/etc/guardian/certs/server.key
    ```

2.  **`agent.conf` Dosyasını Oluşturun (Bu sunucunun kendi agent'ı için):**
    ```bash
    sudo nano /etc/guardian/agent.conf
    ```
    ```ini
    # /etc/guardian/agent.conf
    GUARDIAN_SERVER_HOST=https://127.0.0.1
    GUARDIAN_SERVER_PORT=5555
    GUARDIAN_AGENT_SERVER_ID=2 # Bu sunucunun Guardian'daki ID'si
    GUARDIAN_SECRET_TOKEN=bu-super-gizli-bir-token-DEGISTIR
    TLS_CA_FILE=/etc/guardian/certs/ca.crt
    AGENT_TLS_CERT_FILE=/etc/guardian/certs/agent.crt
    AGENT_TLS_KEY_FILE=/etc/guardian/certs/agent.key
    GUARDIAN_AGENT_SSH_KEY_PATH=/etc/guardian/agent_service_key
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
    server {
        listen 80;
        server_name SUNUCU_IP_ADRESINIZ; # örn: 10.2.60.185

        root /var/www/guardian-ui;
        index index.html;

        location / {
            try_files $uri $uri/ /index.html;
        }

        location /api/ {
            proxy_pass https://SUNUCU_IP_ADRESINIZ:5555; # örn: https://10.2.60.185:5555
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
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

Bu sunucu aynı zamanda bir agent barındıracağı için, **Guardian Agent Kurulumu** rehberindeki aşağıdaki adımları bu sunucuya da uygulamanız gerekmektedir:

*   `Adım 2: Agent için SSH Anahtar Çifti Oluşturma`
*   `Adım 4: Agent Public Anahtarını Yetkilendirme...`
*   `Adım 7: sshd_config Dosyasını Güvenli Hale Getirme`

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
