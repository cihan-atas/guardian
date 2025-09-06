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
CERT_DIR="certs"
DAYS_VALID=3650 # Sertifikaların geçerlilik süresi (10 yıl)

# --- FONKSİYONLAR ---

# CA ve Sunucu sertifikalarını oluşturan fonksiyon
create_ca_and_server() {
    echo ""
    echo -e "${BLUE}--- Adım 1: Kök Sertifika Otoritesi (CA) Oluşturma ---${NC}"
    read -p "CA için bir 'Common Name' girin (örn: Guardian Root CA): " CA_CN
    CA_SUBJ="/C=TR/ST=Istanbul/L=Istanbul/O=Guardian Self-Signed/CN=${CA_CN:-Guardian Root CA}"

    echo "CA özel anahtarı (ca.key) oluşturuluyor..."
    openssl genpkey -algorithm RSA -out "$CERT_DIR/ca.key"

    echo "Kendinden imzalı CA sertifikası (ca.crt) oluşturuluyor..."
    openssl req -x509 -new -nodes -key "$CERT_DIR/ca.key" -sha256 -days $DAYS_VALID -out "$CERT_DIR/ca.crt" -subj "$CA_SUBJ"
    echo -e "${GREEN}  -> ca.key ve ca.crt başarıyla oluşturuldu.${NC}"

    echo ""
    echo -e "${BLUE}--- Adım 2: Guardian Server Sertifikası Oluşturma ---${NC}"
    read -p "Server için bir 'Common Name' girin (genellikle DNS adı, varsayılan: localhost): " SERVER_CN
    SERVER_CN=${SERVER_CN:-localhost}
    # DÜZELTME: Sunucunun IP adresini iste
    read -p "Server için DIŞ IP adresini girin (örn: 10.2.60.185): " SERVER_IP
    SERVER_SUBJ="/C=TR/ST=Istanbul/L=Istanbul/O=Guardian Self-Signed/CN=${SERVER_CN}"

    echo "Server özel anahtarı (server.key) oluşturuluyor..."
    openssl genpkey -algorithm RSA -out "$CERT_DIR/server.key"

    echo "Server için Sertifika İmzalama Talebi (server.csr) oluşturuluyor..."
    openssl req -new -key "$CERT_DIR/server.key" -out "$CERT_DIR/server.csr" -subj "$SERVER_SUBJ"
    
    # DÜZELTME: server.ext dosyasına dış IP adresini ekle
    cat > "$CERT_DIR/server.ext" << EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, keyEncipherment
subjectAltName = @alt_names
[alt_names]
DNS.1 = ${SERVER_CN}
IP.1 = 127.0.0.1
IP.2 = ${SERVER_IP}
EOF

    echo "Server sertifikası (server.crt) CA ile imzalanıyor..."
    openssl x509 -req -in "$CERT_DIR/server.csr" -CA "$CERT_DIR/ca.crt" -CAkey "$CERT_DIR/ca.key" \
    -CAcreateserial -out "$CERT_DIR/server.crt" -days $DAYS_VALID -sha256 -extfile "$CERT_DIR/server.ext"
    echo -e "${GREEN}  -> server.key ve server.crt başarıyla oluşturuldu.${NC}"
}

# Ajan sertifikalarını oluşturan fonksiyon
create_agents() {
    echo ""
    echo -e "${BLUE}--- Guardian Agent Sertifikaları Oluşturma ---${NC}"
    while true; do
        read -p "Kaç adet Agent için sertifika oluşturulacak? " AGENT_COUNT
        if [[ $AGENT_COUNT =~ ^[0-9]+$ ]]; then
            break
        else
            echo -e "${RED}Lütfen geçerli bir sayı girin.${NC}"
        fi
    done

    if [ "$AGENT_COUNT" -eq 0 ]; then
        echo "Ajan sertifikası oluşturulmayacak."
        return
    fi

    for (( i=1; i<=$AGENT_COUNT; i++ )); do
        echo ""
        echo -e "${YELLOW}--- Agent #$i için bilgiler ---${NC}"
        read -p "Agent #$i için ayırt edici bir dosya adı girin (örn: agent-prod-1): " AGENT_FILENAME
        while [ -z "$AGENT_FILENAME" ]; do
            read -p "$(echo -e ${RED}"Dosya adı boş olamaz. Lütfen bir isim girin: "${NC})" AGENT_FILENAME
        done
        read -p "Agent #$i için 'Common Name' girin (örn: prod-web-1.mydomain.com): " AGENT_CN
        AGENT_CN=${AGENT_CN:-$AGENT_FILENAME}
        # DÜZELTME: Agent'ın IP adresini iste
        read -p "Agent #$i için DIŞ IP adresini girin (örn: 10.2.60.185): " AGENT_IP
        AGENT_SUBJ="/C=TR/ST=Istanbul/L=Istanbul/O=Guardian Self-Signed Agent/CN=${AGENT_CN}"

        echo "Agent '$AGENT_FILENAME' için özel anahtar (${AGENT_FILENAME}.key) oluşturuluyor..."
        openssl genpkey -algorithm RSA -out "$CERT_DIR/${AGENT_FILENAME}.key"

        echo "Agent '$AGENT_FILENAME' için CSR (${AGENT_FILENAME}.csr) oluşturuluyor..."
        openssl req -new -key "$CERT_DIR/${AGENT_FILENAME}.key" -out "$CERT_DIR/${AGENT_FILENAME}.csr" -subj "$AGENT_SUBJ"
        
        # DÜZELTME: agent.ext dosyasına dış IP adresini ekle
        cat > "$CERT_DIR/${AGENT_FILENAME}.ext" << EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, keyEncipherment
subjectAltName = @alt_names
[alt_names]
DNS.1 = ${AGENT_CN}
IP.1 = 127.0.0.1
IP.2 = ${AGENT_IP}
EOF

        echo "Agent sertifikası (${AGENT_FILENAME}.crt) CA ile imzalanıyor..."
        openssl x509 -req -in "$CERT_DIR/${AGENT_FILENAME}.csr" -CA "$CERT_DIR/ca.crt" -CAkey "$CERT_DIR/ca.key" \
        -CAcreateserial -out "$CERT_DIR/${AGENT_FILENAME}.crt" -days $DAYS_VALID -sha256 -extfile "$CERT_DIR/${AGENT_FILENAME}.ext"
        
        echo -e "${GREEN}  -> ${AGENT_FILENAME}.key ve ${AGENT_FILENAME}.crt başarıyla oluşturuldu.${NC}"
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


# --- ANA BETİK AKIŞI ---

# openssl aracının kurulu olup olmadığını kontrol et
if ! command -v openssl &> /dev/null; then
    echo -e "${RED}HATA: 'openssl' komutu bulunamadı. Lütfen OpenSSL paketini kurun.${NC}"
    exit 1
fi

echo -e "${BLUE}====================================================${NC}"
echo -e "${BLUE}  Guardian Projesi için İnteraktif Sertifika Oluşturucu ${NC}"
echo -e "${BLUE}====================================================${NC}"

# Ana Menü
echo "Lütfen yapmak istediğiniz işlemi seçin:"
echo -e "  ${YELLOW}1)${NC} Sıfırdan tam kurulum yap (Mevcut 'certs' dizinini siler)"
echo -e "  ${YELLOW}2)${NC} Mevcut CA'yı kullanarak YENİ Agent sertifikası ekle"
echo -e "  ${YELLOW}3)${NC} Çıkış"
read -p "Seçiminiz [1, 2, 3]: " MAIN_CHOICE

case $MAIN_CHOICE in
    1)
        # --- Sıfırdan Kurulum ---
        echo ""
        if [ -d "$CERT_DIR" ]; then
            read -p "$(echo -e ${RED}"UYARI: '$CERT_DIR' dizini silinecek. Emin misiniz? (e/h): "${NC})" -n 1 -r
            echo
            if [[ ! $REPLY =~ ^[Ee]$ ]]; then
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
