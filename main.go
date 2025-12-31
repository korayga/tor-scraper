package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"github.com/chromedp/chromedp"
	"golang.org/x/net/proxy"
	"gopkg.in/yaml.v3"
)

// Hedef dosyasını okuyan yapı
type HedefOkuyucu struct {
	dosyaAdi string
}

// Yeni bir hedef okuyucu oluşturur
func YeniHedefOkuyucu(dosyaAdi string) *HedefOkuyucu {
	return &HedefOkuyucu{dosyaAdi: dosyaAdi}
}

// Dosyadaki hedefleri okur ve temizler
func (ho *HedefOkuyucu) HedefleriOku() ([]string, error) {
	veri, err := os.ReadFile(ho.dosyaAdi)
	if err != nil {
		return nil, fmt.Errorf("dosya açılamadı: %v", err)
	}

	// YAML içeriğini çözümle
	var hedefler []string
	err = yaml.Unmarshal(veri, &hedefler)
	if err != nil {
		return nil, fmt.Errorf("YAML hatası: %v", err)
	}

	// Boşlukları temizle ve listeyi oluştur
	var temizHedefler []string
	for _, hedef := range hedefler {
		temizlenen := strings.TrimSpace(hedef)
		if temizlenen != "" {
			temizHedefler = append(temizHedefler, temizlenen)
		}
	}

	return temizHedefler, nil
}

// Tor Proxy bağlantısını yöneten yapı
type TorIstemcisi struct {
	httpIstemcisi *http.Client
	kayitci       *Kayitci
}

// Yeni bir Tor istemcisi oluşturur
func YeniTorIstemcisi(proxyAdresi string, kayitci *Kayitci) (*TorIstemcisi, error) {
	// SOCKS5 proxy bağlantısı
	baglayici, err := proxy.SOCKS5("tcp", proxyAdresi, nil, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("SOCKS5 hatası: %v", err)
	}

	// IP sızıntısını önleyen özel ağ ayarları
	tasima := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return baglayici.Dial(network, addr)
		},
		DisableKeepAlives:     false,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   60 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// HTTP istemcisi ayarları
	httpIstemcisi := &http.Client{
		Transport: tasima,
		Timeout:   120 * time.Second,
	}

	return &TorIstemcisi{
		httpIstemcisi: httpIstemcisi,
		kayitci:       kayitci,
	}, nil
}

// Tor bağlantısının çalışıp çalışmadığını kontrol eder
func (ti *TorIstemcisi) TorBaglantisiniDogrula() error {
	yanit, err := ti.httpIstemcisi.Get("https://check.torproject.org/api/ip")
	if err != nil {
		return fmt.Errorf("bağlantı doğrulanamadı: %v", err)
	}
	defer yanit.Body.Close()

	govde, err := io.ReadAll(yanit.Body)
	if err != nil {
		return fmt.Errorf("yanıt okunamadı: %v", err)
	}

	ti.kayitci.Bilgi(fmt.Sprintf("Tor IP Kontrolü: %s", string(govde)))

	if strings.Contains(string(govde), "\"IsTor\":true") {
		ti.kayitci.Basarili("Tor bağlantısı doğrulandı!")
		return nil
	}

	return fmt.Errorf("Tor bağlantısı başarısız!")
}

// Tarama işleminin sonucunu tutan yapı
type TaramaSonucu struct {
	URL       string
	DurumKodu int
	Basarili  bool
	Hata      string
	Zaman     time.Time
	Boyut     int
}

// Belirtilen URL'i tarar ve sonucu döndürür
func (ti *TorIstemcisi) URLTara(url string) *TaramaSonucu {
	sonuc := &TaramaSonucu{
		URL:      url,
		Zaman:    time.Now(),
		Basarili: false,
	}

	ti.kayitci.Bilgi(fmt.Sprintf("Taranıyor: %s", url))

	yanit, err := ti.httpIstemcisi.Get(url)
	if err != nil {
		sonuc.Hata = err.Error()
		ti.kayitci.Hata(fmt.Sprintf("Tarama: %s -> BAŞARISIZ (%s)", url, err.Error()))
		return sonuc
	}
	defer yanit.Body.Close()

	sonuc.DurumKodu = yanit.StatusCode

	// İçeriği oku
	govde, err := io.ReadAll(yanit.Body)
	if err != nil {
		sonuc.Hata = fmt.Sprintf("içerik okunamadı: %v", err)
		ti.kayitci.Hata(fmt.Sprintf("Tarama: %s -> OKUMA HATASI", url))
		return sonuc
	}

	sonuc.Boyut = len(govde)

	// Başarılı durum kodları (200-299)
	if yanit.StatusCode >= 200 && yanit.StatusCode < 300 {
		sonuc.Basarili = true
		ti.kayitci.Basarili(fmt.Sprintf("Tarama: %s -> BAŞARILI (Kod: %d, Boyut: %d byte)",
			url, yanit.StatusCode, len(govde)))

		// Veriyi kaydet
		if err := HTMLVerisiniKaydet(url, govde); err != nil {
			ti.kayitci.Hata(fmt.Sprintf("Kaydetme hatası: %v", err))
		}
	} else {
		sonuc.Hata = fmt.Sprintf("HTTP %d", yanit.StatusCode)
		ti.kayitci.Uyari(fmt.Sprintf("Tarama: %s -> HTTP %d", url, yanit.StatusCode))
	}

	return sonuc
}

// HTML verisini dosyaya kaydeder
func HTMLVerisiniKaydet(url string, veri []byte) error {
	// Çıktı klasörü oluştur
	ciktiKlasoru := "scraped_data"
	if err := os.MkdirAll(ciktiKlasoru, 0755); err != nil {
		return fmt.Errorf("klasör oluşturulamadı: %v", err)
	}

	// URL'den güvenli dosya adı oluştur
	dosyaAdi := strings.ReplaceAll(url, "http://", "")
	dosyaAdi = strings.ReplaceAll(dosyaAdi, "https://", "")
	dosyaAdi = strings.ReplaceAll(dosyaAdi, "/", "_")
	dosyaAdi = strings.ReplaceAll(dosyaAdi, ":", "_")

	// Zaman damgası ekle
	zamanDamgasi := time.Now().Format("20060102_150405")
	dosyaAdi = fmt.Sprintf("%s_%s.html", dosyaAdi, zamanDamgasi)

	dosyaYolu := filepath.Join(ciktiKlasoru, dosyaAdi)

	// Dosyaya yaz
	if err := os.WriteFile(dosyaYolu, veri, 0644); err != nil {
		return fmt.Errorf("dosya yazılamadı: %v", err)
	}

	return nil
}

// Hem HTML hem de ekran görüntüsü alan kazıyıcı yapı
type Kaziyici struct {
	proxyAdresi   string
	kontrolAdresi string
	kayitci       *Kayitci
}

// Yeni bir kazıyıcı oluşturur
func YeniKaziyici(proxyAdresi, kontrolAdresi string, kayitci *Kayitci) *Kaziyici {
	return &Kaziyici{
		proxyAdresi:   proxyAdresi,
		kontrolAdresi: kontrolAdresi,
		kayitci:       kayitci,
	}
}

// Kazıma işleminin sonucunu tutan yapı
type KazimaSonucu struct {
	HTML           string
	EkranGoruntusu []byte
}

// Tor kimliğini yeniler (IP değiştirir)
func (k *Kaziyici) TorKimliginiYenile() error {
	if k.kontrolAdresi == "" {
		return nil
	}

	baglanti, err := net.Dial("tcp", k.kontrolAdresi)
	if err != nil {
		return fmt.Errorf("kontrol portuna bağlanılamadı: %v", err)
	}
	defer baglanti.Close()

	// Kimlik doğrulama (Şifresiz varsayılıyor)
	fmt.Fprintf(baglanti, "AUTHENTICATE \"\"\r\n")

	// Yanıtı oku
	tampon := make([]byte, 512)
	n, err := baglanti.Read(tampon)
	if err != nil {
		return fmt.Errorf("yanıt okunamadı: %v", err)
	}

	if !strings.Contains(string(tampon[:n]), "250") {
		return fmt.Errorf("kimlik doğrulama başarısız")
	}

	// Yeni kimlik sinyali gönder
	fmt.Fprintf(baglanti, "SIGNAL NEWNYM\r\n")
	n, err = baglanti.Read(tampon)
	if err != nil {
		return fmt.Errorf("sinyal yanıtı okunamadı: %v", err)
	}

	if !strings.Contains(string(tampon[:n]), "250") {
		return fmt.Errorf("NEWNYM başarısız")
	}

	return nil
}

// Belirtilen URL'i kazır (HTML + Screenshot)
func (k *Kaziyici) Kazi(url string) (*KazimaSonucu, error) {
	// Chrome'u Tor proxy ile başlat
	secenekler := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ProxyServer(fmt.Sprintf("socks5://%s", k.proxyAdresi)),
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.WindowSize(1920, 1080),
	)

	allocCtx, iptal := chromedp.NewExecAllocator(context.Background(), secenekler...)
	defer iptal()

	// Yeniden deneme mekanizması (3 hak)
	maksimumDeneme := 3
	var sonHata error

	for i := 0; i < maksimumDeneme; i++ {
		if i > 0 {
			k.kayitci.Bilgi(fmt.Sprintf("Yeniden deneniyor (%d/%d): %s", i+1, maksimumDeneme, url))

			// IP Değiştirme Denemesi
			if k.kontrolAdresi != "" {
				k.kayitci.Bilgi("Yeni Tor kimliği isteniyor...")
				if err := k.TorKimliginiYenile(); err != nil {
					k.kayitci.Uyari(fmt.Sprintf("IP değiştirilemedi: %v", err))
				} else {
					k.kayitci.Basarili("Yeni Tor kimliği (IP) alındı.")
					time.Sleep(5 * time.Second) // Devrenin kurulması için bekle
				}
			} else {
				time.Sleep(5 * time.Second)
			}
		}

		ctx, iptalContext := chromedp.NewContext(allocCtx)
		// Zaman aşımı ayarla
		ctx, iptalContext = context.WithTimeout(ctx, 120*time.Second)

		var tampon []byte
		var htmlIcerik string

		// Sayfayı ziyaret et, HTML ve screenshot al
		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			chromedp.Sleep(10*time.Second), // Tor yavaş olabilir, süreyi artırdık
			chromedp.OuterHTML("html", &htmlIcerik),
			chromedp.FullScreenshot(&tampon, 90),
		)

		iptalContext() // Context'i temizle

		if err == nil {
			k.kayitci.Basarili(fmt.Sprintf("Kazıma başarılı: %s (%d KB Screenshot, %d bytes HTML)", url, len(tampon)/1024, len(htmlIcerik)))
			return &KazimaSonucu{
				HTML:           htmlIcerik,
				EkranGoruntusu: tampon,
			}, nil
		}

		sonHata = err
		k.kayitci.Uyari(fmt.Sprintf("Kazıma hatası (Deneme %d): %v", i+1, err))
	}

	return nil, fmt.Errorf("3 deneme sonrası başarısız: %v", sonHata)
}

// Ekran görüntüsünü dosyaya kaydeder
func EkranGoruntusunuKaydet(url string, veri []byte) error {
	// Klasör oluştur
	ciktiKlasoru := "screenshots"
	if err := os.MkdirAll(ciktiKlasoru, 0755); err != nil {
		return fmt.Errorf("klasör oluşturulamadı: %v", err)
	}

	// Dosya adını temizle
	dosyaAdi := strings.ReplaceAll(url, "http://", "")
	dosyaAdi = strings.ReplaceAll(dosyaAdi, "https://", "")
	dosyaAdi = strings.ReplaceAll(dosyaAdi, "/", "_")
	dosyaAdi = strings.ReplaceAll(dosyaAdi, ":", "_")

	zamanDamgasi := time.Now().Format("20060102_150405")
	dosyaAdi = fmt.Sprintf("%s_%s.png", dosyaAdi, zamanDamgasi)

	dosyaYolu := filepath.Join(ciktiKlasoru, dosyaAdi)

	// Dosyaya yaz
	if err := os.WriteFile(dosyaYolu, veri, 0644); err != nil {
		return fmt.Errorf("dosya yazılamadı: %v", err)
	}

	return nil
}

// Loglama işlemlerini yöneten yapı
type Kayitci struct {
	kayitDosyasi *os.File
}

// Yeni bir kayıtçı oluşturur
func YeniKayitci(dosyaAdi string) (*Kayitci, error) {
	dosya, err := os.OpenFile(dosyaAdi, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return &Kayitci{kayitDosyasi: dosya}, nil
}

// Temel loglama fonksiyonu
func (k *Kayitci) kayit(seviye, mesaj string) {
	zamanDamgasi := time.Now().Format("2006-01-02 15:04:05")
	logMesaji := fmt.Sprintf("[%s] [%s] %s\n", zamanDamgasi, seviye, mesaj)

	// Hem ekrana hem dosyaya yaz
	fmt.Print(logMesaji)
	if _, err := k.kayitDosyasi.WriteString(logMesaji); err == nil {
		k.kayitDosyasi.Sync() // Diske yazmayı zorla
	}
}

// Bilgi seviyesinde log yazar
func (k *Kayitci) Bilgi(mesaj string) {
	k.kayit("BILGI", mesaj)
}

// Başarılı işlem logu yazar
func (k *Kayitci) Basarili(mesaj string) {
	k.kayit("BASARILI", mesaj)
}

// Uyarı logu yazar
func (k *Kayitci) Uyari(mesaj string) {
	k.kayit("UYARI", mesaj)
}

// Hata logu yazar
func (k *Kayitci) Hata(mesaj string) {
	k.kayit("HATA", mesaj)
}

// Kayıt dosyasını kapatır
func (k *Kayitci) Kapat() {
	k.kayitDosyasi.Close()
}

// Ana Program
func main() {
	fmt.Println("		TOR KAZIYICI (Tek Seferlik Mod)")
	fmt.Println()

	// Kayıtçıyı başlat
	kayitci, err := YeniKayitci("tarama_raporu.log")
	if err != nil {
		log.Fatalf("Kayıtçı oluşturulamadı: %v", err)
	}
	defer kayitci.Kapat()

	kayitci.Bilgi("Program başlatıldı")

	// Ayarlar
	torProxy := "127.0.0.1:9150"   // Varsayılan Tor Browser portu
	torKontrol := "127.0.0.1:9151" // Varsayılan Tor Kontrol portu
	hedefDosyasi := "targets.yaml"

	if len(os.Args) > 1 {
		hedefDosyasi = os.Args[1]
	}
	if len(os.Args) > 2 {
		torProxy = os.Args[2]
	}
	if len(os.Args) > 3 {
		torKontrol = os.Args[3]
	}

	kayitci.Bilgi(fmt.Sprintf("Hedef dosyası: %s", hedefDosyasi))
	kayitci.Bilgi(fmt.Sprintf("Tor Proxy: %s", torProxy))
	kayitci.Bilgi(fmt.Sprintf("Tor Kontrol: %s", torKontrol))

	// Hedefleri oku
	okuyucu := YeniHedefOkuyucu(hedefDosyasi)
	hedefler, err := okuyucu.HedefleriOku()
	if err != nil {
		kayitci.Hata(fmt.Sprintf("Hedefler okunamadı: %v", err))
		log.Fatalf("Hedefler okunamadı: %v", err)
	}

	kayitci.Bilgi(fmt.Sprintf("Toplam %d hedef bulundu", len(hedefler)))

	// Tor İstemcisini oluştur (Sadece IP kontrolü için)
	torIstemcisi, err := YeniTorIstemcisi(torProxy, kayitci)
	if err != nil {
		kayitci.Hata(fmt.Sprintf("Tor istemcisi oluşturulamadı: %v", err))
		kayitci.Bilgi("Not: 9050 portu çalışmıyorsa 9150 portunu deneyin (Tor Browser)")
		// Kritik hata vermiyoruz, belki sadece kazıyıcı çalışır
	} else {
		// Tor bağlantısını doğrula
		kayitci.Bilgi("Tor bağlantısı kontrol ediliyor...")
		if err := torIstemcisi.TorBaglantisiniDogrula(); err != nil {
			kayitci.Uyari(fmt.Sprintf("Tor doğrulama hatası: %v", err))
			kayitci.Uyari("Devam ediliyor... (Tor servisi çalıştığından emin olun)")
		}
	}

	// Kazıyıcıyı oluştur (Tek seferde HTML + Screenshot)
	kaziyici := YeniKaziyici(torProxy, torKontrol, kayitci)

	fmt.Println()
	kayitci.Bilgi("Tarama başlatılıyor...")
	fmt.Println()

	// Her hedefi tara ve kaydet
	basariliSayisi := 0
	basarisizSayisi := 0

	for i, hedef := range hedefler {
		kayitci.Bilgi(fmt.Sprintf("[%d/%d] İşleniyor: %s", i+1, len(hedefler), hedef))

		// Tek Seferlik Kazıma
		sonuc, err := kaziyici.Kazi(hedef)

		if err != nil {
			basarisizSayisi++
			kayitci.Hata(fmt.Sprintf("BAŞARISIZ: %s -> %v", hedef, err))
		} else {
			basariliSayisi++

			// HTML Kaydet
			if err := HTMLVerisiniKaydet(hedef, []byte(sonuc.HTML)); err != nil {
				kayitci.Hata(fmt.Sprintf("HTML kaydetme hatası: %v", err))
			}

			// Screenshot Kaydet
			if err := EkranGoruntusunuKaydet(hedef, sonuc.EkranGoruntusu); err != nil {
				kayitci.Hata(fmt.Sprintf("Screenshot kaydetme hatası: %v", err))
			}
		}

		// Her istek arasında kısa bekleme (hız sınırlaması)
		if i < len(hedefler)-1 {
			time.Sleep(2 * time.Second)
		}
		fmt.Println()
	}

	// Özet rapor
	fmt.Println()
	fmt.Println("           TARAMA TAMAMLANDI")
	fmt.Println("===========================================")

	kayitci.Bilgi(fmt.Sprintf("Tarama tamamlandı - Başarılı: %d, Başarısız: %d", basariliSayisi, basarisizSayisi))
	fmt.Printf("Toplam URL: %d\n", len(hedefler))
	fmt.Printf("Başarılı: %d\n", basariliSayisi)
	fmt.Printf("Başarısız: %d\n", basarisizSayisi)
	fmt.Printf("\nLoglar: tarama_raporu.log\n")
	fmt.Printf("Veriler: scraped_data/ klasörü\n")
	fmt.Printf("Ekran Görüntüleri: screenshots/ klasörü\n")
	fmt.Println("===========================================")
}
