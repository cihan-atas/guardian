package handlers

import (
	"strconv"
	"strings"
)

// installScriptTemplate, hedef sunucuda `curl ... | sudo bash` ile çalıştırılan
// agent kurulum script'idir. Değerler ReplaceAll ile gömülür.
//
// Script; sistem kullanıcısı/grubu, dizinler, agent SSH servis anahtarı,
// TLS anahtarı + CSR üretir; sunucudan sertifika (enroll) ve ca.crt indirir;
// agent.conf + systemd unit yazar; binary'yi indirir ve servisi başlatır.
// sshd_config'e OTOMATİK dokunmaz (kilitlenme riski) — gerekli ayarları yazdırır.
const installScriptTemplate = `#!/usr/bin/env bash
set -euo pipefail

# ==== Guardian Agent Kurulum Script'i (otomatik üretildi) ====
BASE_URL="__BASE_URL__"
SERVER_ID="__SERVER_ID__"
SECRET_TOKEN="__SECRET_TOKEN__"
ENROLL_TOKEN="__ENROLL_TOKEN__"
AGENT_PORT="__AGENT_PORT__"
SERVER_HOST="__SERVER_HOST__"
SERVER_PORT="__SERVER_PORT__"

CERT_DIR="/etc/guardian/certs"
CONF="/etc/guardian/agent.conf"
SSH_KEY="/etc/guardian/agent_service_key"
BIN="/usr/local/bin/guardian-agent"

say() { printf "\033[0;34m[guardian]\033[0m %s\n" "$1"; }
err() { printf "\033[0;31m[guardian][HATA]\033[0m %s\n" "$1" >&2; }

if [ "$(id -u)" -ne 0 ]; then err "Bu script root ile çalıştırılmalıdır (sudo)."; exit 1; fi
for c in curl openssl ssh-keygen; do
  command -v "$c" >/dev/null 2>&1 || { err "'$c' bulunamadı, lütfen kurun."; exit 1; }
done

CURL="curl -fsSL"
# İç PKI (self-signed) olduğundan sunucu sertifikası CA'ya bağlıdır; kurulum
# anında CA henüz yerel değil, bu yüzden indirmelerde -k kullanılır. İndirilen
# ca.crt sonrasında agent tarafından doğrulanır.
CURLK="curl -fsSLk"

say "1/8  Sistem kullanıcısı ve grubu (guardian) hazırlanıyor…"
getent group guardian >/dev/null || groupadd --system guardian
getent passwd guardian >/dev/null || useradd --system -g guardian --no-create-home -s /usr/sbin/nologin -c "Guardian Agent Account" guardian

say "2/8  Dizinler oluşturuluyor…"
mkdir -p /opt/guardian/pids "$CERT_DIR"

say "3/8  Agent SSH servis anahtarı üretiliyor…"
if [ ! -f "$SSH_KEY" ]; then
  ssh-keygen -t ed25519 -f "$SSH_KEY" -N "" -C "guardian-agent-key" >/dev/null
fi

say "4/8  TLS anahtarı + CSR üretiliyor, sunucuya imzalatılıyor…"
HOSTNAME_FQDN="$(hostname -f 2>/dev/null || hostname)"
openssl genpkey -algorithm RSA -out "$CERT_DIR/agent.key" 2>/dev/null
openssl req -new -key "$CERT_DIR/agent.key" -out /tmp/guardian-agent.csr \
  -subj "/C=TR/O=Guardian Self-Signed Agent/CN=${HOSTNAME_FQDN}" 2>/dev/null

# CSR'ı sunucuya gönder → imzalı agent.crt al
$CURLK -X POST \
  -H "X-Enroll-Token: ${ENROLL_TOKEN}" \
  --data-binary @/tmp/guardian-agent.csr \
  "${BASE_URL}/api/agent/enroll" -o "$CERT_DIR/agent.crt"
# CA sertifikasını indir
$CURLK -H "X-Enroll-Token: ${ENROLL_TOKEN}" "${BASE_URL}/api/agent/ca.crt" -o "$CERT_DIR/ca.crt"
rm -f /tmp/guardian-agent.csr
if [ ! -s "$CERT_DIR/agent.crt" ] || ! grep -q "BEGIN CERTIFICATE" "$CERT_DIR/agent.crt"; then
  err "Sertifika imzalama başarısız (enroll token süresi dolmuş olabilir)."; exit 1
fi

say "5/8  agent.conf yazılıyor…"
cat > "$CONF" <<EOF
GUARDIAN_SERVER_HOST=${SERVER_HOST}
GUARDIAN_SERVER_PORT=${SERVER_PORT}
GUARDIAN_AGENT_PORT=${AGENT_PORT}
GUARDIAN_AGENT_SERVER_ID=${SERVER_ID}
GUARDIAN_SECRET_TOKEN=${SECRET_TOKEN}
TLS_CA_FILE=${CERT_DIR}/ca.crt
AGENT_TLS_CERT_FILE=${CERT_DIR}/agent.crt
AGENT_TLS_KEY_FILE=${CERT_DIR}/agent.key
GUARDIAN_AGENT_SSH_KEY_PATH=${SSH_KEY}
GUARDIAN_AGENT_TRUSTED_HOST_KEY=/etc/ssh/ssh_host_ed25519_key.pub
EOF

say "6/8  Agent binary indiriliyor…"
# Çalışan bir agent varsa binary'nin üzerine doğrudan yazmak "Text file busy"
# (ETXTBSY) verir; temp'e indirip mv (rename) ile değiştiriyoruz.
$CURLK -H "X-Enroll-Token: ${ENROLL_TOKEN}" "${BASE_URL}/api/agent/binary" -o "${BIN}.new"
chmod +x "${BIN}.new"
mv -f "${BIN}.new" "$BIN"

say "7/8  İzinler ve systemd servisi ayarlanıyor…"
chown -R guardian:guardian /opt/guardian /etc/guardian
chmod 770 /etc/guardian
chmod 640 "$SSH_KEY" "$SSH_KEY.pub" "$CONF"
chmod 750 "$CERT_DIR"; chmod 640 "$CERT_DIR"/* 2>/dev/null || true

# Root için agent public anahtarını authorized_keys'e ekle (idempotent)
AGENT_PUB="$(cat "$SSH_KEY.pub")"
mkdir -p /root/.ssh
grep -qxF "$AGENT_PUB" /root/.ssh/authorized_keys 2>/dev/null || echo "$AGENT_PUB" >> /root/.ssh/authorized_keys
chmod 700 /root/.ssh && chmod 600 /root/.ssh/authorized_keys

cat > /etc/systemd/system/guardian-agent.service <<'EOF'
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
EOF

systemctl daemon-reload
systemctl enable guardian-agent.service >/dev/null 2>&1 || true
systemctl restart guardian-agent.service

say "8/8  Kurulum tamamlandı."
echo ""
echo "======================================================================"
echo " ✅ guardian-agent kuruldu ve başlatıldı (server_id=${SERVER_ID})."
echo ""
echo " ÖNEMLİ — elle yapılması gerekenler:"
echo " 1) sshd_config'e şu satırları ekleyin ve 'systemctl restart ssh' yapın:"
echo "      PermitUserEnvironment yes"
echo "      PubkeyAuthentication yes"
echo "      HostKey /etc/ssh/ssh_host_ed25519_key"
echo " 2) Proxy'lenecek her sistem kullanıcısı için agent anahtarını ekleyin:"
echo "      echo '${AGENT_PUB}' >> /home/<kullanici>/.ssh/authorized_keys"
echo "      usermod -a -G guardian <kullanici>"
echo "======================================================================"
systemctl --no-pager status guardian-agent.service | head -n 5 || true
`

// renderInstallScript, şablondaki yer tutucuları verilen değerlerle doldurur.
func renderInstallScript(baseURL, serverHost, serverPort, agentPort, secretToken, enrollToken string, serverID int) string {
	r := strings.NewReplacer(
		"__BASE_URL__", baseURL,
		"__SERVER_HOST__", serverHost,
		"__SERVER_PORT__", serverPort,
		"__AGENT_PORT__", agentPort,
		"__SECRET_TOKEN__", secretToken,
		"__ENROLL_TOKEN__", enrollToken,
		"__SERVER_ID__", strconv.Itoa(serverID),
	)
	return r.Replace(installScriptTemplate)
}
