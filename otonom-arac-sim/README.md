# 🚗 Otonom Araç Simülasyonu

KTÜ Optimizasyon Dersi Ödevi - Tek şeritli yolda otonom araç hareketi simülasyonu

## 📋 Proje Açıklaması

Bu proje, tek şeritli bir yol üzerinde hareket eden otonom araçların çarpışmadan ve kilitlenmeden (deadlock) güvenli şekilde hareketinin simülasyonunu gerçekleştirir.

### Sistem Özellikleri

- **Yol Uzunluğu**: 240 metre
- **Sensörler**: Her 40m'de bir (±20m algılama mesafesi)
- **Cepler**: Araçların bekleyebileceği alanlar (80m, 200m)
- **Ara Depolar**: Yük alınan noktalar (70m, 120m, 240m)
- **Minimum Mesafe**: Araçlar arası 20m
- **Maksimum Hız**: 20 km/h

### Güvenlik Kuralları

1. Araçlar arası minimum 20m mesafe
2. Karşı yönden gelen araçlar için cep/depo kullanımı
3. Ceplerde max 1 araç, depolarda max 3 araç

## 🛠️ Teknolojiler

### Backend
- **Go 1.24.2**
- WebSocket (gorilla/websocket)
- REST API

### Frontend
- **React 19** + **TypeScript**
- **Vite** (build tool)
- WebSocket (gerçek zamanlı güncelleme)

## 🚀 Kurulum ve Çalıştırma

### Backend

```bash
cd backend
go run main.go
```

Backend `http://localhost:8080` adresinde çalışacak.

### Frontend

```bash
cd frontend
npm install
npm run dev
```

Frontend `http://localhost:5173` adresinde çalışacak (port meşgulse Vite otomatik olarak 5174, 5175 gibi bir sonraki portu seçer).

## 📖 Kullanım

1. Backend'i başlatın
2. Frontend'i başlatın
3. Tarayıcıda `http://localhost:5173` adresine gidin (Vite farklı port seçtiyse terminalde gösterilen adresi kullanın)
4. **"+ Araç Ekle"** butonları ile araç ekleyin (her biri farklı depoya gider)
5. **"▶ Başlat"** butonu ile simülasyonu başlatın
6. Araçların hareketini izleyin

### Kontroller

- **▶ Başlat**: Simülasyonu başlatır
- **⏸ Durdur**: Simülasyonu duraklatır
- **🔄 Sıfırla**: Tüm araçları temizler ve simülasyonu sıfırlar
- **+ Araç Ekle → Xm**: Belirtilen depoya gidecek yeni araç ekler

### Renk Kodları

- 🔴 **Kırmızı**: Boş araç (depoya gidiyor)
- 🟢 **Yeşil**: Yüklü araç (başlangıca dönüyor)
- 🟠 **Turuncu**: Bekleyen araç (cep/depoda)
- 🔵 **Mavi**: Yükleme yapıyor
- ⚫ **Gri**: Görevi tamamlamış araç

## 📁 Proje Yapısı

```
otonom-arac-sim/
├── backend/
│   ├── main.go              # Ana server
│   ├── models/
│   │   └── models.go        # Veri modelleri
│   └── simulation/
│       └── simulation.go    # Simülasyon motoru
├── frontend/
│   ├── src/
│   │   ├── App.tsx          # Ana React bileşeni
│   │   ├── App.css          # Stil dosyası
│   │   └── main.tsx         # Giriş noktası
│   └── package.json
└── README.md
```

## 🎯 Özellikler

### Mevcut Özellikler
- ✅ Gerçek zamanlı simülasyon (500ms tick)
- ✅ WebSocket ile canlı güncelleme
- ✅ Çarpışma algılama ve önleme
- ✅ Cep/depo yönetimi ve rezervasyon sistemi
- ✅ Sensör sistemi
- ✅ Görsel arayüz (SVG tabanlı)
- ✅ Araç durumu takibi (7 durum makinesi)
- ✅ Olay günlüğü
- ✅ Öncelikli araç desteği (⭐ skoru)
- ✅ Ağırlıklı skor ile optimal geri çekilme kararı
- ✅ Deadlock tespiti ve otomatik çözümü
- ✅ Optimizasyon istatistikleri (kaçınılan çarpışma, çözülen deadlock, bekleme süresi)

### Geliştirilebilir Özellikler
- ⏳ Dinamik hız ayarlama
- ⏳ Senaryo kaydetme/yükleme

## 📊 API Endpoints

### REST API

- `GET /api/state` - Mevcut simülasyon durumunu al
- `GET /api/config` - Konfigürasyon bilgilerini al
- `POST /api/control` - Simülasyonu kontrol et (start/stop/reset)
- `POST /api/vehicle/add` - Yeni araç ekle

### WebSocket

- `ws://localhost:8080/ws` - Gerçek zamanlı durum güncellemeleri

## 👥 Geliştirici

KTÜ Bilgisayar Mühendisliği 2025/26

## 📝 Lisans

Bu proje eğitim amaçlıdır.
