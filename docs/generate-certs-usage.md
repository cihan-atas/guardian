## 🔐 TLS Sertifikalarını Oluşturma ve Dağıtma

Guardian'ın tüm bileşenleri (Server, Agent, CLI) arasındaki iletişim, güvenliği sağlamak amacıyla TLS ile şifrelenir. Bu, "ortadaki adam" (man-in-the-middle) saldırılarını önler ve verinin gizliliğini garanti eder. Bu güvenli yapıyı kurmak için, kendi Kök Sertifika Otoritemizi (Root CA) oluşturmalı ve bu otorite ile tüm bileşenler için sertifikalar imzalamalıyız.

Proje, bu süreci basitleştirmek için interaktif bir `generate-certs.sh` betiği içerir.

### Ön Gereksinimler

*   `openssl` komut satırı aracının sisteminizde yüklü olması.

### `generate-certs.sh` Betiğinin Kullanımı

Bu betik, tüm sertifika oluşturma ve imzalama adımlarını sizin için yönetir.

1.  **Betiği Çalıştırın:**
    Projenin ana dizininde aşağıdaki komutu çalıştırın:
    ```bash
    ./generate-certs.sh
    ```

2.  **İnteraktif Menü:**
    Karşınıza interaktif bir menü çıkacaktır:
    ```
    ====================================================
      Guardian Projesi için İnteraktif Sertifika Oluşturucu 
    ====================================================
    Lütfen yapmak istediğiniz işlemi seçin:
      1) Sıfırdan tam kurulum yap (Mevcut 'certs' dizinini siler)
      2) Mevcut CA'yı kullanarak YENİ Agent sertifikası ekle
      3) Çıkış
    Seçiminiz [1, 2, 3]:
    ```
    *   **Seçenek 1 (Sıfırdan tam kurulum):** Projeyi ilk defa kuruyorsanız bu seçeneği kullanmalısınız. Bu seçenek, mevcut `certs` dizinini siler ve sıfırdan bir Kök CA, bir Sunucu Sertifikası ve belirttiğiniz sayıda Agent Sertifikası oluşturur.
    *   **Seçenek 2 (Yeni Agent Ekle):** Mevcut bir Guardian kurulumunuz varsa ve sisteme yeni bir agent sunucusu eklemek istiyorsanız bu seçeneği kullanın. Bu, mevcut CA'nızı kullanarak sadece yeni bir agent için sertifika ve anahtar oluşturur.

### Betiğin Oluşturduğu Dosyalar ve Anlamları

"Sıfırdan tam kurulum" (Seçenek 1) tamamlandığında, projenin ana dizinindeki `certs/` klasörü içinde aşağıdaki gibi dosyalar oluşur:

| Dosya Adı | Açıklama | Nereye Gidecek? |
| :--- | :--- | :--- |
| **`ca.key`** | 🔐 **KÖK ÖZEL ANAHTARI.** Tüm güvenliğin temelidir. **ASLA sunuculara kopyalamayın!** | **Güvenli, çevrimdışı bir yerde saklayın.** |
| **`ca.crt`** | 📜 **Kök Sertifika Otoritesi.** Tüm bileşenlerin birbirine güvenmesini sağlar. | **Tüm** Sunucu ve Agent'lara. |
| **`server.key`** | 🔑 **Sunucu Özel Anahtarı.** Sunucunun kimliğini doğrular. Sadece sunucuda kalmalıdır. | Guardian Sunucusuna. |
| **`server.crt`** | 📄 **Sunucu Sertifikası.** `ca.crt` ile imzalanmıştır. | Guardian Sunucusuna. |
| **`agent0.key`** | 🔑 **Agent Özel Anahtarı.** Agent'ın kimliğini doğrular. Sadece ilgili agent'ta kalmalıdır. | 1. Agent Sunucusuna (server üstündeki agent için). |
| **`agent0.crt`** | 📄 **Agent Sertifikası.** `ca.crt` ile imzalanmıştır. | 1. Agent Sunucusuna (server üstündeki agent için). |
| **`agent1.key`** | 🔑 **Agent Özel Anahtarı.** | 2. Agent Sunucusuna. |
| **`agent1.crt`** | 📄 **Agent Sertifikası.** | 2. Agent Sunucusuna. |
| ... | ... | ... |

### Sertifikaların Sunuculara Dağıtımı

Sertifikaları oluşturduktan sonra, ilgili dosyaları doğru sunuculara `scp` veya benzeri bir yöntemle kopyalamanız gerekir.

#### 1. Guardian Sunucusuna Kopyalanacak Dosyalar

Aşağıdaki dosyaları geliştirme makinenizdeki `certs/` dizininden, Guardian Sunucusundaki `/etc/guardian/certs/` dizinine kopyalayın:

*   `ca.crt`
*   `server.crt`
*   `server.key`
*   Sunucunun kendi agent'ı için oluşturulan sertifika ve anahtar (örn: `agent0.crt` ve `agent0.key`)

Sunucuya kopyaladıktan sonra, agent dosyalarının adlarını standart hale getirin:
```bash
# Guardian Sunucusunda çalıştırın
cd /etc/guardian/certs/
sudo mv agent0.crt agent.crt
sudo mv agent0.key agent.key
```

#### 2. Hedef Agent Sunucusuna Kopyalanacak Dosyalar

Her bir agent sunucusu için, o sunucuya özel olarak oluşturulmuş sertifikaları kopyalayın. Örneğin, `agent1` olarak adlandırdığınız sunucu için:

*   `ca.crt`
*   `agent1.crt`
*   `agent1.key`

Bu dosyaları hedef agent sunucusundaki `/etc/guardian/certs/` dizinine kopyalayın ve yine adlarını standart hale getirin:
```bash
# Hedef Agent Sunucusunda çalıştırın
cd /etc/guardian/certs/
sudo mv agent1.crt agent.crt
sudo mv agent1.key agent.key
```

Tüm sertifikalar doğru sunuculara dağıtıldıktan ve dosya izinleri ayarlandıktan sonra, kurulum aşamalarına geçebilirsiniz
