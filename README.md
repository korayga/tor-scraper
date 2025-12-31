# Tor Web Scraper

> Tor ağı üzerinden çalışan, CTI ve OSINT araştırmaları için web scraping aracı.

---

## 🌟 Özellikler

### 🧅 Tor Ağı Entegrasyonu
- Otomatik SOCKS5 Proxy: Tor Browser (9150) veya Tor Service (9050) ile çalışır
- IP Rotasyonu: Her tarama sonrası otomatik Tor kimliği değiştirme
- Tor Doğrulama: `check.torproject.org` ile bağlantı kontrolü
- Kontrol Portu Desteği: 9151 (Tor Browser) veya 9051 (Tor Service)

### 📸 Full-Page Screenshot
- Sayfanın tamamını %90 kalitede PNG formatında kaydeder
- Headless Chrome ile render edilen tam sayfa görüntüsü
- 1920x1080 viewport çözünürlüğü
- Lazy-loaded elementleri destekler

### ⚡ Dinamik İçerik Desteği
- JavaScript ile yüklenen içerikleri yakalar (SPA, React, Vue, Angular)
- 10 saniye sayfa yükleme bekleme süresi
- Chromedp tabanlı tam tarayıcı emülasyonu
- Ajax/Fetch isteklerini bekler

### 🔄 Akıllı Yeniden Deneme Mekanizması
- Her URL için 3 otomatik deneme hakkı
- Başarısız denemelerde yeni Tor kimliği (IP değiştirme)
- 5 saniye bekleme + Tor devre kurulumu
- Hata durumunda detaylı loglama

### 💾 Çoklu Kayıt Formatı
- HTML Kaydetme: Tam sayfa kaynak kodu (scraped_data/)
- Screenshot Kaydetme: PNG ekran görüntüleri (screenshots/)
- Zaman Damgalı Dosyalar: `20060102_150405` formatında
- URL Temizleme: Güvenli dosya adları

### 📋 YAML Hedef Yönetimi
- `targets.yaml` dosyasından toplu URL okuma
- Boşluk ve satır temizleme
- `.onion` ve normal URL desteği
- Dosyaya Kayıt: `tarama_raporu.log` otomatik oluşturulur
- Özet Rapor: Tarama sonunda istatistikler

---


### Kurulum Adımları

```bash
# 1. Projeyi klonlayın
git clone https://github.com/korayga/tor-scraper.git
cd tor-scraper

# 2. Bağımlılıkları yükleyin
go mod download

# 3. Tor Browser'ı başlatın (9150/9151 portları)
# Veya Tor servisini çalıştırın

## 📦 Çıktı Dosyaları

Program her çalıştırmada **3 klasör** oluşturur:

### 1. scraped_data/ - HTML İçerikleri
```text
example_onion_20251231_143012.html
another-site_onion_20251231_143045.html
- İçerik: JavaScript render sonrası tam HTML
- Format: UTF-8 encoded
- Boyut: ~50KB - 5MB

### 2. screenshots/ - Ekran Görüntüleri
```text
example_onion_20251231_143012.png
another-site_onion_20251231_143045.png
```
- Çözünürlük: 1920x1080 viewport
- Özellik: Full-page (scroll dahil)
- Boyut: ~200KB - 3MB
[2025-12-31 14:30:12] [BILGI] Program başlatıldı
[2025-12-31 14:30:15] [BASARILI] Tor bağlantısı doğrulandı!
[2025-12-31 14:30:20] [BASARILI] Kazıma başarılı: http://example.onion
[2025-12-31 14:30:25] [HATA] BAŞARISIZ: http://broken-site.onion
```

---

## 🔧 Teknik Detaylar

### Kullanılan Teknolojiler

| Paket | Sürüm | Açıklama |
|-------|-------|----------|
| chromedp/chromedp | v0.14.2 | Headless Chrome otomasyonu |
| golang.org/x/net/proxy | v0.48.0 | SOCKS5 proxy desteği |
| gopkg.in/yaml.v3 | v3.0.1 | YAML dosya okuma |
| context | stdlib | Timeout ve iptal yönetimi |

### Chrome Bayrakları (Flags)

```go
chromedp.ProxyServer("socks5://127.0.0.1:9150")  // Tor proxy
chromedp.Flag("headless", true)                   // Gizli mod
chromedp.Flag("disable-gpu", true)                // GPU kapalı
chromedp.Flag("no-sandbox", true)                 // Sandbox bypass
chromedp.Flag("disable-dev-shm-usage", true)      // Düşük RAM mod
chromedp.WindowSize(1920, 1080)                   // Full HD
```


## 🛠️ Sorun Giderme

### ❌ "SOCKS5 hatası: connection refused"
Sebep: Tor servisi çalışmıyor
Çözüm:
```bash
# Tor Browser'ı başlatın (9150 portu)
# VEYA
# Tor servisini başlatın
sudo systemctl start tor    # Linux
brew services start tor     # macOS
```

### ❌ "Tor bağlantısı başarısız!"
Sebep: Tor proxy çalışıyor ama bağlantı kurulamıyor
Çözüm:
```bash
# Port kontrolü
netstat -an | grep 9150
# Tor Browser'ı yeniden başlatın
# Veya torrc ayarlarını kontrol edin
```

### ❌ "3 deneme sonrası başarısız"
Sebep: .onion sitesi erişilebilir değil veya çok yavaş
Çözümler:
1. Timeout süresini artırın (main.go):
```go
ctx, iptalContext = context.WithTimeout(ctx, 180*time.Second) // 120 → 180
chromedp.Sleep(15*time.Second) // 10 → 15
```
2. Manuel test edin:
```bash
# Tor Browser'da siteyi açın
# Eğer tarayıcıda da yavaşsa, site problemi var
```
3. Tor kimliğini daha sık değiştirin:
```go
✅ SOCKS5 Proxy yönetimi  
```

### ❌ "Permission denied" hatası
Sebep: Klasör yazma izni yok
Çözüm:
```bash
# Linux/macOS
chmod 755 scraped_data screenshots
sudo chown $USER:$USER .
# Windows (PowerShell Admin)
✅ HTTP client özelleştirme  
```

### ❌ "context deadline exceeded"
Sebep: Sayfa 120 saniyede yüklenemedi
Çözüm:
```go
ctx, iptalContext = context.WithTimeout(ctx, 300*time.Second) // 5 dakika
```

### ⚠️ Captcha/Bot Tespiti
Sebep: Site Tor trafiğini engelliyor
Çözümler:
1. User-Agent ekleyin:
```go
chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
```
2. Headless modunu kapatın (test için):
```go
chromedp.Flag("headless", false) // Tarayıcıyı göster
```
3. Bekleme süresini artırın:
```go
chromedp.Sleep(20*time.Second) // Captcha için manuel müdahale
```

---



### 🛡️ Güvenli Kullanım İpuçları
1. VPN Kullanın: Tor + VPN = Çift koruma
2. Rate Limiting Uygulayın: Her istek arasında bekleyin
3. Test Ortamı Kullanın: İlk testleri kendi sitenizde yapın
4. Log Güvenliği: Tarama sonrası logları temizleyin

---

## 📚 Kaynaklar

- [Tor Project](https://www.torproject.org/)
- [Go Documentation](https://golang.org/doc/)
- [golang.org/x/net/proxy](https://pkg.go.dev/golang.org/x/net/proxy)
- [SOCKS5 Protocol](https://datatracker.ietf.org/doc/html/rfc1928)

---

## 📞 İletişim

- **GitHub Issues:** [Sorun bildir](https://github.com/korayga/tor-scraper/issues)
- **Linkedin:** [korayga](https://www.linkedin.com/in/koray-garip/)

---
