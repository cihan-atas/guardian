<<<<<<< HEAD
# guardian
=======
# Guardian Dağıtık Mimarisi Kurulum Rehberi

Bu rehber, Guardian uygulamasının iki farklı sunucu üzerinde (`Ana Sunucu` ve `Agent Sunucusu`) nasıl kurulacağını, yapılandırılacağını ve `systemd` servisleri olarak nasıl yönetileceğini adım adım açıklar.

## Mimariye Genel Bakış

*   **Ana Sunucu (PC 1):** Projenin merkezi bileşenlerini barındırır.
    *   `PostgreSQL`: Docker container'ı içinde çalışan veritabanı.
    *   `guardian-server`: Kullanıcı arayüzü ve ajanlarla iletişim kuran ana backend uygulaması.
    *   `guardian-ui`: Nginx tarafından sunulan Angular tabanlı web arayüzü.
*   **Agent Sunucusu (PC 2):** Hedef sunucularda çalışan ve `guardian-server`'dan komut alan ajan.
    *   `guardian-agent`: SSH proxy'si ve komut dinleyicisi.

---

## 1. Ortak Ön Hazırlık Adımları (Her İki Sunucuda da Uygulanacak)

Bu adımlar hem Ana Sunucu'da hem de Agent Sunucusu'nda gerçekleştirilmelidir.

### 1.1. `guardian` Kullanıcısı ve Grubu Oluşturma

Servisleri `root` yerine daha az yetkili bir kullanıcı ile çalıştırarak güvenliği artırıyoruz.

```bash
sudo useradd --system --no-create-home --shell /bin/false guardian
```

### 1.2. Gerekli Dizinleri Oluşturma

Uygulama dosyaları ve yapılandırma dosyaları için standart dizinler oluşturulur.

```bash
# Uygulama dosyaları için (/opt standart bir yerdir)
sudo mkdir -p /opt/guardian

# Yapılandırma ve sertifikalar için
sudo mkdir -p /etc/guardian/certs
```

---

## 2. Ana Sunucu (PC 1) Kurulumu

### 2.1. Gerekli Yazılımları Kurma

Docker, Docker Compose ve Nginx web sunucusunu kurun.

```bash
# Gerekli paketleri kur (Debian/Ubuntu için örnek)
sudo apt update
sudo apt install -y docker.io docker-compose-v2 nginx

# Docker servisini başlat ve sistem açılışında etkinleştir
sudo systemctl start docker
sudo systemctl enable docker
```

### 2.2. Proje Dosyalarını Derleme ve Taşıma

1.  **`guardian-server`'ı Derleyin:**
    ```bash
    cd /path/to/your/project/guardian-server
    go build -o guardian-server .
    ```

2.  **Gerekli Dosyaları `/opt/guardian` Dizinine Kopyalayın:**
    ```bash
    # Derlenmiş sunucu uygulamasını
    sudo cp /path/to/your/project/guardian-server/guardian-server /opt/guardian/

    # Veritabanı için docker-compose dosyasını
    sudo cp /path/to/your/project/docker-compose.yml /opt/guardian/

    # Veritabanı şemasını
    sudo cp /path/to/your/project/schema.sql /opt/guardian/
    ```

3.  **`guardian-ui`'yi Derleyin ve Taşıyın:**
    ```bash
    # UI projesini build et
    cd /path/to/your/project/guardian-ui
    ng build

    # Nginx için hedef dizini oluştur
    sudo mkdir -p /var/www/guardian-ui

    # Build edilmiş dosyaları taşı
    sudo cp -r ./dist/guardian-ui/browser/* /var/www/guardian-ui/
    ```

### 2.3. Yapılandırma Dosyalarını Oluşturma

1.  **Sertifikaları `/etc/guardian/certs` Altına Kopyalayın:**
    `ca.crt`, `server.crt`, `server.key` gibi tüm sertifikaları bu dizine taşıyın.

2.  **`server.conf` Oluşturun:**
    `sudo nano /etc/guardian/server.conf` komutuyla dosyayı oluşturun ve aşağıdaki içerikle doldurun.
    ```ini
    # /etc/guardian/server.conf

    # PostgreSQL Ayarları
    POSTGRES_USER=guardian_user
    POSTGRES_PASSWORD=guardian_password
    POSTGRES_DB=guardian_db
    POSTGRES_HOST=localhost

    # Guardian Server Ayarları
    GUARDIAN_SERVER_HOST=https://localhost
    GUARDIAN_SERVER_PORT=5555
    GUARDIAN_AGENT_PORT=6666

    # Güvenlik ve Token'lar
    GUARDIAN_SECRET_TOKEN=bu-super-gizli-bir-token
    GUARDIAN_ADMIN_TOKEN=yonetici-icin-cok-gizli-bir-token

    # Sertifika Yolları
    TLS_CA_FILE=/etc/guardian/certs/ca.crt
    TLS_CERT_FILE=/etc/guardian/certs/server.crt
    TLS_KEY_FILE=/etc/guardian/certs/server.key
    ```

3.  **`docker-compose.yml` Dosyasını Güncelleyin:**
    `sudo nano /opt/guardian/docker-compose.yml` komutuyla dosyayı açın ve içeriğinin aşağıdaki gibi olduğundan emin olun. Bu, veritabanı şemasını otomatik yükleyecektir.
    ```yaml
    version: '3.8'
    services:
      db:
        image: postgres:14-alpine
        restart: always
        env_file:
          - /etc/guardian/server.conf
        ports:
          - "127.0.0.1:5432:5432" # Sadece localhost'tan erişim için
        volumes:
          - guardian_postgres_data:/var/lib/postgresql/data
          - /opt/guardian/schema.sql:/docker-entrypoint-initdb.d/init.sql
    volumes:
      guardian_postgres_data:
    ```

### 2.4. `systemd` Servislerini Oluşturma

1.  **`guardian-db.service`:**
    `sudo nano /etc/systemd/system/guardian-db.service`
    ```ini
    [Unit]
    Description=Guardian PostgreSQL Database (via Docker Compose)
    Requires=docker.service
    After=docker.service

    [Service]
    Type=oneshot
    RemainAfterExit=yes
    WorkingDirectory=/opt/guardian
    ExecStart=/usr/bin/docker-compose -f /opt/guardian/docker-compose.yml up -d
    ExecStop=/usr/bin/docker-compose -f /opt/guardian/docker-compose.yml down
    
    [Install]
    WantedBy=multi-user.target
    ```

2.  **`guardian-server.service`:**
    `sudo nano /etc/systemd/system/guardian-server.service`
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
    ExecStart=/opt/guardian/guardian-server
    Restart=on-failure
    RestartSec=5s
    
    [Install]
    WantedBy=multi-user.target
    ```

### 2.5. Nginx Yapılandırması

`sudo nano /etc/nginx/sites-available/guardian` komutuyla bir dosya oluşturun ve UI'ı sunacak şekilde yapılandırın.

```nginx
server {
    listen 80;
    server_name sunucu_ip_adresiniz_veya_domain.com;

    root /var/www/guardian-ui;
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;
    }

    location /api/ {
        proxy_pass https://localhost:5555;
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
Yeni siteyi etkinleştirin:
```bash
sudo ln -s /etc/nginx/sites-available/guardian /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl restart nginx
```

### 2.6. İzinleri Ayarlama ve Servisleri Başlatma

1.  **Dosya Sahipliğini Ayarlayın:**
    ```bash
    sudo chown -R guardian:guardian /opt/guardian /etc/guardian
    sudo chown -R www-data:www-data /var/www/guardian-ui
    # Kritik anahtar dosyalarının izinlerini sıkılaştırın
    sudo chmod 600 /etc/guardian/certs/*.key
    ```
2.  **Veritabanını Sıfırlayın (İlk Kurulumda):**
    ```bash
    cd /opt/guardian
    sudo docker-compose down --volumes
    ```
3.  **Servisleri Başlatın ve Etkinleştirin:**
    ```bash
    sudo systemctl daemon-reload
    sudo systemctl start guardian-db.service guardian-server.service nginx.service
    sudo systemctl enable guardian-db.service guardian-server.service nginx.service
    ```

---

## 3. Agent Sunucusu (PC 2) Kurulumu

### 3.1. Proje Dosyalarını Derleme ve Taşıma

1.  **`guardian-agent`'ı Derleyin:**
    ```bash
    cd /path/to/your/project/guardian-agent
    go build -o guardian-agent .
    ```
2.  **Derlenmiş `guardian-agent`'ı `/opt/guardian` Dizinine Kopyalayın:**
    ```bash
    # scp veya benzeri bir yöntemle agent sunucusuna aktarın
    sudo cp /path/to/your/local/guardian-agent /opt/guardian/
    sudo chmod +x /opt/guardian/guardian-agent
    ```

### 3.2. Yapılandırma Dosyalarını Oluşturma

1.  **Gerekli Sertifikaları ve Anahtarları Kopyalayın:**
    Ana sunucudan `ca.crt`, `agent1.crt`, `agent1.key`, `agent_service_key` dosyalarını agent sunucusundaki `/etc/guardian/` altına kopyalayın.

2.  **`agent.conf` Oluşturun:**
    `sudo nano /etc/guardian/agent.conf` komutuyla dosyayı oluşturun. **`GUARDIAN_SERVER_HOST` değişkenini Ana Sunucunun IP adresiyle değiştirdiğinizden emin olun.**
    ```ini
    # /etc/guardian/agent.conf
    GUARDIAN_SERVER_HOST=https://<ANA_SUNUCUNUN_IP_ADRESI>
    GUARDIAN_SERVER_PORT=5555
    GUARDIAN_AGENT_SERVER_ID=2 # Her agent için farklı bir ID
    GUARDIAN_SECRET_TOKEN=bu-super-gizli-bir-token

    # Gerekli sertifika yolları
    AGENT_TLS_CERT_FILE=/etc/guardian/certs/agent1.crt
    AGENT_TLS_KEY_FILE=/etc/guardian/certs/agent1.key
    TLS_CA_FILE=/etc/guardian/certs/ca.crt
    GUARDIAN_AGENT_SSH_KEY_PATH=/etc/guardian/agent_service_key
    ```

3.  **Anahtar Yönetim Betiğini Oluşturun:**
    Güvenli anahtar yönetimi için `sudo nano /opt/guardian/manage-keys.sh` komutuyla bir betik oluşturun. (Önceki yanıtlarda verilen betik içeriğini buraya yapıştırın). Ardından izinlerini ayarlayın:
    ```bash
    sudo chmod +x /opt/guardian/manage-keys.sh
    sudo chown root:root /opt/guardian/manage-keys.sh
    ```

4.  **`sudoers` Dosyasını Yapılandırın:**
    `sudo visudo` komutunu çalıştırın ve dosyanın sonuna şu satırı ekleyin:
    ```
    guardian ALL=(ALL) NOPASSWD: /opt/guardian/manage-keys.sh
    ```

### 3.3. SSH Sunucusunu Yapılandırma

`sudo nano /etc/ssh/sshd_config` ile SSH sunucu yapılandırmasını açın ve aşağıdaki satırın `yes` olduğundan emin olun:
```sshd_config
PermitUserEnvironment yes
```
Değişiklikten sonra SSH servisini yeniden başlatın: `sudo systemctl restart sshd`

### 3.4. `systemd` Servisini Oluşturma

`sudo nano /etc/systemd/system/guardian-agent.service`
```ini
[Unit]
Description=Guardian Agent (Listener for Server Commands)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=guardian
Group=guardian
WorkingDirectory=/opt/guardian
EnvironmentFile=/etc/guardian/agent.conf
ExecStart=/opt/guardian/guardian-agent serve
Restart=on-failure
RestartSec=10s

[Install]
WantedBy=multi-user.target
```

### 3.5. İzinleri Ayarlama ve Servisi Başlatma

1.  **Dosya ve Grup İzinlerini Ayarlayın:**
    ```bash
    # Dizin sahipliği
    sudo chown -R guardian:guardian /opt/guardian /etc/guardian

    # Kritik dosya izinleri
    sudo chmod 750 /etc/guardian /etc/guardian/certs
    sudo chmod 640 /etc/guardian/agent.conf
    sudo chmod 640 /etc/guardian/certs/*.crt
    sudo chmod 640 /etc/guardian/certs/*.key
    sudo chmod 640 /etc/guardian/agent_service_key

    # Hedef SSH kullanıcılarını guardian grubuna ekleyin
    sudo usermod -a -G guardian <hedef_kullanici_adi>
    ```

2.  **Servisi Başlatın ve Etkinleştirin:**
    ```bash
    sudo systemctl daemon-reload
    sudo systemctl start guardian-agent.service
    sudo systemctl enable guardian-agent.service
    ```
    
>>>>>>> master
