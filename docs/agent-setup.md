## 🛡️ Guardian Agent Kurulumu

Bu rehber, yönetilecek bir hedef Linux sunucusuna `guardian-agent`'ın nasıl kurulacağını adım adım anlatır. Tüm komutlar `root` veya `sudo` yetkileriyle çalıştırılmalıdır.

### Adım 1: `guardian` Sistem Kullanıcısını ve Grubunu Oluşturma

Agent'ın, sistemdeki diğer kullanıcılardan izole bir şekilde çalışabilmesi için kendine ait, kısıtlı yetkilere sahip bir sistem kullanıcısına ve grubuna ihtiyacı vardır.

1.  **Önce `guardian` grubunu oluşturun:**
    ```bash
    sudo groupadd --system guardian
    ```

2.  **Sonra `guardian` kullanıcısını oluşturun:**
    ```bash
    sudo useradd --system -g guardian --no-create-home -s /usr/sbin/nologin -c "Guardian Agent Account" guardian
    ```

### Adım 2: Dizinleri ve Yapılandırma Dosyalarını Oluşturma

Bu adımda, agent için gerekli tüm dizinleri ve yapılandırma dosyalarını `root` yetkisiyle oluşturacağız.

1.  **Gerekli Dizinleri Oluşturun:**
    ```bash
    sudo mkdir -p /opt/guardian/pids
    sudo mkdir -p /etc/guardian/certs
    ```

2.  **Agent için SSH Anahtar Çifti Oluşturun:**
    ```bash
    sudo ssh-keygen -t ed25519 -f /etc/guardian/agent_service_key -N "" -C "guardian-agent-key"
    ```

3.  **`agent.conf` Yapılandırma Dosyasını Oluşturun:**
    ```bash
    sudo nano /etc/guardian/agent.conf
    ```
    Aşağıdaki şablonu dosyaya yapıştırın ve **değerleri kendi ortamınıza göre güncelleyin**:
    ```ini
    # /etc/guardian/agent.conf
    GUARDIAN_SERVER_HOST=https://10.2.60.185
    GUARDIAN_SERVER_PORT=5555
    # !!! ÖNEMLİ !!! Guardian Server'dan alınacak sunucu ID'si.
    GUARDIAN_AGENT_SERVER_ID=3
    # Guardian Server ile paylaşılacak gizli token.
    GUARDIAN_SECRET_TOKEN=bu-super-gizli-bir-token-DEGISTIR
    # Sertifika ve anahtar yolları
    TLS_CA_FILE=/etc/guardian/certs/ca.crt
    AGENT_TLS_CERT_FILE=/etc/guardian/certs/agent.crt
    AGENT_TLS_KEY_FILE=/etc/guardian/certs/agent.key
    GUARDIAN_AGENT_SSH_KEY_PATH=/etc/guardian/agent_service_key
    GUARDIAN_AGENT_TRUSTED_HOST_KEY=/etc/ssh/ssh_host_ed25519_key.pub
    ```

### Adım 3: `guardian-agent` Binary'sini Derleme ve Kopyalama

`systemd` servisini oluşturmadan önce, çalıştıracağı programın (`guardian-agent`) sistemde mevcut olması gerekir.

1.  **Geliştirme Makinenizde Derleyin:**
    Proje kaynak kodlarının bulunduğu geliştirme makinenizde, `guardian-agent` dizinine gidin ve Go derleyicisini kullanarak binary'yi oluşturun:
    ```bash
    # Geliştirme makinenizde çalıştırın
    cd /path/to/your/project/guardian/guardian-agent
    go build -o guardian-agent .
    ```

2.  **Hedef Sunucuya Kopyalayın:**
    Oluşturulan `guardian-agent` dosyasını, `scp` veya başka bir yöntemle hedef sunucudaki `/usr/local/bin/` dizinine kopyalayın.
    ```bash
    # Geliştirme makinenizde çalıştırın
    scp ./guardian-agent user@hedef-sunucu-ip:/tmp/guardian-agent
    ```

3.  **Hedef Sunucuda Yerine Taşıyın ve İzinleri Ayarlayın:**
    Hedef sunucuya SSH ile bağlanın ve dosyayı doğru konuma taşıyıp çalıştırma izni verin.
    ```bash
    # Hedef sunucuda çalıştırın
    sudo mv /tmp/guardian-agent /usr/local/bin/guardian-agent
    sudo chmod +x /usr/local/bin/guardian-agent
    ```

### Adım 4: Agent Public Anahtarını Yetkilendirme ve Kullanıcıları Gruba Ekleme

Agent'ın, proxy'lik yapacağı **tüm sistem kullanıcıları** adına `localhost`'a SSH yapabilmesi için:
1. Agent'ın genel anahtarını, bu kullanıcıların `authorized_keys` dosyasına ekleyin.
2. Bu kullanıcıları (`root` hariç), agent'ın özel anahtarını okuyabilmeleri için `guardian` grubuna ekleyin.

```bash
# Public anahtarı bir değişkene alalım
AGENT_PUB_KEY=$(sudo cat /etc/guardian/agent_service_key.pub)

# Root kullanıcısı için sadece anahtarı ekle
sudo mkdir -p /root/.ssh
echo "$AGENT_PUB_KEY" | sudo tee -a /root/.ssh/authorized_keys
sudo chmod 700 /root/.ssh && sudo chmod 600 /root/.ssh/authorized_keys && sudo chown -R root:root /root/.ssh

# agent01 kullanıcısı için anahtarı ekle VE guardian grubuna dahil et
sudo mkdir -p /home/agent01/.ssh
echo "$AGENT_PUB_KEY" | sudo tee -a /home/agent01/.ssh/authorized_keys
sudo usermod -a -G guardian agent01
sudo chmod 700 /home/agent01/.ssh && sudo chmod 600 /home/agent01/.ssh/authorized_keys && sudo chown -R agent01:agent01 /home/agent01/.ssh
```

### Adım 5: Sahiplik ve İzinleri Ayarlama (Güvenlik Adımı)

Oluşturduğumuz dizin ve dosyaların sahipliğini ve izinlerini güvenli olacak şekilde ayarlayalım.

```bash
sudo chown -R guardian:guardian /opt/guardian /etc/guardian
sudo chmod 770 /etc/guardian
sudo chmod 640 /etc/guardian/agent_service_key
sudo chmod 640 /etc/guardian/agent.conf
sudo chmod 640 /etc/guardian/agent_service_key.pub
```

### Adım 6: Sertifikaları Yerleştirme

Bu sunucu için gerekli olan TLS sertifikalarının (`ca.crt`, `agent.crt`, `agent.key`) oluşturulması ve dağıtımı, ana projenin `README.md` dosyasındaki **"TLS Sertifikalarını Oluşturma ve Dağıtma"** bölümünde detaylı olarak anlatılmıştır. Lütfen o bölümdeki adımları takip ederek ilgili sertifika dosyalarını bu sunucudaki `/etc/guardian/certs/` dizinine kopyalayın.

Kopyalama işlemi tamamlandıktan sonra, dizinin ve dosyaların izinlerini sıkılaştırın:
```bash
sudo chown -R guardian:guardian /etc/guardian/certs
sudo chmod 750 /etc/guardian/certs
sudo chmod 640 /etc/guardian/certs/*
```

### Adım 7: `sshd_config` Dosyasını Güvenli Hale Getirme

SSH servisini, Guardian'ın proxy mekanizmasıyla tutarlı ve güvenli bir şekilde çalışacak şekilde yapılandıralım. `/etc/ssh/sshd_config` dosyasını düzenleyin:

```bash
sudo nano /etc/ssh/sshd_config
```
Aşağıdaki satırların **aktif (başında `#` olmadan)** ve belirtilen değerlerde olduğundan emin olun:
```properties
# Sadece public anahtar ile girişe izin ver, şifreyi kapat.
PubkeyAuthentication yes
PasswordAuthentication no
KbdInteractiveAuthentication no

# Guardian'ın proxy komutuna ortam değişkenlerini aktarabilmesi için ZORUNLU.
PermitUserEnvironment yes

# !!! ZORUNLU: SSH sunucusunu SADECE TEK BİR host anahtarı kullanmaya zorla. !!!
# Bu, agent.conf dosyasındaki GUARDIAN_AGENT_TRUSTED_HOST_KEY ile eşleşmelidir.
# Diğer tüm HostKey satırlarını yorum satırı (#) yapın.
HostKey /etc/ssh/ssh_host_ed25519_key

# Diğer önerilen ayarlar
PermitRootLogin prohibit-password
Port 22
```

### Adım 8: `systemd` Servisini Oluşturma

`/etc/systemd/system/guardian-agent.service` dosyasını oluşturun:
```bash
sudo nano /etc/systemd/system/guardian-agent.service
```
İçine aşağıdaki içeriği yapıştırın:
```ini
[Unit]
Description=Guardian Agent (Listener for Server Commands)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
Group=root
WorkingDirectory=/opt/guardian
EnvironmentFile=/etc/guardian/agent.conf
ExecStart=/usr/local/bin/guardian-agent serve
Restart=on-failure
RestartSec=10s
StandardOutput=journal
StandardError=journal
SyslogIdentifier=guardian-agent

[Install]
WantedBy=multi-user.target
```

### Adım 9: Servisleri Aktifleştirme

1.  **SSH Servisini Yeniden Başlatma:**
    ```bash
    sudo systemctl daemon-reload
    sudo systemctl restart ssh
    ```

2.  **Guardian Agent Servisini Hazırlama (Henüz Başlatmayın!):**
    `guardian-agent` servisi, merkezi sunucu kurulup `agent.conf` dosyası doğru `SERVER_ID` ile güncellenene kadar **başlatılmamalıdır**.

    Merkezi sunucu kurulumu tamamlandıktan sonra bu sunucuya geri dönüp aşağıdaki komutları çalıştırın:
    ```bash
    # SADECE Guardian Server kurulduktan ve agent.conf güncellendikten sonra çalıştırın:
    sudo systemctl enable guardian-agent.service
    sudo systemctl start guardian-agent.service

    # Servisin durumunu kontrol etmek için:
    sudo systemctl status guardian-agent.service
    ```

