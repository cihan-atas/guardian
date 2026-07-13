#Requires -RunAsAdministrator
<#
.SYNOPSIS
    guardian-server.exe'yi bir Windows servisi olarak kurar.

.DESCRIPTION
    Bu script şunları yapar:
      1) C:\ProgramData\guardian ve certs alt dizinlerini oluşturur.
      2) guardian-server.exe'yi kurulum dizinine kopyalar.
      3) server.conf yoksa örnek bir şablon yazar (elle doldurulacak).
      4) GUARDIAN_SERVER_CONFIG makine ortam değişkenini ayarlar
         (Windows'ta systemd EnvironmentFile olmadığından config kaynağı budur).
      5) "guardian-server" adında, otomatik başlatmalı bir Windows servisi kurar.

    Sertifika üretimi bu script'in KAPSAMINDA DEĞİLDİR; docs/server-setup-windows.md
    içindeki "Sertifika Bootstrap" bölümünü izleyin (guardian-server gen-certs veya
    Git Bash/WSL ile generate-certs.sh).

.PARAMETER BinarySource
    Kopyalanacak guardian-server.exe'nin kaynak yolu. Verilmezse script'in
    bulunduğu dizinde ya da geçerli dizinde aranır.

.PARAMETER InstallDir
    Kurulum dizini. Varsayılan: C:\ProgramData\guardian

.PARAMETER ServiceName
    Windows servis adı. Varsayılan: guardian-server

.PARAMETER StartService
    Kurulumdan sonra servisi başlat (varsayılan: açık). Kapatmak için:
    -StartService:$false

.EXAMPLE
    .\install-server-windows.ps1 -BinarySource C:\build\guardian-server.exe

.NOTES
    Yönetici (Administrator) PowerShell'de çalıştırılmalıdır.
#>

param(
    [string]$BinarySource = "",
    [string]$InstallDir   = (Join-Path $env:ProgramData "guardian"),
    [string]$ServiceName  = "guardian-server",
    [bool]$StartService   = $true
)

$ErrorActionPreference = "Stop"

$CertDir = Join-Path $InstallDir "certs"
$Conf    = Join-Path $InstallDir "server.conf"
$Bin     = Join-Path $InstallDir "guardian-server.exe"

Write-Host "[guardian-server] 1/6  Dizinler olusturuluyor..."
New-Item -ItemType Directory -Force -Path $InstallDir, $CertDir | Out-Null

Write-Host "[guardian-server] 2/6  Binary bulunuyor/kopyalaniyor..."
if (-not $BinarySource) {
    # Once script dizini, sonra gecerli dizin denenir.
    $candidates = @(
        (Join-Path $PSScriptRoot "guardian-server.exe"),
        (Join-Path (Get-Location) "guardian-server.exe")
    )
    foreach ($c in $candidates) {
        if (Test-Path $c) { $BinarySource = $c; break }
    }
}
if (-not $BinarySource -or -not (Test-Path $BinarySource)) {
    throw "guardian-server.exe bulunamadi. -BinarySource ile yolunu belirtin."
}
# Servis calisiyorsa binary kilitli olabilir; once durdur.
$existing = Get-Service $ServiceName -ErrorAction SilentlyContinue
if ($existing -and $existing.Status -eq "Running") {
    Write-Host "    Mevcut servis durduruluyor (binary guncellemesi icin)..."
    Stop-Service $ServiceName -ErrorAction SilentlyContinue
    Start-Sleep -Milliseconds 500
}
Copy-Item -Force $BinarySource $Bin

Write-Host "[guardian-server] 3/6  server.conf kontrol ediliyor..."
if (-not (Test-Path $Conf)) {
    # Ornek sablon; DEGERLERI ELLE DOLDURUN. Zaten varsa dokunulmaz.
    $confTemplate = @"
# Guardian Server yapilandirmasi (Windows)
# Konum: $Conf
# NOT: Degerleri doldurun; satir basi # ile yorum.

# --- Veritabani ---
POSTGRES_USER=guardian_user
POSTGRES_PASSWORD=DEGISTIR
POSTGRES_DB=guardian_db
POSTGRES_HOST=localhost
POSTGRES_PORT=5432

# --- Ag portlari ---
GUARDIAN_SERVER_PORT=5555
GUARDIAN_AGENT_PORT=6666

# --- Guvenlik ---
GUARDIAN_SECRET_TOKEN=DEGISTIR-super-gizli-token
GUARDIAN_ADMIN_USERNAME=admin
GUARDIAN_ADMIN_PASSWORD=DEGISTIR-guclu-parola

# --- TLS sertifikalari ---
TLS_CA_FILE=$CertDir\ca.crt
TLS_CERT_FILE=$CertDir\server.crt
TLS_KEY_FILE=$CertDir\server.key
# Agent oto-kurulumu (enroll-bundle) kullanilacaksa CA ozel anahtari da gerekir:
# TLS_CA_KEY_FILE=$CertDir\ca.key

# --- Agent binary (Windows hedeflere oto-kurulum icin) ---
# GUARDIAN_AGENT_BINARY_PATH_WINDOWS=$InstallDir\guardian-agent.exe
"@
    Set-Content -Path $Conf -Value $confTemplate -Encoding UTF8
    Write-Host "    Ornek server.conf yazildi: $Conf"
    Write-Host "    ONEMLI: Servisi baslatmadan once bu dosyadaki DEGISTIR alanlarini doldurun." -ForegroundColor Yellow
} else {
    Write-Host "    Mevcut server.conf korundu: $Conf"
}

Write-Host "[guardian-server] 4/6  GUARDIAN_SERVER_CONFIG makine env'i ayarlaniyor..."
# Servis, config'i bu env'in gosterdigi dosyadan okur (config_file.go).
[System.Environment]::SetEnvironmentVariable("GUARDIAN_SERVER_CONFIG", $Conf, "Machine")

Write-Host "[guardian-server] 5/6  Windows servisi kaydediliyor..."
$svc = Get-Service $ServiceName -ErrorAction SilentlyContinue
if (-not $svc) {
    # ProgramData yolunda bosluk yoktur; binPath tek arguman olarak gecer.
    # guardian-server ayri bir alt-komut olmadan (arg'siz) sunucu modunda calisir.
    New-Service -Name $ServiceName -BinaryPathName "`"$Bin`"" `
        -DisplayName "Guardian Server" -StartupType Automatic | Out-Null
    Write-Host "    Servis olusturuldu: $ServiceName"
} else {
    # Var olan servisin binPath'ini ve otomatik baslatmasini guncelle.
    & sc.exe config $ServiceName binPath= "`"$Bin`"" start= auto | Out-Null
    Write-Host "    Mevcut servis guncellendi: $ServiceName"
}
# Servis WorkingDirectory'sini New-Service dogrudan desteklemez; servis
# calisma dizini ImagePath'in bulundugu dizindir. Binary InstallDir'de
# oldugundan calisma dizini $InstallDir olur (goreli ../certs yollarina
# bel baglanmaz; server.conf mutlak yollar kullanir).

Write-Host "[guardian-server] 6/6  Servis baslatiliyor..."
if ($StartService) {
    try {
        Start-Service $ServiceName
        Write-Host "    Servis baslatildi." -ForegroundColor Green
    } catch {
        Write-Host "    Servis baslatilamadi: $($_.Exception.Message)" -ForegroundColor Red
        Write-Host "    server.conf, sertifikalar ve PostgreSQL baglantisini kontrol edin." -ForegroundColor Yellow
    }
} else {
    Write-Host "    -StartService:`$false verildi; servis baslatilmadi."
}

Write-Host ""
Write-Host "======================================================================"
Write-Host " guardian-server kuruldu."
Write-Host "   Kurulum dizini : $InstallDir"
Write-Host "   Config         : $Conf"
Write-Host "   Servis         : $ServiceName (StartupType=Automatic)"
Write-Host ""
Write-Host " Yonetim komutlari:"
Write-Host "   Start-Service $ServiceName"
Write-Host "   Restart-Service $ServiceName"
Write-Host "   Get-Service $ServiceName"
Write-Host "   Stop-Service $ServiceName; sc.exe delete $ServiceName   # kaldirma"
Write-Host ""
Write-Host " Sonraki adimlar (bkz. docs/server-setup-windows.md):"
Write-Host "   1) Sertifikalari uretin (guardian-server gen-certs veya generate-certs.sh)"
Write-Host "   2) server.conf'taki DEGISTIR alanlarini doldurun"
Write-Host "   3) PostgreSQL'i kurup schema.sql'i iceri aktarin"
Write-Host "   4) UI icin reverse-proxy (nginx/IIS) yapilandirin"
Write-Host "======================================================================"
Get-Service $ServiceName -ErrorAction SilentlyContinue | Format-Table -AutoSize
