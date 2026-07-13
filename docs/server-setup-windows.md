# 🪟 Guardian Server — Windows Kurulumu

Bu rehber, merkezi bileşen `guardian-server`'ı bir **Windows** makinesinde
(Windows Server veya Windows 10/11) çalıştırmayı anlatır. Linux'taki systemd
tabanlı kurulumun ([server-setup.md](./server-setup.md)) Windows karşılığıdır;
ikisi aynı anda kullanılmaz.

> Windows'ta systemd `EnvironmentFile` yoktur. `guardian-server`, yapılandırmayı
> `GUARDIAN_SERVER_CONFIG` ile gösterilen `KEY=VALUE` dosyasından okur
> (varsayılan: `C:\ProgramData\guardian\server.conf`). Bu, `config_file.go`'daki
> mekanizmadır ve zaten set edilmiş ortam değişkenlerini ezmez.

## Genel akış

1. Sertifikaları üret (bootstrap).
2. PostgreSQL kur ve `schema.sql`'i içe aktar.
3. `guardian-server.exe`'yi derle/kopyala, `server.conf`'u doldur.
4. Servisi kur (`scripts/install-server-windows.ps1`).
5. UI'yı reverse-proxy ile servis et.

---

## Adım 1: Sertifika Bootstrap (iki seçenek)

TLS sertifikaları (CA + sunucu sertifikası) sunucu başlamadan önce hazır
olmalıdır. Windows'ta iki yol vardır.

### Seçenek A (önerilen): `guardian-server gen-certs`

`guardian-server.exe`, OpenSSL'e ihtiyaç duymadan Go'nun `crypto/x509`'u ile
CA + sunucu sertifikası üretebilen bir alt-komuta sahiptir (`gencerts.go`).
Yolları ve SAN'ları ortam değişkenlerinden alır:

| Değişken | Açıklama | Varsayılan |
| :--- | :--- | :--- |
| `TLS_CA_FILE` | CA sertifikası çıktısı | `..\certs\ca.crt` |
| `TLS_CA_KEY_FILE` | CA özel anahtarı çıktısı | `..\certs\ca.key` |
| `TLS_CERT_FILE` | Sunucu sertifikası çıktısı | `..\certs\server.crt` |
| `TLS_KEY_FILE` | Sunucu özel anahtarı çıktısı | `..\certs\server.key` |
| `GUARDIAN_CERT_HOSTS` | Virgüllü SAN listesi (DNS ve/veya IP) | `localhost,127.0.0.1` |

`127.0.0.1` her zaman SAN'a eklenir. Mevcut dosyalar `--force` verilmedikçe
korunur (üzerine yazılmaz).

```powershell
$env:TLS_CA_FILE      = "C:\ProgramData\guardian\certs\ca.crt"
$env:TLS_CA_KEY_FILE  = "C:\ProgramData\guardian\certs\ca.key"
$env:TLS_CERT_FILE    = "C:\ProgramData\guardian\certs\server.crt"
$env:TLS_KEY_FILE     = "C:\ProgramData\guardian\certs\server.key"
$env:GUARDIAN_CERT_HOSTS = "guardian.local,10.2.60.185"

# certs dizinini olustur ve uret:
New-Item -ItemType Directory -Force -Path C:\ProgramData\guardian\certs | Out-Null
& "C:\ProgramData\guardian\guardian-server.exe" gen-certs
# Yeniden uretmek (uzerine yazmak) icin:  ... gen-certs --force
```

> ⚠️ Bu alt-komut yalnızca **CA + sunucu** sertifikası üretir. Windows agent'lar
> sertifikalarını sunucudan `enroll-bundle` ile alır
> ([agent-setup-windows.md](./agent-setup-windows.md)); bunun için sunucuda
> `TLS_CA_KEY_FILE` (yani `ca.key`) yüklü olmalıdır.

> 🔐 `ca.key` tüm güvenliğin köküdür. Agent oto-kurulumu kullanmıyorsanız bu
> dosyayı sunucuda **bırakmayın**; güvenli/çevrimdışı bir yerde saklayın.

### Seçenek B: `generate-certs.sh` (Git Bash veya WSL)

Repo kökündeki `generate-certs.sh`, OpenSSL kullanan interaktif betiktir ve
Windows'ta **Git Bash** ya da **WSL** içinde çalışır (her ikisinde de `openssl`
bulunur). Betik ayrıca etkileşimsiz (non-interactive) çalışabilir:

```bash
# Git Bash / WSL icinde, repo kokunde:
GUARDIAN_CA_CN="Guardian Root CA" \
GUARDIAN_SERVER_CN=guardian.local GUARDIAN_SERVER_IP=10.2.60.185 \
GUARDIAN_AGENT_COUNT=1 GUARDIAN_AGENT1_FILENAME=agent0 \
GUARDIAN_AGENT1_CN=guardian.local GUARDIAN_AGENT1_IP=10.2.60.185 \
./generate-certs.sh 1 --force
```

Detay ve alan açıklamaları için [generate-certs-usage.md](./generate-certs-usage.md).
Üretilen `certs\` içeriğini `C:\ProgramData\guardian\certs\` altına kopyalayın
(sunucunun kendi agent'ı olacaksa `agent0.crt/.key` → `agent.crt/.key`).

---

## Adım 2: PostgreSQL Kurulumu

### Seçenek A: Docker Desktop (önerilen)

Docker Desktop for Windows kuruluysa, veritabanını bir container'da çalıştırın.
Repo'daki `docker-compose.yml`'in yalnızca `db` servisini kullanabilirsiniz:

```powershell
# Repo dizininde:
docker compose up -d db
```

`docker-compose.yml`, PostgreSQL portunu yalnızca `127.0.0.1:5432`'ye açar; bu
güvenli varsayılanı koruyun. İlk açılışta `schema.sql` otomatik içe aktarılır
(veritabanı volume'u boşsa).

### Seçenek B: Yerel PostgreSQL kurulumu

1. **PostgreSQL for Windows**'u kurun (EnterpriseDB installer veya
   `winget install PostgreSQL.PostgreSQL`).
2. Veritabanı ve kullanıcıyı oluşturun (kurulumla gelen `psql`, genellikle
   `C:\Program Files\PostgreSQL\<sürüm>\bin\psql.exe`):
   ```powershell
   & "C:\Program Files\PostgreSQL\16\bin\psql.exe" -U postgres -c "CREATE USER guardian_user WITH PASSWORD 'DEGISTIR';"
   & "C:\Program Files\PostgreSQL\16\bin\psql.exe" -U postgres -c "CREATE DATABASE guardian_db OWNER guardian_user;"
   ```
3. Şemayı içe aktarın (repo'daki `schema.sql`):
   ```powershell
   & "C:\Program Files\PostgreSQL\16\bin\psql.exe" -U guardian_user -d guardian_db -f C:\path\to\schema.sql
   ```
   > Not: `transaction_timeout` gibi bir uyarı görebilirsiniz; sürüm farkından
   > kaynaklanır ve göz ardı edilebilir (bkz. Linux rehberindeki aynı not).
4. Güvenlik için PostgreSQL'in yalnızca `localhost`'u dinlediğinden emin olun
   (`postgresql.conf` → `listen_addresses = 'localhost'`).

Her iki seçenekte de bağlantı bilgileri `server.conf`'a şu şekilde yazılır:
`POSTGRES_HOST=localhost`, `POSTGRES_PORT=5432`, `POSTGRES_USER`,
`POSTGRES_PASSWORD`, `POSTGRES_DB`.

---

## Adım 3: Binary ve Yapılandırma

1. **Derleyin** (geliştirme makinesinde, çapraz derleme ile):
   ```bash
   cd guardian-server
   GOOS=windows GOARCH=amd64 go build -o guardian-server.exe .
   ```
   veya doğrudan Windows'ta `go build -o guardian-server.exe .`.
2. `guardian-server.exe`'yi hedef Windows makinesine kopyalayın (örn.
   `C:\ProgramData\guardian\guardian-server.exe`).
3. `server.conf`'u oluşturun. Bir sonraki adımdaki kurulum script'i, dosya
   yoksa doldurulacak bir **şablon** yazar; mevcut dosyaya dokunmaz. Alanlar
   Linux'taki `server.conf` ile birebir aynıdır (bkz.
   [server-setup.md](./server-setup.md) Adım 5) — tek fark yolların Windows
   biçiminde olmasıdır:
   ```ini
   POSTGRES_USER=guardian_user
   POSTGRES_PASSWORD=DEGISTIR
   POSTGRES_DB=guardian_db
   POSTGRES_HOST=localhost
   POSTGRES_PORT=5432
   GUARDIAN_SERVER_PORT=5555
   GUARDIAN_AGENT_PORT=6666
   GUARDIAN_SECRET_TOKEN=DEGISTIR-super-gizli-token
   GUARDIAN_ADMIN_USERNAME=admin
   GUARDIAN_ADMIN_PASSWORD=DEGISTIR-guclu-parola
   TLS_CA_FILE=C:\ProgramData\guardian\certs\ca.crt
   TLS_CERT_FILE=C:\ProgramData\guardian\certs\server.crt
   TLS_KEY_FILE=C:\ProgramData\guardian\certs\server.key
   # Windows agent oto-kurulumu icin:
   # TLS_CA_KEY_FILE=C:\ProgramData\guardian\certs\ca.key
   # GUARDIAN_AGENT_BINARY_PATH_WINDOWS=C:\ProgramData\guardian\guardian-agent.exe
   ```

### Ortam değişkenlerini verme yöntemleri

- **`GUARDIAN_SERVER_CONFIG` dosyası (önerilen):** Tüm ayarları tek bir
  `server.conf` dosyasında tutun ve bu env'i o dosyaya yönlendirin. Kurulum
  script'i bunu makine (Machine) düzeyinde otomatik ayarlar.
- **Makine/servis düzeyi env değişkenleri:** Alternatif olarak her değeri
  makine düzeyinde tanımlayabilirsiniz:
  ```powershell
  [System.Environment]::SetEnvironmentVariable("GUARDIAN_SECRET_TOKEN","...","Machine")
  ```
  Makine env'i, config dosyasından **önceliklidir** (config dosyası yalnızca
  boş olan anahtarları doldurur).

---

## Adım 4: Windows Servisi Olarak Kurma

Repo'daki [`scripts/install-server-windows.ps1`](../scripts/install-server-windows.ps1)
script'i, servisi kurmayı otomatikleştirir. **Yönetici (Administrator)
PowerShell**'de çalıştırın:

```powershell
# Repo dizininden veya binary'nin yanindan:
.\scripts\install-server-windows.ps1 -BinarySource C:\build\guardian-server.exe
```

Script şunları yapar:
- `C:\ProgramData\guardian` ve `certs\` dizinlerini oluşturur,
- binary'yi kurulum dizinine kopyalar (servis çalışıyorsa önce durdurur),
- `server.conf` yoksa şablon yazar,
- `GUARDIAN_SERVER_CONFIG`'i makine env'i olarak ayarlar,
- `guardian-server` adında, **otomatik başlatmalı** bir servis kurar ve başlatır.

Servisi hemen başlatmak istemiyorsanız (ör. önce `server.conf`'u
dolduracaksanız): `-StartService:$false`.

Elle kurmak isterseniz eşdeğer komutlar:
```powershell
New-Service -Name guardian-server -BinaryPathName '"C:\ProgramData\guardian\guardian-server.exe"' `
    -DisplayName "Guardian Server" -StartupType Automatic
[System.Environment]::SetEnvironmentVariable("GUARDIAN_SERVER_CONFIG","C:\ProgramData\guardian\server.conf","Machine")
Start-Service guardian-server
```

### Servis yönetimi

```powershell
Get-Service guardian-server
Restart-Service guardian-server
Stop-Service guardian-server
# Kaldirma:
Stop-Service guardian-server; sc.exe delete guardian-server
```

Loglar: `guardian-server` konsola (stdout/stderr) yazar. Servis olarak
çalışırken çıktıyı görmek için servisi bir kez elle çalıştırıp doğrulayın:
```powershell
$env:GUARDIAN_SERVER_CONFIG="C:\ProgramData\guardian\server.conf"
& "C:\ProgramData\guardian\guardian-server.exe"
```
Sorun yaşarsanız çıktıyı bir dosyaya yönlendirmek için servis komutunu
`cmd /c ".. > log.txt 2>&1"` biçiminde saran bir sarmalayıcı kullanabilir veya
[NSSM](https://nssm.cc/) gibi bir servis yöneticisiyle log dosyası
yapılandırabilirsiniz.

---

## Adım 5: UI'yı Reverse-Proxy ile Servis Etme

`guardian-server`, `5555` portunda **HTTPS (self-signed)** dinler ve yalnızca
API sağlar. UI statik dosyaları (`guardian-ui` production build çıktısı) ayrı
servis edilir. Reverse-proxy görevi: `/api/` → guardian-server, `/` → UI
statik dosyaları.

### Seçenek A: nginx for Windows

[nginx for Windows](https://nginx.org/en/download.html)'i indirin ve
`conf\nginx.conf` içine bir `server` bloğu ekleyin. Yollar Windows biçiminde ve
`/` ile yazılır:

```nginx
server {
    listen 80;
    server_name 10.2.60.185;   # sunucunun IP'si veya DNS adi

    root   C:/guardian-ui;      # ng build ciktisi (dist/guardian-ui/browser)
    index  index.html;

    location / {
        try_files $uri $uri/ /index.html;   # Angular SPA yonlendirmesi
    }

    location /api/ {
        proxy_pass https://127.0.0.1:5555;  # guardian-server (self-signed)
        proxy_ssl_verify off;               # ic self-signed CA; dogrulama kapali

        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket (canli veri akisi) icin:
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

nginx'i servis olarak çalıştırmak için [NSSM](https://nssm.cc/) veya
`winsw` kullanabilirsiniz.

### Seçenek B: IIS (URL Rewrite + ARR)

IIS ile:
1. **Application Request Routing (ARR)** ve **URL Rewrite** modüllerini kurun.
   IIS Manager → ARR → *Server Proxy Settings* → **Enable proxy**.
2. Bir site oluşturun, fiziksel yol olarak UI build dizinini
   (`dist\guardian-ui\browser`) verin.
3. UI dizinine bir `web.config` ekleyin: `/api/*` isteklerini
   `https://127.0.0.1:5555/api/*`'e yönlendiren bir rewrite kuralı ve SPA için
   fallback kuralı:
   ```xml
   <configuration>
     <system.webServer>
       <rewrite>
         <rules>
           <!-- API'yi guardian-server'a proxy'le -->
           <rule name="guardian-api" stopProcessing="true">
             <match url="^api/(.*)" />
             <action type="Rewrite" url="https://127.0.0.1:5555/api/{R:1}" />
           </rule>
           <!-- Angular SPA fallback -->
           <rule name="spa-fallback" stopProcessing="true">
             <match url=".*" />
             <conditions logicalGrouping="MatchAll">
               <add input="{REQUEST_FILENAME}" matchType="IsFile" negate="true" />
               <add input="{REQUEST_FILENAME}" matchType="IsDirectory" negate="true" />
             </conditions>
             <action type="Rewrite" url="/index.html" />
           </rule>
         </rules>
       </rewrite>
     </system.webServer>
   </configuration>
   ```
   > guardian-server self-signed sertifika kullandığından, ARR'nin arka uç
   > sertifikasını doğrulamaması gerekir. Güven zinciri iç ağa dayanır (Linux
   > nginx kurulumundaki `proxy_ssl_verify off` ile aynı mantık).

> Production'da reverse-proxy'yi 443 (HTTPS) ile, geçerli bir sertifikayla
> yayına almanız önerilir.

---

## Bilinen sınırlamalar

- Bu akış Windows'ta uçtan uca gerçek bir Windows Server üzerinde
  doğrulanmamıştır; adımlar Linux kurulumu ve mevcut Windows agent akışından
  türetilmiştir. Gerçek ortamda doğrulanması önerilir.
- `guardian-server` kendi başına dosyaya log yazmaz; servis çıktısını görmek
  için NSSM gibi bir sarmalayıcı ya da elle çalıştırma önerilir (Adım 4).
