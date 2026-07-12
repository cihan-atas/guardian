# 🪟 Guardian Agent — Windows Kurulumu

Windows hedeflerde agent, Linux'takiyle **aynı mimariyi** kullanır: yerel PTY
yerine `localhost`'a SSH ile bağlanır ve PTY'yi Windows OpenSSH sshd'den ister
(ConPTY'yi sshd sağlar). Bu yüzden ek bir terminal kütüphanesi gerekmez.

## Önerilen yol: UI'dan otomatik kurulum

1. Web arayüzünde **Sunucular**'a hedef sunucuyu ekleyin (hostname + IP).
2. **Agent Kurulumu** ekranına gidin → sunucuyu seçin → **İşletim sistemi: Windows**
   ve süreyi seçin → **Kurulum Komutu Üret**.
3. Üretilen PowerShell komutunu **hedef sunucuda yönetici (Administrator)
   PowerShell**'de çalıştırın:
   ```powershell
   iwr -UseBasicParsing -SkipCertificateCheck -Headers @{'X-Enroll-Token'='<token>'} https://<server>:5555/api/agent/install.ps1 | iex
   ```
   Script şunları yapar: `C:\ProgramData\guardian` dizinleri, agent SSH servis
   anahtarı, sunucudan imzalı sertifika + `ca.crt` (enroll-bundle), `agent.conf`,
   `guardian-agent.exe` indirme ve **`guardian-agent` Windows servisini** kurup
   başlatma.

> Sunucu tarafında `GUARDIAN_AGENT_BINARY_PATH_WINDOWS` ayarlı olmalı ve o yolda
> `guardian-agent.exe` bulunmalıdır (script binary'yi oradan indirir). Ayrıca CA
> anahtarı (`TLS_CA_KEY_FILE`) yüklü olmalıdır.

## Ön koşullar (hedef Windows sunucu)

1. **OpenSSH Server** kurulu ve çalışır olmalı:
   ```powershell
   Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0
   Start-Service sshd; Set-Service sshd -StartupType Automatic
   ```
2. `C:\ProgramData\ssh\sshd_config` içinde:
   ```
   PubkeyAuthentication yes
   PermitUserEnvironment yes
   ```
   Değişiklik sonrası: `Restart-Service sshd`.
3. **Proxy'lenecek her kullanıcı** için agent public anahtarını yetkilendirin:
   - Standart kullanıcı: `C:\Users\<kullanici>\.ssh\authorized_keys`
   - **Yönetici hesap:** `C:\ProgramData\ssh\administrators_authorized_keys`
     (Windows OpenSSH yönetici anahtarlarını yalnızca bu paylaşımlı dosyadan
     okur). Bu durumda agent tarafında `agent.conf`'a
     `GUARDIAN_WINDOWS_ADMIN_USERS=<kullanici1>,<kullanici2>` ekleyin ki agent
     anahtarı doğru dosyaya yazsın.
   Agent public anahtarı: `C:\ProgramData\guardian\agent_service_key.pub`.

## Servis / config notları

- Windows'ta systemd `EnvironmentFile` olmadığından agent, config'i
  `GUARDIAN_AGENT_CONFIG` ile gösterilen dosyadan okur (kurulum script'i bunu
  makine ortam değişkeni olarak ayarlar; varsayılan
  `C:\ProgramData\guardian\agent.conf`).
- Servisi yönetmek: `Restart-Service guardian-agent`, `Get-Service guardian-agent`.
- Kaldırma: `Stop-Service guardian-agent; sc.exe delete guardian-agent`.

## Bilinen sınırlamalar

- SSH ile **uzaktan otomatik kurulum** (sunucudan hedefe bağlanma) şu an yalnızca
  Linux hedefler içindir; Windows'ta yukarıdaki PowerShell komutunu hedefte
  manuel çalıştırın.
- Bu akış Linux'ta uçtan uca test edilmiştir; Windows tarafının gerçek bir
  Windows Server üzerinde doğrulanması önerilir.
