package handlers

import (
	"strconv"
	"strings"
)

// installScriptPSTemplate, Windows hedeflerde `... | iex` ile çalıştırılan
// PowerShell kurulum script'idir. Windows'ta openssl bulunmayabileceğinden
// anahtar+sertifika sunucudan hazır alınır (enroll-bundle). ConPTY'yi Windows
// OpenSSH sshd kendisi sağladığından agent tarafında ek bir şey gerekmez.
const installScriptPSTemplate = `#Requires -RunAsAdministrator
$ErrorActionPreference = "Stop"

$BaseUrl      = "__BASE_URL__"
$ServerId     = "__SERVER_ID__"
$SecretToken  = "__SECRET_TOKEN__"
$EnrollToken  = "__ENROLL_TOKEN__"
$AgentPort    = "__AGENT_PORT__"
$ServerHost   = "__SERVER_HOST__"
$ServerPort   = "__SERVER_PORT__"

$GuardianDir = Join-Path $env:ProgramData "guardian"
$CertDir     = Join-Path $GuardianDir "certs"
$Conf        = Join-Path $GuardianDir "agent.conf"
$SshKey      = Join-Path $GuardianDir "agent_service_key"
$Bin         = Join-Path $GuardianDir "guardian-agent.exe"

# Self-signed iç CA olduğundan indirmelerde sertifika doğrulaması atlanır.
# PowerShell 5.1 ve 7 uyumu:
try { [System.Net.ServicePointManager]::ServerCertificateValidationCallback = { $true } } catch {}
function Invoke-Guardian($Method, $Path, $OutFile, $BodyBytes) {
    $headers = @{ "X-Enroll-Token" = $EnrollToken }
    $uri = "$BaseUrl$Path"
    $params = @{ Method = $Method; Uri = $uri; Headers = $headers; UseBasicParsing = $true }
    if ($PSVersionTable.PSVersion.Major -ge 6) { $params.SkipCertificateCheck = $true }
    if ($OutFile) { $params.OutFile = $OutFile }
    if ($BodyBytes) { $params.Body = $BodyBytes }
    return Invoke-RestMethod @params
}

Write-Host "[guardian] 1/6  Dizinler olusturuluyor..."
New-Item -ItemType Directory -Force -Path $GuardianDir, $CertDir | Out-Null

Write-Host "[guardian] 2/6  Agent SSH servis anahtari uretiliyor..."
if (-not (Test-Path "$SshKey")) {
    & ssh-keygen -t ed25519 -f "$SshKey" -N '""' -C "guardian-agent-key" | Out-Null
}

Write-Host "[guardian] 3/6  Sertifika sunucudan aliniyor (enroll-bundle)..."
$bundle = Invoke-Guardian "POST" "/api/agent/enroll-bundle" $null $null
Set-Content -Path (Join-Path $CertDir "agent.key") -Value $bundle.agent_key -NoNewline
Set-Content -Path (Join-Path $CertDir "agent.crt") -Value $bundle.agent_crt -NoNewline
Set-Content -Path (Join-Path $CertDir "ca.crt")    -Value $bundle.ca_crt    -NoNewline

Write-Host "[guardian] 4/6  agent.conf yaziliyor..."
$confContent = @"
GUARDIAN_SERVER_HOST=$ServerHost
GUARDIAN_SERVER_PORT=$ServerPort
GUARDIAN_AGENT_PORT=$AgentPort
GUARDIAN_AGENT_SERVER_ID=$ServerId
GUARDIAN_SECRET_TOKEN=$SecretToken
TLS_CA_FILE=$CertDir\ca.crt
AGENT_TLS_CERT_FILE=$CertDir\agent.crt
AGENT_TLS_KEY_FILE=$CertDir\agent.key
GUARDIAN_AGENT_SSH_KEY_PATH=$SshKey
GUARDIAN_AGENT_TRUSTED_HOST_KEY=$env:ProgramData\ssh\ssh_host_ed25519_key.pub
"@
Set-Content -Path $Conf -Value $confContent

Write-Host "[guardian] 5/6  Agent binary indiriliyor..."
Invoke-Guardian "GET" "/api/agent/binary?os=windows" "$Bin.new" $null
if (Test-Path $Bin) {
    try { Stop-Service guardian-agent -ErrorAction SilentlyContinue } catch {}
    Start-Sleep -Milliseconds 500
}
Move-Item -Force "$Bin.new" $Bin

Write-Host "[guardian] 6/6  Windows servisi kaydediliyor..."
# ProgramData yolunda bosluk yoktur; "$Bin serve" tek arguman olarak ImagePath'e gecer.
$svc = Get-Service guardian-agent -ErrorAction SilentlyContinue
if (-not $svc) {
    New-Service -Name "guardian-agent" -BinaryPathName "$Bin serve" -DisplayName "Guardian Agent" -StartupType Automatic | Out-Null
}
# Servis config'i env'den okumadigindan GUARDIAN_AGENT_CONFIG'i makine env'ine yaz.
[System.Environment]::SetEnvironmentVariable("GUARDIAN_AGENT_CONFIG", $Conf, "Machine")
Restart-Service guardian-agent -ErrorAction SilentlyContinue
Start-Service guardian-agent -ErrorAction SilentlyContinue

Write-Host ""
Write-Host "======================================================================"
Write-Host " guardian-agent kuruldu ve baslatildi (server_id=$ServerId)."
Write-Host ""
Write-Host " ONEMLI - elle yapilmasi gerekenler:"
Write-Host " 1) OpenSSH Server (sshd) kurulu ve calisir olmali; sshd_config:"
Write-Host "      PubkeyAuthentication yes"
Write-Host "      PermitUserEnvironment yes"
Write-Host " 2) Proxy'lenecek her kullanicinin authorized_keys dosyasina agent"
Write-Host "    public anahtarini ekleyin ($SshKey.pub). Yonetici hesaplar icin"
Write-Host "    %ProgramData%\ssh\administrators_authorized_keys kullanin ve agent"
Write-Host "    tarafinda GUARDIAN_WINDOWS_ADMIN_USERS listesine ekleyin."
Write-Host "======================================================================"
Get-Service guardian-agent | Format-Table -AutoSize
`

// renderInstallScriptPS, PowerShell şablonundaki yer tutucuları doldurur.
func renderInstallScriptPS(baseURL, serverHost, serverPort, agentPort, secretToken, enrollToken string, serverID int) string {
	r := strings.NewReplacer(
		"__BASE_URL__", baseURL,
		"__SERVER_HOST__", serverHost,
		"__SERVER_PORT__", serverPort,
		"__AGENT_PORT__", agentPort,
		"__SECRET_TOKEN__", secretToken,
		"__ENROLL_TOKEN__", enrollToken,
		"__SERVER_ID__", strconv.Itoa(serverID),
	)
	return r.Replace(installScriptPSTemplate)
}
