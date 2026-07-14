#Requires -Version 5.1
<#
.SYNOPSIS
    Guardian'ın Windows üzerinde çalışma (runtime) doğrulamasını yapan otomatik
    duman testi (smoke test).

.DESCRIPTION
    Bu betik, GERÇEK bir Windows host üzerinde çalıştırılmak üzere tasarlanmıştır
    (geliştirme ortamı Linux olduğundan Windows runtime'ı orada doğrulanamaz).
    Ağır bağımlılık gerektirmeyen adımları OTOMATİK doğrular ve her adım için
    PASS/FAIL raporlar:

      1) Ön koşullar: Go, OpenSSH (ssh/sshd) sürümleri.
      2) Binary derleme/doğrulama: guardian-server.exe + guardian-agent.exe.
      3) Sertifika bootstrap (openssl'siz): `guardian-server gen-certs` çalıştırılır;
         ca.crt/server.crt/server.key üretimi ve server.crt'nin ca.crt tarafından
         imzalanmışlığı + 127.0.0.1 SAN'ı .NET X509 API'siyle doğrulanır.
         (Windows'taki en kritik boşluk buydu; artık openssl gerekmeden çalışıyor.)
      4) Birim testler: `go test ./...` (Windows'a özgü build-tag'li testler dahil
         — administrators_authorized_keys yolu, ProgramData config yolu vb.).

    Canlı sunucu + agent + SSH oturumu akışı PostgreSQL ve çalışan bir OpenSSH
    sshd gerektirdiğinden OTOMATİK KAPSAMDA DEĞİLDİR; bu betik en sonda o akış
    için elle takip edilecek bir kontrol listesi (checklist) yazdırır.

.PARAMETER RepoRoot
    Depo kök dizini. Varsayılan: betiğin bir üst dizini.

.PARAMETER WorkDir
    Sertifikaların üretileceği geçici çalışma dizini. Varsayılan: yeni bir temp dizin.

.PARAMETER SkipBuild
    Binary derlemeyi atla (mevcut .exe'ler kullanılır/aranır).

.PARAMETER SkipTests
    `go test` adımını atla.

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File .\scripts\verify-windows.ps1
#>
[CmdletBinding()]
param(
    [string]$RepoRoot = (Split-Path -Parent $PSScriptRoot),
    [string]$WorkDir = (Join-Path $env:TEMP ("guardian-verify-" + [Guid]::NewGuid().ToString("N").Substring(0, 8))),
    [switch]$SkipBuild,
    [switch]$SkipTests
)

$ErrorActionPreference = 'Stop'
$script:pass = 0
$script:fail = 0
$script:skip = 0

function Step-Result {
    param([string]$Name, [ValidateSet('PASS', 'FAIL', 'SKIP')][string]$Status, [string]$Detail = '')
    $color = @{ PASS = 'Green'; FAIL = 'Red'; SKIP = 'Yellow' }[$Status]
    Write-Host ("[{0}] {1}" -f $Status, $Name) -ForegroundColor $color
    if ($Detail) { Write-Host ("       {0}" -f $Detail) -ForegroundColor DarkGray }
    switch ($Status) { 'PASS' { $script:pass++ } 'FAIL' { $script:fail++ } 'SKIP' { $script:skip++ } }
}

Write-Host "=== Guardian Windows Runtime Doğrulaması ===" -ForegroundColor Cyan
Write-Host ("Depo kökü : {0}" -f $RepoRoot)
Write-Host ("Çalışma   : {0}" -f $WorkDir)
Write-Host ""

# --- 1) Ön koşullar --------------------------------------------------------
$goExe = Get-Command go -ErrorAction SilentlyContinue
if ($goExe) { Step-Result "Go bulundu" PASS (& go version) }
else { Step-Result "Go bulundu" SKIP "Go yok; derleme ve testler atlanacak" }

$sshExe = Get-Command ssh -ErrorAction SilentlyContinue
if ($sshExe) { Step-Result "OpenSSH istemci (ssh) bulundu" PASS ((& ssh -V 2>&1) -join ' ') }
else { Step-Result "OpenSSH istemci (ssh) bulundu" SKIP "Canlı SSH akışı için gerekli" }

$sshdExe = Get-Command sshd -ErrorAction SilentlyContinue
if ($sshdExe) { Step-Result "OpenSSH sunucu (sshd) bulundu" PASS $sshdExe.Source }
else { Step-Result "OpenSSH sunucu (sshd) bulundu" SKIP "Canlı SSH akışı için gerekli (Ekle: Add-WindowsCapability)" }

New-Item -ItemType Directory -Force -Path $WorkDir | Out-Null

# --- 2) Binary derleme -----------------------------------------------------
$serverExe = Join-Path $WorkDir "guardian-server.exe"
$agentExe = Join-Path $WorkDir "guardian-agent.exe"
if (-not $SkipBuild -and $goExe) {
    try {
        Push-Location (Join-Path $RepoRoot "guardian-server")
        & go build -o $serverExe .
        Pop-Location
        Push-Location (Join-Path $RepoRoot "guardian-agent")
        & go build -o $agentExe .
        Pop-Location
        if ((Test-Path $serverExe) -and (Test-Path $agentExe)) {
            Step-Result "Binary derleme (server + agent)" PASS "$serverExe, $agentExe"
        }
        else { Step-Result "Binary derleme (server + agent)" FAIL "çıktı bulunamadı" }
    }
    catch { Step-Result "Binary derleme (server + agent)" FAIL $_.Exception.Message }
}
else {
    Step-Result "Binary derleme (server + agent)" SKIP "SkipBuild veya Go yok"
}

# --- 3) Sertifika bootstrap (openssl'siz gen-certs) ------------------------
$caCrt = Join-Path $WorkDir "ca.crt"
$serverCrt = Join-Path $WorkDir "server.crt"
$serverKey = Join-Path $WorkDir "server.key"
if (Test-Path $serverExe) {
    try {
        $env:TLS_CA_FILE = $caCrt
        $env:TLS_CA_KEY_FILE = (Join-Path $WorkDir "ca.key")
        $env:TLS_CERT_FILE = $serverCrt
        $env:TLS_KEY_FILE = $serverKey
        $env:GUARDIAN_CERT_HOSTS = "127.0.0.1,localhost"
        & $serverExe gen-certs --force | Out-Null

        if ((Test-Path $caCrt) -and (Test-Path $serverCrt) -and (Test-Path $serverKey)) {
            Step-Result "gen-certs dosya üretimi" PASS "ca.crt, server.crt, server.key"

            # server.crt gerçekten ca.crt tarafından mı imzalanmış + SAN 127.0.0.1?
            $ca = [System.Security.Cryptography.X509Certificates.X509Certificate2]::CreateFromPemFile($caCrt)
            $srv = [System.Security.Cryptography.X509Certificates.X509Certificate2]::CreateFromPemFile($serverCrt)
            $chain = [System.Security.Cryptography.X509Certificates.X509Chain]::new()
            $chain.ChainPolicy.RevocationMode = 'NoCheck'
            $chain.ChainPolicy.VerificationFlags = 'AllowUnknownCertificateAuthority'
            $chain.ChainPolicy.ExtraStore.Add($ca) | Out-Null
            $built = $chain.Build($srv)
            $signedByCA = $false
            foreach ($el in $chain.ChainElements) {
                if ($el.Certificate.Thumbprint -eq $ca.Thumbprint) { $signedByCA = $true }
            }
            if ($built -and $signedByCA) { Step-Result "server.crt ca.crt tarafından imzalı" PASS $srv.Subject }
            else { Step-Result "server.crt ca.crt tarafından imzalı" FAIL "zincir doğrulanamadı" }

            $san = ($srv.Extensions | Where-Object { $_.Oid.Value -eq '2.5.29.17' })
            if ($san -and ($san.Format($false) -match '127\.0\.0\.1')) {
                Step-Result "server.crt SAN 127.0.0.1 içeriyor" PASS $san.Format($false)
            }
            else { Step-Result "server.crt SAN 127.0.0.1 içeriyor" FAIL "SAN bulunamadı" }
        }
        else { Step-Result "gen-certs dosya üretimi" FAIL "beklenen cert dosyaları yok" }
    }
    catch { Step-Result "gen-certs sertifika bootstrap" FAIL $_.Exception.Message }
}
else {
    Step-Result "gen-certs sertifika bootstrap" SKIP "server.exe yok"
}

# --- 4) Birim testler (Windows build-tag'li testler dahil) -----------------
if (-not $SkipTests -and $goExe) {
    foreach ($mod in @("guardian-agent", "guardian-server")) {
        try {
            Push-Location (Join-Path $RepoRoot $mod)
            & go test ./... 2>&1 | Tee-Object -Variable testOut | Out-Null
            $code = $LASTEXITCODE
            Pop-Location
            if ($code -eq 0) { Step-Result "go test ($mod)" PASS "tüm paketler ok" }
            else { Step-Result "go test ($mod)" FAIL (($testOut | Select-Object -Last 5) -join "`n") }
        }
        catch { Step-Result "go test ($mod)" FAIL $_.Exception.Message }
    }
}
else {
    Step-Result "go test (agent + server)" SKIP "SkipTests veya Go yok"
}

# --- Özet ------------------------------------------------------------------
Write-Host ""
Write-Host ("=== Özet: {0} PASS, {1} FAIL, {2} SKIP ===" -f $script:pass, $script:fail, $script:skip) -ForegroundColor Cyan

Write-Host ""
Write-Host "--- ELLE DOĞRULAMA KONTROL LİSTESİ (canlı uçtan uca) ---" -ForegroundColor Cyan
@"
Aşağıdaki akış PostgreSQL + OpenSSH sshd gerektirir; otomatik kapsamda değildir:

  [ ] PostgreSQL kur ve GUARDIAN_DATABASE_URL / server.conf'u ayarla.
  [ ] server.conf (GUARDIAN_SERVER_CONFIG) içine TLS_* yollarını ve DB'yi yaz.
  [ ] scripts\install-server-windows.ps1 ile guardian-server servisini kur ve başlat.
  [ ] https://127.0.0.1:5555/api/agent/ca.crt 200 dönüyor mu (TLS ayakta)?
  [ ] UI'dan bir sunucu ekle; Agent Kurulumu > Windows > PowerShell tek-satır komutu üret.
  [ ] Hedef Windows'ta OpenSSH Server'ı kur (Add-WindowsCapability OpenSSH.Server),
      sshd servisini başlat.
  [ ] Üretilen install.ps1 komutunu çalıştır: enroll-bundle ile sertifika alınıyor,
      guardian-agent servisi kuruluyor ve başlıyor mu?
  [ ] UI'da erişim kuralı oluştur; ssh ile bağlan (agent proxy ForceCommand).
      administrators_authorized_keys'e anahtar yazılıyor, oturum kaydediliyor mu?
  [ ] Oturumu UI'dan canlı izle + replay; kural süresi dolunca oturum kapanıyor mu?
  [ ] SIGWINCH karşılığı: terminal yeniden boyutlanınca uzak PTY güncelleniyor mu?
"@ | Write-Host -ForegroundColor Gray

if ($script:fail -gt 0) { exit 1 } else { exit 0 }
