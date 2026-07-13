#!/bin/bash

# Renk kodları
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Herhangi bir komut başarısız olursa betiği durdur
set -e

# --- DEĞİŞKENLER ---
CERT_DIR="${GUARDIAN_CERT_DIR:-certs}"
DAYS_VALID="${GUARDIAN_CERT_DAYS:-3650}" # Sertifikaların geçerlilik süresi (10 yıl)

# Subject/DN'in sabit (kuruluş) alanları — tek yerden yönetilir.
# İstenirse ortam değişkenleriyle ezilebilir.
CERT_COUNTRY="${GUARDIAN_CERT_COUNTRY:-TR}"
CERT_STATE="${GUARDIAN_CERT_STATE:-Istanbul}"
CERT_LOCALITY="${GUARDIAN_CERT_LOCALITY:-Istanbul}"
CERT_ORG="${GUARDIAN_CERT_ORG:-Guardian Self-Signed}"
CERT_ORG_AGENT="${GUARDIAN_CERT_ORG_AGENT:-Guardian Self-Signed Agent}"

# Etkileşimsiz (non-interactive) çalışma için bayraklar.
# FORCE=1  -> yıkıcı işlemleri (certs dizinini silme / cert üzerine yazma) onaysız yapar.
FORCE="${GUARDIAN_CERT_FORCE:-0}"

# Etkileşimsiz alan değerleri GUARDIAN_ önekiyle verilir; betik içindeki kısa
# değişkenlere (ask() bunları kontrol eder) burada aktarılır.
CA_CN="${GUARDIAN_CA_CN:-}"
SERVER_CN="${GUARDIAN_SERVER_CN:-}"
SERVER_IP="${GUARDIAN_SERVER_IP:-}"

# --- YARDIMCI FONKSİYONLAR ---
#
# KÖK NEDEN NOTU (eski davranış):
#   Eski sürüm interaktif alanları `read -p` (ve onay için `read -n 1`) ile
#   okuyordu. Betiğe pipe/heredoc ile besleme yapıldığında `read -n 1` yalnızca
#   TEK karakter tükettiği için satır sonundaki '\n' tamponda kalıyor; sonraki
#   `read` bu boş satırı yakalıyor ve tüm girdi bir satır kayıyordu. Sonuç:
#   CA için verilen değer boş kalıp Server alanına kayıyor, alanlar "karışıyordu".
#
# ÇÖZÜM:
#   Tüm alanlar `ask()` üzerinden alınır. Öncelik sırası:
#     1) Ortam değişkeni doluysa onu kullan (etkileşimsiz, karışma imkânsız).
#     2) stdin bir TTY ise tam satır (`read -r`) sorulur.
#     3) Aksi halde (pipe/CI) varsayılan kullanılır — pipe'tan satır okunmaz.
#   Böylece pipe'lı besleme kaynaklı hizalama/alan karışması ortadan kalkar.

# ask <DEGISKEN_ADI> <PROMPT> <VARSAYILAN>
ask() {
    local __var="$1"
    local __prompt="$2"
    local __default="$3"
    local __input=""

    # 1) Ortamdan gelen değer varsa dokunma.
    if [ -n "${!__var}" ]; then
        return 0
    fi

    # 2) TTY ise interaktif sor, değilse varsayılana düş.
    if [ -t 0 ]; then
        read -r -p "$__prompt" __input || __input=""
    fi

    if [ -z "$__input" ]; then
        __input="$__default"
    fi

    printf -v "$__var" '%s' "$__input"
}

# confirm <PROMPT> -> 0 (evet) / 1 (hayır)
confirm() {
    if [ "$FORCE" = "1" ]; then
        return 0
    fi
    # Etkileşimsiz ortamda FORCE yoksa yıkıcı işleme İZİN VERME.
    if [ ! -t 0 ]; then
        echo -e "${RED}Etkileşimsiz ortam: onay alınamıyor. Yıkıcı işlem atlandı (FORCE=1 veya --force ile zorlayabilirsiniz).${NC}"
        return 1
    fi
    local __reply=""
    read -r -p "$1" __reply || __reply=""
    [[ $__reply =~ ^[Ee]$ ]]
}

# subject_dn <ORG> <CN> -> "/C=.../ST=.../L=.../O=.../CN=..."
subject_dn() {
    local __org="$1"
    local __cn="$2"
    printf '/C=%s/ST=%s/L=%s/O=%s/CN=%s' \
        "$CERT_COUNTRY" "$CERT_STATE" "$CERT_LOCALITY" "$__org" "$__cn"
}

# write_ext <CIKTI_DOSYASI> <CN> <IP>
# SAN eklentisini yazar. IP boş veya 127.0.0.1 ise yalnızca IP.1 kalır.
write_ext() {
    local __out="$1"
    local __cn="$2"
    local __ip="$3"
    {
        echo "authorityKeyIdentifier=keyid,issuer"
        echo "basicConstraints=CA:FALSE"
        echo "keyUsage = digitalSignature, keyEncipherment"
        echo "subjectAltName = @alt_names"
        echo "[alt_names]"
        echo "DNS.1 = ${__cn}"
        echo "IP.1 = 127.0.0.1"
        if [ -n "$__ip" ] && [ "$__ip" != "127.0.0.1" ]; then
            echo "IP.2 = ${__ip}"
        fi
    } > "$__out"
}

# --- ANA FONKSİYONLAR ---

# CA ve Sunucu sertifikalarını oluşturan fonksiyon
create_ca_and_server() {
    echo ""
    echo -e "${BLUE}--- Adım 1: Kök Sertifika Otoritesi (CA) Oluşturma ---${NC}"

    # CA zaten varsa üzerine yazmaya karşı koru.
    if [ -f "$CERT_DIR/ca.key" ] || [ -f "$CERT_DIR/ca.crt" ]; then
        if ! confirm "$(echo -e ${RED}"UYARI: Mevcut CA (ca.key/ca.crt) üzerine yazılacak. Emin misiniz? (e/h): "${NC})"; then
            echo "CA oluşturma atlandı."
            return 1
        fi
    fi

    ask CA_CN "CA için bir 'Common Name' girin (örn: Guardian Root CA): " "Guardian Root CA"
    local CA_SUBJ
    CA_SUBJ="$(subject_dn "$CERT_ORG" "$CA_CN")"

    echo "CA özel anahtarı (ca.key) oluşturuluyor..."
    openssl genpkey -algorithm RSA -out "$CERT_DIR/ca.key"

    echo "Kendinden imzalı CA sertifikası (ca.crt) oluşturuluyor..."
    openssl req -x509 -new -nodes -key "$CERT_DIR/ca.key" -sha256 -days "$DAYS_VALID" \
        -out "$CERT_DIR/ca.crt" -subj "$CA_SUBJ"
    echo -e "${GREEN}  -> ca.key ve ca.crt başarıyla oluşturuldu.${NC}"

    echo ""
    echo -e "${BLUE}--- Adım 2: Guardian Server Sertifikası Oluşturma ---${NC}"
    ask SERVER_CN "Server için bir 'Common Name' girin (genellikle DNS adı, varsayılan: localhost): " "localhost"
    ask SERVER_IP "Server için DIŞ IP adresini girin (örn: 10.2.60.185): " ""
    local SERVER_SUBJ
    SERVER_SUBJ="$(subject_dn "$CERT_ORG" "$SERVER_CN")"

    echo "Server özel anahtarı (server.key) oluşturuluyor..."
    openssl genpkey -algorithm RSA -out "$CERT_DIR/server.key"

    echo "Server için Sertifika İmzalama Talebi (server.csr) oluşturuluyor..."
    openssl req -new -key "$CERT_DIR/server.key" -out "$CERT_DIR/server.csr" -subj "$SERVER_SUBJ"

    write_ext "$CERT_DIR/server.ext" "$SERVER_CN" "$SERVER_IP"

    echo "Server sertifikası (server.crt) CA ile imzalanıyor..."
    openssl x509 -req -in "$CERT_DIR/server.csr" -CA "$CERT_DIR/ca.crt" -CAkey "$CERT_DIR/ca.key" \
        -CAcreateserial -out "$CERT_DIR/server.crt" -days "$DAYS_VALID" -sha256 -extfile "$CERT_DIR/server.ext"
    echo -e "${GREEN}  -> server.key ve server.crt başarıyla oluşturuldu.${NC}"
}

# Tek bir agent sertifikası üretir.
# create_one_agent <ETIKET> <DOSYA_ADI_DEG> <CN_DEG> <IP_DEG>
create_one_agent() {
    local __label="$1"
    local AGENT_FILENAME="$2"
    local AGENT_CN="$3"
    local AGENT_IP="$4"

    echo ""
    echo -e "${YELLOW}--- ${__label} için bilgiler ---${NC}"

    # Dosya adı zorunlu — interaktifse boş bırakılamaz.
    while [ -z "$AGENT_FILENAME" ]; do
        if [ ! -t 0 ]; then
            echo -e "${RED}HATA: Etkileşimsiz modda agent dosya adı belirtilmeli.${NC}"
            return 1
        fi
        read -r -p "$(echo -e ${RED}"Dosya adı boş olamaz. Lütfen bir isim girin: "${NC})" AGENT_FILENAME || AGENT_FILENAME=""
    done

    if [ -z "$AGENT_CN" ]; then
        AGENT_CN="$AGENT_FILENAME"
    fi

    # Mevcut agent cert'ini kazara ezmeye karşı koru.
    if [ -f "$CERT_DIR/${AGENT_FILENAME}.key" ] || [ -f "$CERT_DIR/${AGENT_FILENAME}.crt" ]; then
        if ! confirm "$(echo -e ${RED}"UYARI: '${AGENT_FILENAME}' için mevcut sertifika üzerine yazılacak. Emin misiniz? (e/h): "${NC})"; then
            echo "'${AGENT_FILENAME}' atlandı."
            return 0
        fi
    fi

    local AGENT_SUBJ
    AGENT_SUBJ="$(subject_dn "$CERT_ORG_AGENT" "$AGENT_CN")"

    echo "Agent '$AGENT_FILENAME' için özel anahtar (${AGENT_FILENAME}.key) oluşturuluyor..."
    openssl genpkey -algorithm RSA -out "$CERT_DIR/${AGENT_FILENAME}.key"

    echo "Agent '$AGENT_FILENAME' için CSR (${AGENT_FILENAME}.csr) oluşturuluyor..."
    openssl req -new -key "$CERT_DIR/${AGENT_FILENAME}.key" -out "$CERT_DIR/${AGENT_FILENAME}.csr" -subj "$AGENT_SUBJ"

    write_ext "$CERT_DIR/${AGENT_FILENAME}.ext" "$AGENT_CN" "$AGENT_IP"

    echo "Agent sertifikası (${AGENT_FILENAME}.crt) CA ile imzalanıyor..."
    openssl x509 -req -in "$CERT_DIR/${AGENT_FILENAME}.csr" -CA "$CERT_DIR/ca.crt" -CAkey "$CERT_DIR/ca.key" \
        -CAcreateserial -out "$CERT_DIR/${AGENT_FILENAME}.crt" -days "$DAYS_VALID" -sha256 -extfile "$CERT_DIR/${AGENT_FILENAME}.ext"

    echo -e "${GREEN}  -> ${AGENT_FILENAME}.key ve ${AGENT_FILENAME}.crt başarıyla oluşturuldu.${NC}"
}

# Ajan sertifikalarını oluşturan fonksiyon
create_agents() {
    echo ""
    echo -e "${BLUE}--- Guardian Agent Sertifikaları Oluşturma ---${NC}"

    # Agent sayısı: ortamdan (GUARDIAN_AGENT_COUNT) ya da interaktif.
    if [ -n "$GUARDIAN_AGENT_COUNT" ]; then
        AGENT_COUNT="$GUARDIAN_AGENT_COUNT"
    else
        while true; do
            if [ ! -t 0 ]; then
                AGENT_COUNT=0
                break
            fi
            read -r -p "Kaç adet Agent için sertifika oluşturulacak? " AGENT_COUNT || AGENT_COUNT=""
            if [[ $AGENT_COUNT =~ ^[0-9]+$ ]]; then
                break
            else
                echo -e "${RED}Lütfen geçerli bir sayı girin.${NC}"
            fi
        done
    fi

    if ! [[ $AGENT_COUNT =~ ^[0-9]+$ ]]; then
        echo -e "${RED}HATA: Geçersiz agent sayısı: '$AGENT_COUNT'.${NC}"
        return 1
    fi

    if [ "$AGENT_COUNT" -eq 0 ]; then
        echo "Ajan sertifikası oluşturulmayacak."
        return 0
    fi

    local i
    for (( i=1; i<=AGENT_COUNT; i++ )); do
        # Etkileşimsiz mod için alan başına ortam değişkeni desteği:
        #   GUARDIAN_AGENT<i>_FILENAME / _CN / _IP
        local fn_var="GUARDIAN_AGENT${i}_FILENAME"
        local cn_var="GUARDIAN_AGENT${i}_CN"
        local ip_var="GUARDIAN_AGENT${i}_IP"

        local a_filename="${!fn_var}"
        local a_cn="${!cn_var}"
        local a_ip="${!ip_var}"

        # İnteraktif ise (ve ortamdan gelmediyse) sor.
        if [ -z "$a_filename" ] && [ -t 0 ]; then
            read -r -p "Agent #$i için ayırt edici bir dosya adı girin (örn: agent-prod-1): " a_filename || a_filename=""
        fi
        if [ -z "$a_cn" ] && [ -t 0 ]; then
            read -r -p "Agent #$i için 'Common Name' girin (örn: prod-web-1.mydomain.com): " a_cn || a_cn=""
        fi
        if [ -z "$a_ip" ] && [ -t 0 ]; then
            read -r -p "Agent #$i için DIŞ IP adresini girin (örn: 10.2.60.185): " a_ip || a_ip=""
        fi

        create_one_agent "Agent #$i" "$a_filename" "$a_cn" "$a_ip"
    done
}

# Temizlik ve sonuç gösterme fonksiyonu
finalize() {
    echo ""
    echo -e "${BLUE}--- Temizlik ---${NC}"
    echo "Geçici .csr, .ext ve .srl dosyaları siliniyor..."
    rm -f "$CERT_DIR"/*.csr
    rm -f "$CERT_DIR"/*.ext
    rm -f "$CERT_DIR"/*.srl
    echo -e "${GREEN}  -> Temizlik tamamlandı.${NC}"

    echo ""
    echo -e "${GREEN}====================================================${NC}"
    echo -e "${GREEN}      ✅ İşlem başarıyla tamamlandı!      ${NC}"
    echo -e "${GREEN}====================================================${NC}"
    echo "'$CERT_DIR/' dizininin son durumu:"
    ls -l "$CERT_DIR"
    echo ""
}

usage() {
    cat <<'EOF'
Kullanım: ./generate-certs.sh [SEÇENEK] [MOD]

MOD (opsiyonel, menüyü atlar):
  1    Sıfırdan tam kurulum (mevcut certs dizinini siler)
  2    Mevcut CA ile yeni agent sertifikası ekle
  3    Çıkış

SEÇENEKLER:
  -y, --force   Yıkıcı işlemleri (dizin silme / cert üzerine yazma) onaysız yap.
                Etkileşimsiz (pipe/CI) kullanımda gereklidir.
  -h, --help    Bu yardımı göster.

ETKİLEŞİMSİZ KULLANIM (ortam değişkenleri):
  GUARDIAN_CA_CN, GUARDIAN_SERVER_CN, GUARDIAN_SERVER_IP
  GUARDIAN_AGENT_COUNT ve her agent için:
    GUARDIAN_AGENT<i>_FILENAME, GUARDIAN_AGENT<i>_CN, GUARDIAN_AGENT<i>_IP
  GUARDIAN_CERT_DIR, GUARDIAN_CERT_DAYS, GUARDIAN_CERT_ORG, ... (bkz. betik başı)
  GUARDIAN_CERT_FORCE=1  ->  --force ile eşdeğer.

Örnek (etkileşimsiz, tek server + bir agent):
  GUARDIAN_CA_CN="Guardian Root CA" \
  GUARDIAN_SERVER_CN=guardian.local GUARDIAN_SERVER_IP=10.2.60.185 \
  GUARDIAN_AGENT_COUNT=1 GUARDIAN_AGENT1_FILENAME=agent0 \
  GUARDIAN_AGENT1_CN=guardian.local GUARDIAN_AGENT1_IP=10.2.60.185 \
  ./generate-certs.sh --force 1
EOF
}

# --- ARGÜMAN AYRIŞTIRMA ---
MAIN_CHOICE=""
while [ $# -gt 0 ]; do
    case "$1" in
        -y|--force) FORCE=1; shift ;;
        -h|--help) usage; exit 0 ;;
        1|2|3) MAIN_CHOICE="$1"; shift ;;
        *) echo -e "${RED}Bilinmeyen argüman: $1${NC}"; usage; exit 1 ;;
    esac
done

# --- ANA BETİK AKIŞI ---

# openssl aracının kurulu olup olmadığını kontrol et
if ! command -v openssl &> /dev/null; then
    echo -e "${RED}HATA: 'openssl' komutu bulunamadı. Lütfen OpenSSL paketini kurun.${NC}"
    exit 1
fi

echo -e "${BLUE}====================================================${NC}"
echo -e "${BLUE}  Guardian Projesi için Sertifika Oluşturucu ${NC}"
echo -e "${BLUE}====================================================${NC}"

# Mod argümanla verilmediyse menüden al (interaktif) veya varsayılana düş.
if [ -z "$MAIN_CHOICE" ]; then
    echo "Lütfen yapmak istediğiniz işlemi seçin:"
    echo -e "  ${YELLOW}1)${NC} Sıfırdan tam kurulum yap (Mevcut 'certs' dizinini siler)"
    echo -e "  ${YELLOW}2)${NC} Mevcut CA'yı kullanarak YENİ Agent sertifikası ekle"
    echo -e "  ${YELLOW}3)${NC} Çıkış"
    if [ -t 0 ]; then
        read -r -p "Seçiminiz [1, 2, 3]: " MAIN_CHOICE || MAIN_CHOICE=""
    else
        echo -e "${RED}HATA: Etkileşimsiz mod. Modu argümanla verin (örn: './generate-certs.sh 1 --force').${NC}"
        exit 1
    fi
fi

case $MAIN_CHOICE in
    1)
        # --- Sıfırdan Kurulum ---
        echo ""
        if [ -d "$CERT_DIR" ]; then
            if ! confirm "$(echo -e ${RED}"UYARI: '$CERT_DIR' dizini silinecek. Emin misiniz? (e/h): "${NC})"; then
                echo "İşlem iptal edildi."
                exit 0
            fi
            rm -rf "$CERT_DIR"
        fi
        echo "'$CERT_DIR' dizini oluşturuluyor..."
        mkdir -p "$CERT_DIR"

        create_ca_and_server
        create_agents
        finalize
        ;;
    2)
        # --- Yeni Ajan Ekleme ---
        echo ""
        if [ ! -f "$CERT_DIR/ca.key" ]; then
            echo -e "${RED}HATA: Yeni bir ajan eklemek için '$CERT_DIR/ca.key' dosyası bulunamadı.${NC}"
            echo "Lütfen önce sıfırdan bir kurulum yapın (Seçenek 1)."
            exit 1
        fi
        echo -e "${YELLOW}Mevcut CA kullanılarak yeni ajan(lar) eklenecek...${NC}"

        create_agents
        finalize
        ;;
    3)
        # --- Çıkış ---
        echo "İşlem iptal edildi."
        exit 0
        ;;
    *)
        # --- Geçersiz Seçim ---
        echo -e "${RED}Geçersiz seçim. Betik sonlandırılıyor.${NC}"
        exit 1
        ;;
esac
