# 🛡 Guardian: Geçici ve Denetlenebilir SSH Erişim Yönetimi

**Guardian**, modern altyapılar için tasarlanmış, **geçici ve denetlenebilir SSH erişimi sağlayan** açık kaynaklı bir Ayrıcalıklı Erişim Yönetimi (Privileged Access Management - PAM) çözümüdür.

## 🎯 Guardian Hangi Sorunu Çözüyor?

Geleneksel sistemlerde, sunuculara SSH erişimi genellikle kalıcı `authorized_keys` dosyaları üzerinden sağlanır. Bu yaklaşım, aşağıdaki gibi ciddi güvenlik riskleri ve operasyonel zorluklar yaratır:

*   **Kalıcı Yetkiler:** Bir çalışanın SSH anahtarı bir sunucuya eklendiğinde, bu anahtar manuel olarak kaldırılana kadar erişim devam eder. Çalışan ekipten ayrıldığında veya görevi değiştiğinde, anahtarının tüm sunuculardan temizlenmesi karmaşık ve hataya açık bir süreçtir.
*   **Denetim Eksikliği:** Kimin, ne zaman, hangi sunucuya bağlandığını ve oturum sırasında hangi komutları çalıştırdığını merkezi olarak takip etmek zordur.
*   **"Paylaşılan Anahtar" Problemi:** Bir ekibin ortak kullandığı bir SSH anahtarının (örneğin, `dev-team.pem`) sızdırılması, tüm altyapıyı riske atar.

**Guardian**, bu sorunları çözmek için tasarlanmıştır. Statik SSH anahtarları yerine, **"Just-in-Time" (Tam Zamanında) erişim prensibini** benimser. Yöneticiler, belirli bir görev için, belirli bir kullanıcıya, belirli bir sunucuya ve **sadece belirli bir süre için** geçerli olan erişim kuralları tanımlar. Süresi dolan erişim hakları, hiçbir manuel müdahaleye gerek kalmadan sistemden otomatik olarak kaldırılır.

[](https://i.imgur.com/eTjWq9m.png)
_Guardian'ın modern web arayüzü, sisteminize 360 derece bir bakış sunar._

## ✨ Temel Özellikler

*   **Geçici Erişim Kuralları:** Sunuculara `15 dakika`, `2 saat` veya `1 gün` gibi belirli sürelerle sınırlı erişim yetkileri tanımlayın. Süresi dolan kurallar otomatik olarak devre dışı bırakılır.
*   **Merkezi Yönetim:** Tüm sunucuları, sistem kullanıcılarını, genel SSH anahtarlarını ve erişim kurallarını tek bir web arayüzünden veya CLI üzerinden yönetin.
*   **Oturum Kaydı ve Tekrar Oynatma:** Tüm SSH oturumları kaydedilir. Tamamlanmış oturumları **video gibi tekrar oynatarak** yapılan tüm işlemleri denetleyin.
*   **Canlı Oturum İzleme:** Aktif SSH oturumlarına anında bağlanarak **canlı olarak izleyin** ve şüpheli durumlarda oturumu zorla sonlandırın.
*   **Detaylı API Dokümantasyonu:** Tüm API endpoint'leri, `docs/swagger.yaml` dosyasında OpenAPI 3.0 standardında belgelenmiştir.
*   **Komut Satırı Aracı (CLI):** Güçlü ve interaktif CLI ile tüm sistemi otomasyon betiklerinize entegre edin veya komut satırından yönetin.
*   **Güvenlik Odaklı Mimari:** Tüm bileşenler arası iletişim, kendi Kök Sertifika Otoriteniz (Root CA) ile imzaladığınız TLS sertifikalarıyla uçtan uca şifrelenir.
*   **Modern ve Hızlı Arayüz:** Angular ve TailwindCSS ile oluşturulmuş, anlık veriler sunan reaktif bir web arayüzü.

## 🏛️ Mimari

Guardian, birbirinden bağımsız ama entegre çalışan dört ana bileşenden oluşur:

1.  **Guardian Server (Go):** Projenin beyni. API'yi sunar, veritabanını yönetir, zamanlanmış görevleri çalıştırır ve ajanlarla iletişim kurar.
2.  **Guardian UI (Angular):** Yöneticilerin sistemi yönettiği, oturumları izlediği ve raporları gördüğü web tabanlı arayüz.
3.  **Guardian Agent (Go):** Yönetilen hedef sunucularda çalışan hafif ajan. `authorized_keys` dosyasını dinamik olarak yönetir ve SSH oturumlarını proxy'leyerek kaydeder.
4.  **Guardian CLI (Go):** Yöneticiler için tasarlanmış, komut satırından sistemi yönetmeyi sağlayan araç.

 _(Buraya mimari şemasının bir görselini ekleyebilirsiniz.)_

## 🚀 Hızlı Başlangıç

Guardian'ı kendi ortamınızda kurmak için aşağıdaki adımları izleyin. Detaylı talimatlar için ilgili kurulum belgelerine göz atın.

### 1. Ön Gereksinimler

*   Docker ve Docker Compose
*   Go (v1.24+)
*   Node.js ve Angular CLI
*   Nginx (veya başka bir reverse proxy)
*   `openssl` komut satırı aracı

### 2. Sertifikaların Oluşturulması

Tüm bileşenler arası güvenli iletişim için kendi TLS sertifikalarınızı oluşturmanız gerekmektedir. Proje, bu süreci basitleştiren interaktif bir betik içerir.

```bash
# Proje ana dizininde çalıştırın
./generate-certs.sh
```

Bu betik size adım adım rehberlik edecektir. Detaylı bilgi için **[Sertifika Oluşturma Rehberi](./docs/generate-certs-usage.md)**'ne bakın.

> 🔐 **GÜVENLİK UYARISI:** Oluşturulan `ca.key` dosyası, tüm sistemin güvenliğinin anahtarıdır. Bu dosyayı **ASLA** sunucularınıza kopyalamayın ve güvenli, çevrimdışı bir ortamda saklayın.

### 3. Sunucu Kurulumu

Projenin merkezi bileşenlerini (Server, Veritabanı, UI) bir sunucuya kurun. İki yöntemden birini seçebilirsiniz:

➡️ **Manuel (systemd) kurulum için: [Guardian Sunucu Kurulum Rehberi](./docs/server-setup.md)**

➡️ **Docker Compose ile kurulum için: [Guardian Docker Kurulum Rehberi](./docs/docker-setup.md)**

### 4. Agent Kurulumu

Yönetmek istediğiniz her bir hedef sunucuya Guardian Agent'ı kurun.

➡️ **Detaylı talimatlar için: [Guardian Agent Kurulum Rehberi](./docs/agent-setup.md)**

## 💻 Kullanım

Guardian sistemini hem web arayüzü hem de komut satırı aracı (CLI) ile yönetebilirsiniz.

### Web Arayüzü

Kurulum tamamlandıktan sonra, Nginx'i yapılandırdığınız sunucu IP adresine veya alan adına gidin. Giriş ekranına **kullanıcı adı ve parolanızı** girerek sisteme erişebilirsiniz. İlk yönetici hesabı, sunucu ilk açıldığında `GUARDIAN_ADMIN_USERNAME`/`GUARDIAN_ADMIN_PASSWORD` ortam değişkenlerinden oluşturulur (parola boş bırakılırsa rastgele bir geçici parola üretilip sunucu log'una yazılır). Yeni yöneticiler ve roller (İzleyici / Operatör / Yönetici) panelin **Yöneticiler** ekranından yönetilir.

### Komut Satırı Aracı (CLI)

CLI, otomasyon ve hızlı işlemler için idealdir. Kullanmadan önce, CLI'ın çalışacağı makinede aşağıdaki ortam değişkenlerini ayarlamanız gerekir:

```bash
export GUARDIAN_SERVER_HOST="https://<sunucu-ip-adresiniz>"
export GUARDIAN_SERVER_PORT="5555" # Veya Nginx portunuz (443)
export GUARDIAN_ADMIN_USERNAME="<yonetici-kullanici-adi>"
export GUARDIAN_ADMIN_PASSWORD="<yonetici-parolasi>"
export TLS_CA_FILE="/path/to/your/ca.crt"
```

**Bazı Örnek Komutlar:**

```bash
# Tüm sunucuları listele
guardian-cli get servers

# İnteraktif modda yeni bir erişim kuralı oluştur (1 saat geçerli)
guardian-cli create rule

# Belirli bir sunucuya 30 dakikalık erişim kuralı oluştur
guardian-cli create rule --server-id 1 --user-id 2 --key-id 3 --duration 30m

# Aktif bir oturumu canlı izle (tarayıcıda açar)
guardian-cli watch session 42

# 5 numaralı kuralı sil
guardian-cli delete rule 5
```

## 📖 API Dokümantasyonu

Guardian Server, tüm özelliklerini bir RESTful API üzerinden sunar. API'nin tüm endpoint'leri, parametreleri ve beklenen yanıtları **OpenAPI 3.0** standardında belgelenmiştir.

➡️ **API referansı için: [Swagger API Dokümantasyonu](./docs/swagger.yaml)**

Bu dosyayı Swagger Editor veya benzeri bir araçla açarak API'yi interaktif olarak inceleyebilirsiniz.

## 🛠️ Derleme ve Geliştirme

Projeyi yerel makinenizde derlemek ve geliştirmek için:

*   **Backend (Server & Agent):** İlgili dizine gidin ve `go build .` komutunu çalıştırın.
*   **Frontend (UI):** `guardian/guardian-ui` dizinine gidin, `npm install` komutuyla bağımlılıkları yükleyin ve `ng serve` ile geliştirme sunucusunu başlatın.
*   **CLI:** `guardian/guardian-cli` dizinine gidin ve `go build .` komutunu çalıştırın.

## 🤝 Katkıda Bulunma

Katkılarınız bizim için değerlidir! Lütfen bir "issue" açarak veya "pull request" göndererek projeye katkıda bulunun.

## 📜 Lisans

Bu proje MIT Lisansı altında lisanslanmıştır. Detaylar için `LICENSE` dosyasına bakın.
