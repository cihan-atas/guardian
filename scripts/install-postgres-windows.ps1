#Requires -Version 5.1
<#
.SYNOPSIS
    Guardian için Windows'ta PostgreSQL veritabanını hazırlar (rol + veritabanı +
    schema.sql uygulama + isteğe bağlı servis ortam değişkenleri).

.DESCRIPTION
    guardian-server, temel tabloları (servers, system_users, public_keys,
    access_rules, sessions, session_events, audit_logs) repo kökündeki schema.sql
    ile bekler; yardımcı tabloları (auth, settings, alerts, enroll, key_bans)
    açılışta kendi oluşturur. Linux'ta bu, docker-compose'un schema.sql'i Postgres
    initdb'ye bağlamasıyla oluyordu. Bu betik aynı hazırlığı Windows'ta yapar:

      1) PostgreSQL istemcisini (psql) bulur; yoksa ve Chocolatey varsa
         (-InstallIfMissing ile) `choco install postgresql` dener.
      2) Süper kullanıcı (varsayılan: postgres) ile bağlanır.
      3) Guardian rolünü (LOGIN + parola) ve veritabanını idempotent oluşturur.
      4) schema.sql'i YALNIZCA temel tablolar henüz yoksa uygular (tek seferlik
         init; schema.sql'deki CREATE TABLE'lar IF NOT EXISTS içermez, yeniden
         uygulanamaz).
      5) -SetServiceEnv verilirse POSTGRES_* makine ortam değişkenlerini ayarlar
         (guardian-server servisinin okuması için).

    Betik idempotenttir: rol/DB zaten varsa yeniden oluşturmaz; temel tablolar
    zaten varsa schema.sql'i tekrar UYGULAMAZ (aksi halde hata verirdi).

.PARAMETER DbName          Guardian veritabanı adı (varsayılan: guardian).
.PARAMETER DbUser          Guardian rol adı (varsayılan: guardian).
.PARAMETER DbPassword      Guardian rol parolası (zorunlu; verilmezse sorulur).
.PARAMETER DbHost          Sunucu adresi (varsayılan: 127.0.0.1).
.PARAMETER DbPort          Port (varsayılan: 5432).
.PARAMETER SuperUser       Postgres süper kullanıcı (varsayılan: postgres).
.PARAMETER SuperPassword   Süper kullanıcı parolası (verilmezse sorulur).
.PARAMETER SchemaPath      schema.sql yolu (varsayılan: repo kökü\schema.sql).
.PARAMETER PsqlPath        psql.exe yolu (verilmezse PATH + standart dizinler taranır).
.PARAMETER InstallIfMissing  psql yoksa Chocolatey ile kurmayı dene.
.PARAMETER SetServiceEnv   POSTGRES_* makine ortam değişkenlerini ayarla.

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File .\scripts\install-postgres-windows.ps1 `
        -DbPassword 'S3cret!' -SuperPassword 'postgres-parolasi' -SetServiceEnv
#>
[CmdletBinding()]
param(
    [string]$DbName = "guardian",
    [string]$DbUser = "guardian",
    [string]$DbPassword,
    [string]$DbHost = "127.0.0.1",
    [int]$DbPort = 5432,
    [string]$SuperUser = "postgres",
    [string]$SuperPassword,
    [string]$SchemaPath = (Join-Path (Split-Path -Parent $PSScriptRoot) "schema.sql"),
    [string]$PsqlPath,
    [switch]$InstallIfMissing,
    [switch]$SetServiceEnv
)

$ErrorActionPreference = 'Stop'

function Resolve-Psql {
    param([string]$Explicit)
    if ($Explicit -and (Test-Path $Explicit)) { return $Explicit }
    $cmd = Get-Command psql -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    # Standart kurulum dizinleri: C:\Program Files\PostgreSQL\<sürüm>\bin\psql.exe
    $candidates = Get-ChildItem "C:\Program Files\PostgreSQL" -Directory -ErrorAction SilentlyContinue |
        Sort-Object Name -Descending |
        ForEach-Object { Join-Path $_.FullName "bin\psql.exe" } |
        Where-Object { Test-Path $_ }
    if ($candidates) { return $candidates[0] }
    return $null
}

# Bir SQL komutunu belirtilen veritabanına karşı süper kullanıcı ile çalıştırır.
function Invoke-Psql {
    param([string]$Psql, [string]$Database, [string]$Sql, [string]$Password)
    $prevPw = $env:PGPASSWORD
    $env:PGPASSWORD = $Password
    try {
        $out = & $Psql -h $DbHost -p $DbPort -U $SuperUser -d $Database -v ON_ERROR_STOP=1 -tAc $Sql 2>&1
        if ($LASTEXITCODE -ne 0) { throw "psql hata (db=$Database): $out" }
        return ($out -join "`n").Trim()
    }
    finally { $env:PGPASSWORD = $prevPw }
}

# --- 1) psql'i bul ---------------------------------------------------------
$psql = Resolve-Psql -Explicit $PsqlPath
if (-not $psql) {
    if ($InstallIfMissing -and (Get-Command choco -ErrorAction SilentlyContinue)) {
        Write-Host "psql bulunamadı; Chocolatey ile PostgreSQL kuruluyor..." -ForegroundColor Yellow
        & choco install postgresql -y
        $psql = Resolve-Psql
    }
}
if (-not $psql) {
    throw "psql.exe bulunamadı. PostgreSQL'i kurun (https://www.postgresql.org/download/windows/) veya -PsqlPath verin. Chocolatey varsa -InstallIfMissing kullanabilirsiniz."
}
Write-Host ("psql : {0}" -f $psql) -ForegroundColor Green

# --- 2) Parolalar ----------------------------------------------------------
if (-not $SuperPassword) {
    $sec = Read-Host "Süper kullanıcı ($SuperUser) parolası" -AsSecureString
    $SuperPassword = [Runtime.InteropServices.Marshal]::PtrToStringAuto([Runtime.InteropServices.Marshal]::SecureStringToBSTR($sec))
}
if (-not $DbPassword) {
    $sec = Read-Host "Guardian rolü ($DbUser) için yeni parola" -AsSecureString
    $DbPassword = [Runtime.InteropServices.Marshal]::PtrToStringAuto([Runtime.InteropServices.Marshal]::SecureStringToBSTR($sec))
}

# --- 3) Bağlantı testi -----------------------------------------------------
$ver = Invoke-Psql -Psql $psql -Database "postgres" -Password $SuperPassword -Sql "SHOW server_version;"
Write-Host ("PostgreSQL sürümü: {0}" -f $ver) -ForegroundColor Green

# --- 4) Rol (idempotent) ---------------------------------------------------
$roleExists = Invoke-Psql -Psql $psql -Database "postgres" -Password $SuperPassword `
    -Sql "SELECT 1 FROM pg_roles WHERE rolname = '$DbUser';"
$escapedPw = $DbPassword.Replace("'", "''")
if ($roleExists -eq "1") {
    Invoke-Psql -Psql $psql -Database "postgres" -Password $SuperPassword `
        -Sql "ALTER ROLE `"$DbUser`" WITH LOGIN PASSWORD '$escapedPw';" | Out-Null
    Write-Host ("Rol '{0}' zaten var; parola güncellendi." -f $DbUser) -ForegroundColor Green
}
else {
    Invoke-Psql -Psql $psql -Database "postgres" -Password $SuperPassword `
        -Sql "CREATE ROLE `"$DbUser`" WITH LOGIN PASSWORD '$escapedPw';" | Out-Null
    Write-Host ("Rol '{0}' oluşturuldu." -f $DbUser) -ForegroundColor Green
}

# --- 5) Veritabanı (idempotent; CREATE DATABASE IF NOT EXISTS yok) ----------
$dbExists = Invoke-Psql -Psql $psql -Database "postgres" -Password $SuperPassword `
    -Sql "SELECT 1 FROM pg_database WHERE datname = '$DbName';"
if ($dbExists -eq "1") {
    Write-Host ("Veritabanı '{0}' zaten var." -f $DbName) -ForegroundColor Green
}
else {
    Invoke-Psql -Psql $psql -Database "postgres" -Password $SuperPassword `
        -Sql "CREATE DATABASE `"$DbName`" OWNER `"$DbUser`";" | Out-Null
    Write-Host ("Veritabanı '{0}' oluşturuldu (sahip: {1})." -f $DbName, $DbUser) -ForegroundColor Green
}

# --- 6) schema.sql uygula (yalnızca temel tablolar yoksa) ------------------
if (-not (Test-Path $SchemaPath)) { throw "schema.sql bulunamadı: $SchemaPath" }
# access_rules temel tablosu var mı? (to_regclass NULL => yok)
$baseExists = Invoke-Psql -Psql $psql -Database $DbName -Password $SuperPassword `
    -Sql "SELECT to_regclass('public.access_rules') IS NOT NULL;"
if ($baseExists -eq "t") {
    Write-Host "Temel tablolar zaten mevcut; schema.sql yeniden uygulanmıyor (tek seferlik)." -ForegroundColor Green
}
else {
    $prevPw = $env:PGPASSWORD
    $env:PGPASSWORD = $SuperPassword
    try {
        & $psql -h $DbHost -p $DbPort -U $SuperUser -d $DbName -v ON_ERROR_STOP=1 -f $SchemaPath
        if ($LASTEXITCODE -ne 0) { throw "schema.sql uygulanamadı." }
    }
    finally { $env:PGPASSWORD = $prevPw }
    Write-Host "schema.sql uygulandı (temel tablolar oluşturuldu)." -ForegroundColor Green
}
# Guardian rolüne public şema üzerindeki tablo/sequence yetkilerini ver (idempotent).
Invoke-Psql -Psql $psql -Database $DbName -Password $SuperPassword `
    -Sql "GRANT ALL ON ALL TABLES IN SCHEMA public TO `"$DbUser`"; GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO `"$DbUser`"; GRANT ALL ON SCHEMA public TO `"$DbUser`";" | Out-Null
Write-Host "public şema yetkileri guardian rolüne verildi." -ForegroundColor Green

# --- 7) Bağlantıyı guardian rolüyle doğrula --------------------------------
$prevPw = $env:PGPASSWORD
$env:PGPASSWORD = $DbPassword
try {
    $tblCount = & $psql -h $DbHost -p $DbPort -U $DbUser -d $DbName -tAc "SELECT count(*) FROM information_schema.tables WHERE table_schema='public';" 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-Host ("Guardian rolüyle bağlantı OK; public tablo sayısı: {0}" -f ($tblCount -join '').Trim()) -ForegroundColor Green
    }
    else {
        Write-Warning ("Guardian rolüyle bağlantı doğrulanamadı: {0}" -f ($tblCount -join ' '))
    }
}
finally { $env:PGPASSWORD = $prevPw }

# --- 8) Servis ortam değişkenleri ------------------------------------------
if ($SetServiceEnv) {
    [Environment]::SetEnvironmentVariable("POSTGRES_USER", $DbUser, "Machine")
    [Environment]::SetEnvironmentVariable("POSTGRES_PASSWORD", $DbPassword, "Machine")
    [Environment]::SetEnvironmentVariable("POSTGRES_DB", $DbName, "Machine")
    [Environment]::SetEnvironmentVariable("POSTGRES_HOST", $DbHost, "Machine")
    [Environment]::SetEnvironmentVariable("POSTGRES_PORT", "$DbPort", "Machine")
    Write-Host "POSTGRES_* makine ortam değişkenleri ayarlandı (guardian-server için)." -ForegroundColor Green
    Write-Host "Not: Parola makine ortam değişkeninde saklanır; alternatif olarak server.conf (GUARDIAN_SERVER_CONFIG) kullanılabilir." -ForegroundColor DarkGray
}

Write-Host ""
Write-Host "=== Tamamlandı ===" -ForegroundColor Cyan
Write-Host ("DSN: postgres://{0}:***@{1}:{2}/{3}?sslmode=disable" -f $DbUser, $DbHost, $DbPort, $DbName)
Write-Host "guardian-server bu değişkenleri okur: POSTGRES_USER/PASSWORD/DB/HOST/PORT."
