# 🚀 Hızlı Başlangıç Kılavuzu

## ✅ Şu Anda Çalışan Servisler

### Backend (Go)
- **Adres**: http://localhost:8080
- **Durum**: ✅ ÇALIŞIYOR
- **WebSocket**: ws://localhost:8080/ws

### Frontend (React)
- **Adres**: http://localhost:5173 (port meşgulse Vite otomatik olarak 5174, 5175 seçer — terminalde gösterilen adresi kullanın)
- **Durum**: ✅ ÇALIŞIYOR

## 🎮 Nasıl Kullanılır?

### 1. Tarayıcıda Aç
Tarayıcınızda şu adresi açın:
```
http://localhost:5173
```
(Vite farklı bir port seçtiyse terminalde gösterilen adresi kullanın)

### 2. Araç Ekle
- Sağ üstte **"+ Araç Ekle → 70m"**, **"+ Araç Ekle → 120m"** veya **"+ Araç Ekle → 240m"** butonlarından birine tıklayın
- Her buton, o depoya gidecek bir araç ekler

### 3. Simülasyonu Başlat
- **"▶ Başlat"** butonuna tıklayın
- Araçlar hareket etmeye başlayacak

### 4. İzle
- Araçların yol üzerinde hareketini izleyin
- Alt panelde araç durumlarını ve olayları görebilirsiniz

## 🎨 Ekran Açıklaması

### Üst Panel
- **Bağlı**: WebSocket bağlantı durumu (● Bağlı = yeşil)
- **Tick**: Simülasyon adım sayısı (her 500ms bir tick)
- **Araç**: Sistemdeki toplam araç sayısı
- **🔒 Deadlock**: Otomatik olarak çözülen deadlock sayısı

### Yol Görünümü
- **Siyah çizgi**: 240 metre uzunluğunda tek şeritli yol
- **Yeşil/Kırmızı daireler**: Sensörler (kırmızı = araç algılandı)
- **Yeşil kutular**: Cepler (araçların bekleyebileceği yerler)
- **Mavi kutular**: Depolar (yük alınan yerler)
- **Renkli dikdörtgenler**: Araçlar

### Araç Renkleri
- 🔴 **Kırmızı**: Boş araç (depoya gidiyor)
- 🟢 **Yeşil**: Yüklü araç (başlangıca dönüyor)
- 🟠 **Turuncu**: Bekleyen araç
- 🔵 **Mavi**: Yükleme yapıyor
- ⚫ **Gri**: Görevi tamamladı

### Alt Paneller
1. **Araç Durumları**: Her aracın ID, pozisyon, durum, bekleme tick sayısı (kırmızı = deadlock riski)
2. **Olaylar**: Simülasyonda olan önemli olaylar (çarpışma önleme, geri çekilme, deadlock çözümü)
3. **Sistem Bilgisi + Optimizasyon İstatistikleri**:
   - 🛑 Kaçınılan Çarpışma: `calculateOptimalRetreat` ile önlenen çarpışma sayısı
   - ⚠️ Çözülen Deadlock: `resolveDeadlock` ile çözülen kilitlenme sayısı
   - ⏱ Toplam Bekleme: Tüm araçların toplam bekleme tick'i

## 🧪 Test Senaryoları

### Senaryo 1: Tek Araç
1. "Sıfırla" butonuna tıklayın
2. "Araç Ekle → 120m" butonuna tıklayın
3. "Başlat" butonuna tıklayın
4. Aracın 120m'ye gidip yük alıp geri dönmesini izleyin

### Senaryo 2: Karşılaşma
1. "Sıfırla" butonuna tıklayın
2. "Araç Ekle → 240m" butonuna tıklayın
3. "Başlat" butonuna tıklayın
4. Araç 240m'ye ulaşmadan "Durdur" butonuna tıklayın
5. "Araç Ekle → 70m" butonuna tıklayın (bu araç hemen yük alıp geri dönecek)
6. "Başlat" butonuna tıklayın
7. İki aracın karşılaşmasını ve birinin cepe girmesini izleyin

### Senaryo 3: Çoklu Araç
1. "Sıfırla" butonuna tıklayın
2. 3-4 araç ekleyin (farklı depolara)
3. "Başlat" butonuna tıklayın
4. Araçların birbirlerinden kaçınmasını izleyin

### Senaryo 4: Öncelikli Araç
1. "Sıfırla" butonuna tıklayın
2. Normal bir araç ekleyin (+ Araç → 240m)
3. "Başlat" butonuna tıklayın
4. Araç yoldayken ⭐ butonu ile öncelikli araç ekleyin
5. Öncelikli aracın (⭐ altın çerçeveli) normal aracı geri çekilmeye zorladığını izleyin

### Senaryo 5: Deadlock Çözümü
1. "Sıfırla" butonuna tıklayın
2. 4-5 araç ekleyin (hem 70m hem 240m hedefli)
3. "Başlat" butonuna tıklayın
4. Tüm cep/depolar dolduğunda sistem otomatik deadlock çözümü devreye girer
5. Olay günlüğünde "⚠️ DEADLOCK çözüldü" mesajlarını ve üst panelde 🔒 sayacını izleyin

## 🛑 Durdurma

Servisleri durdurmak için:
- Backend ve Frontend terminallerinde `Ctrl+C` yapın
- Veya terminalleri kapatın

## 🔧 Sorun Giderme

### Frontend bağlanamıyor
- Backend'in çalıştığından emin olun: http://localhost:8080/api/config
- Tarayıcı konsolunu kontrol edin (F12)

### Araçlar hareket etmiyor
- "Başlat" butonuna tıkladığınızdan emin olun
- "Tick" sayısının artıp artmadığını kontrol edin

### Port zaten kullanımda
- Backend için: 8080 portunu kullanan başka bir program varsa kapatın
- Frontend için: Otomatik olarak başka bir port seçilir (5174, 5175, vb.)

## 📊 Sistem Parametreleri

- **Yol Uzunluğu**: 240m
- **Sensör Aralığı**: 40m (her 40m'de bir sensör)
- **Sensör Menzili**: ±20m
- **Min Araç Mesafesi**: 20m
- **Araç Hızı**: 18 km/h (varsayılan, maksimum 20 km/h)
- **Cep Konumları**: 80m, 200m
- **Depo Konumları**: 70m, 120m, 240m
- **Cep Kapasitesi**: 1 araç
- **Depo Kapasitesi**: 3 araç

## 🎯 Sonraki Adımlar

Projeyi geliştirmek için:
1. `backend/simulation/simulation.go` - Simülasyon mantığını düzenleyin
2. `frontend/src/App.tsx` - Arayüzü özelleştirin
3. Yeni özellikler ekleyin (öncelikli araçlar, dinamik hız, vb.)

İyi çalışmalar! 🚗💨
