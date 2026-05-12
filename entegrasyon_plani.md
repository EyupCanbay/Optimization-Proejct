# 🗺️ Optimizasyon & Deadlock Çözme — Entegrasyon Yol Haritası

## 1. Mevcut Sistemin Özeti

### Veri Akışı (Genel Bakış)

```
[REST POST /api/vehicle/add]
         │
         ▼
  sim.AddVehicle()  →  Vehicles[] listesine eklenir (Status: queued)
         │
         ▼
[Goroutine: her 500ms]
  sim.Tick()        →  Her araç için karar verilir
  sim.GetState()    →  SimState JSON'a çevrilir
  hub.broadcast()   →  Tüm WS istemcilerine gönderilir
         │
         ▼
  [React Frontend]
  socket.onmessage  →  setState(data)  →  SVG render
```

### Durum Makinesi (Her Araç)

```
queued → moving → [hedef depoya ulaşınca] loading
                                             │
                                             ▼
                                          exiting → moving (DirLeft)
                                                        │
                                          ┌─────────────┤
                                          │ çarpışma/mesafe ihlali
                                          ▼
                                      retreating → waiting → moving
                                                               │
                                                               ▼
                                                        [0m'e varınca] done
```

### Mevcut Fonksiyonlar (simulation.go)

| Fonksiyon | Satır | Görevi |
|---|---|---|
| `Tick()` | 159–487 | Ana döngü. Her araç için durum geçişi |
| `updateSensors()` | 490–526 | Sensör kırmızı/yeşil durumu |
| `hasOncomingVehicle()` | 528–564 | Karşı yönde araç var mı? (50m içinde) |
| `wouldCollide()` | 566–618 | Sonraki pozisyonda çarpışma var mı? |
| `findNearestWaitingSpot()` | 620–669 | Geride kalan en yakın boş cep/depo |
| `occupyWaitingSpot()` | 672–686 | Cep/depoyu dolu işaretle |
| `releaseWaitingSpot()` | 688–707 | Cep/depoyu boş işaretle |
| `getFrontVehicle()` | 709–770 | Aynı yönde öndeki araç |

---

## 2. Tespit Edilen Eksiklikler & Sorunlar

### A. Deadlock Durumları (Mevcut Kodda Çözülmemiş)

Şu an `findNearestWaitingSpot()` sadece **geride** kalan cep/depoları arar.  
Bu yüzden bazı senaryolarda araç ne ilerleyebilir ne de geri çekilebilir:

| Senaryo | Sorun |
|---|---|
| DirRight araç 180m'deyken karşıdan araç geliyor | 200m'deki cep önünde (ileride) → atlanıyor. 80m'deki cep çok uzak → tercih edilmiyor. Sonuç: cep bulunamıyor (`spot = -1`), araç donuyor |
| İki araç aynı anda aynı cebi hedef aldığında | `occupyWaitingSpot` iki kez çağrılabiliyor → cep kapasitesi aşılıyor |
| Ceplerin tamamı dolu + depo kapasitesi dolu | `mySpot = -1` → `shouldRetreat = false` → iki araç birbirine kilitlenir |
| Depo dolu olduğunda hedef depo değiştirilemiyor | Araç depo önünde sıkışıyor |

### B. 20m Mesafe Kuralı Tutarsızlığı

`wouldCollide()` aynı yöndeki araçlar için:
```go
if dist < float64(cfg.MinDistance) && dist > 0 { return true }
```
Ama `getFrontVehicle()` takip mesafesini `1.5 * MinDistance` ile ölçüyor. Bu iki eşik birbiriyle çelişiyor.

### C. Optimizasyon Kararında Öncelik Eksik

Şu an hangi aracın geri çekileceği "kendi cep mesafesi vs. diğerinin cep mesafesi" karşılaştırmasıyla belirleniyor, ancak bu karar:
- Araç önceliğini (`priority` alanı var ama hiç kullanılmıyor)
- Araçların ne kadar yol kat ettiğini (katettikleri mesafe / görev uzaklığı)
- Ağırlık skorunu (weighted score)  
dikkate **almıyor**.

---

## 3. Entegrasyon Planı

### Genel Strateji

> **Mevcut WebSocket akışını, `SimState` yapısını ve REST endpoint'lerini değiştirmiyoruz.**  
> Tüm değişiklikler **`simulation/simulation.go`** içinde kalacak.  
> `models/models.go`'ya **sadece yeni alan ekleyeceğiz** (geriye dönük uyumlu).  
> Frontend hiç değişmeyecek (yeni alanlar JSON'da otomatik iletilir).

---

### ADIM 1 — `models/models.go` Değişiklikleri

**Dosya:** `backend/models/models.go`  
**Değişiklik türü:** Alan ekleme (mevcut alanlar değişmiyor)

```go
// Vehicle struct'a eklenecek alanlar:
WaitTickCount  int  `json:"waitTickCount"`  // Kaç tick bekledi (deadlock tespiti için)
AssignedSpot   int  `json:"assignedSpot"`   // Rezerve edilmiş spot (-1 = yok)
```

```go
// SimState struct'a eklenecek alan:
DeadlockCount  int      `json:"deadlockCount"`   // Toplam çözülen deadlock sayısı
OptStats       OptStats `json:"optStats"`         // Optimizasyon istatistikleri
```

```go
// Yeni struct (models.go'ya eklenir):
type OptStats struct {
    TotalWaitTime    int `json:"totalWaitTime"`    // Toplam bekleme tick'i
    CollisionAvoided int `json:"collisionAvoided"` // Kaçınılan çarpışma sayısı
    DeadlockResolved int `json:"deadlockResolved"` // Çözülen deadlock sayısı
}
```

---

### ADIM 2 — `simulation/simulation.go` Yeni Fonksiyonlar

**Dosya:** `backend/simulation/simulation.go`  
**Ekleme yeri:** Mevcut yardımcı fonksiyonların altına (771. satırdan sonra)

#### 2.1 — `detectDeadlock()` → Yeni fonksiyon

Bir araç `n` tick'ten fazla `waiting` veya `retreating` durumundaysa ve önünde hala karşı yön araç varsa → deadlock.

```go
func (s *Simulation) detectDeadlock() []int
// Döndürür: deadlock'taki araç ID listesi
```

**Mantık:**
```
Her araç için:
  if WaitTickCount > DEADLOCK_THRESHOLD (örn: 15 tick):
    if hala hasOncomingVehicle() == true:
      bu araç deadlock'ta → listeye ekle
```

#### 2.2 — `resolveDeadlock()` → Yeni fonksiyon

Deadlock tespit edildiğinde zorunlu yer değiştirme kararı.

```go
func (s *Simulation) resolveDeadlock(vehicleIDs []int) []string
// Döndürür: event mesajları
```

**Karar mekanizması (ağırlıklı skor):**

```
Her deadlock araçsı için skor hesapla:
  score = (v.TargetDepo - v.Position) / v.TargetDepo  // Tamamlanan yol oranı
  if v.Priority: score += 100                          // Öncelikli araç avantajı
  if v.HasLoad:  score += 50                           // Yüklü araç avantajı

En düşük skorlu araç zorla geri çekilir:
  findForwardWaitingSpot(v)  // İLERİDE de spot arar (normal mantığın tersi)
```

#### 2.3 — `findForwardWaitingSpot()` → Yeni fonksiyon

Mevcut `findNearestWaitingSpot()` sadece geride arar. Bu yeni fonksiyon deadlock çözümünde **her iki yönde** bakar.

```go
func (s *Simulation) findForwardWaitingSpot(v *models.Vehicle) int
// Araç yönünün ilerisindeki en yakın boş cep/depo
```

#### 2.4 — `reserveSpot()` → Mevcut `occupyWaitingSpot()`'ın yerine

İki araç aynı anda aynı cebe gitmesini engellemek için **rezervasyon sistemi**:

```go
func (s *Simulation) reserveSpot(vehicleID int, pos int) bool
// true: rezervasyon başarılı, false: zaten rezerve
// Vehicle.AssignedSpot'u set eder
```

Mevcut `occupyWaitingSpot()` → sadece fiziksel işgali yönetir (değişmez).  
`reserveSpot()` → `retreating` kararı verildiği anda çağrılır.

#### 2.5 — `calculateOptimalRetreat()` → Mevcut `wouldCollide` bloğunun yerine

`Tick()` içindeki karşılaşma kararını merkezileştirir.

```go
func (s *Simulation) calculateOptimalRetreat(v *models.Vehicle) (shouldRetreat bool, spot int, events []string)
```

**İçinde kullanılacak ağırlıklı karar:**
1. Her iki aracın skoru hesapla
2. Düşük skorlu araç geri çekilir
3. Eşit skorsa önce küçük ID geri çekilir (deterministik davranış)
4. `reserveSpot()` ile spot rezerve et → çift rezervasyon engelle

---

### ADIM 3 — `Tick()` Fonksiyonu İçine Entegrasyon

**Dosya:** `backend/simulation/simulation.go`  
**Değişiklik yeri:** `Tick()` fonksiyonu içi

```
Tick() akış değişikliği:

[mevcut]                           [yeni]
s.state.Tick++                     s.state.Tick++
s.updateSensors()                  s.updateSensors()

for each vehicle:                  // YENİ: WaitTickCount arttır
  ...                              s.incrementWaitCounters()

                                   // YENİ: Deadlock tespiti
                                   deadlocked := s.detectDeadlock()
                                   if len(deadlocked) > 0:
                                     events += s.resolveDeadlock(deadlocked)

for each vehicle:                  for each vehicle:
  StatusMoving bloğu içinde:         StatusMoving bloğu içinde:
    if s.wouldCollide(...):    →       dlEvents := s.calculateOptimalRetreat(v)
      [eski manuel karar]              events = append(events, dlEvents...)
```

#### Tick() içine eklenen küçük yardımcı:

```go
func (s *Simulation) incrementWaitCounters() {
  for i := range s.state.Vehicles {
    v := &s.state.Vehicles[i]
    if v.Status == StatusWaiting || v.Status == StatusRetreating {
      v.WaitTickCount++
    } else {
      v.WaitTickCount = 0  // Reset: hareket ettiğinde sıfırla
    }
  }
}
```

---

### ADIM 4 — Hangi Satırlar Değişecek (Özet)

| Dosya | Satır Aralığı | Değişiklik |
|---|---|---|
| `models/models.go` | 37–47 (Vehicle struct) | `WaitTickCount`, `AssignedSpot` alanı eklenir |
| `models/models.go` | 80–90 (SimState struct) | `DeadlockCount`, `OptStats` alanı eklenir |
| `models/models.go` | — | `OptStats` struct eklenir |
| `simulation/simulation.go` | 167–168 (Tick başı) | `s.incrementWaitCounters()` çağrısı |
| `simulation/simulation.go` | 175–176 (updateSensors sonrası) | `detectDeadlock()` + `resolveDeadlock()` çağrısı |
| `simulation/simulation.go` | 387–426 (wouldCollide bloğu) | `calculateOptimalRetreat()` ile değiştirilir |
| `simulation/simulation.go` | 421–424 (retreating kararı) | `reserveSpot()` çağrısı eklenir |
| `simulation/simulation.go` | 346–358 (retreating hareketi) | `WaitTickCount++` zaten `incrementWaitCounters`'da |
| `simulation/simulation.go` | 771+ (dosya sonu) | 5 yeni fonksiyon eklenir |

---

### ADIM 5 — Frontend'e Yansıma (Değişiklik Gerektirmez)

`models.go`'ya eklenen yeni alanlar (`waitTickCount`, `deadlockCount`, `optStats`) JSON olarak otomatik iletilir.  
Frontend'de bunları görüntülemek isterseniz:

- `SimState` interface'ine `deadlockCount: number` eklenebilir
- Sistem Bilgisi paneline `<li>Deadlock Çözüldü: {state.deadlockCount}</li>` eklenebilir
- Bu **opsiyonel** ve sonradan yapılabilir — zorunlu değil.

---

## 4. Algoritmanın Özeti (Karar Ağacı)

```
Her 500ms Tick():
│
├─ [Her araç] WaitTickCount güncelle
│
├─ Deadlock Tespiti:
│   └─ WaitTickCount > 15 VE karşıdan araç hala geliyor?
│       └─ YES → resolveDeadlock() → Zorla geri çekil (ileriye de bakabilir)
│
└─ [Her Moving araç] Karşılaşma kontrolü (calculateOptimalRetreat):
    │
    ├─ wouldCollide() == false → Normal hareket
    │
    └─ wouldCollide() == true:
        ├─ Skor hesapla (her iki araç için)
        │   score = tamamlanan_yol_oranı + öncelik_bonusu + yük_bonusu
        │
        ├─ Yüksek skorlu araç devam eder
        │
        └─ Düşük skorlu araç:
            ├─ findNearestWaitingSpot() → Geride boş spot var mı?
            │   YES → reserveSpot() → StatusRetreating
            │
            └─ Spot yok → findForwardWaitingSpot() → İleride spot var mı?
                YES → reserveSpot() → StatusRetreating (ileriye gider, karşıdaki geçer)
                NO  → WaitTickCount artar → Deadlock döngüsüne girecek
```

---

## 5. Dosya Bağımlılık Haritası

```
main.go
  └─ simulation.New(cfg)        [simulation.go]
  └─ sim.Tick()                 [simulation.go]  ← Optimizasyon buraya giriyor
  └─ sim.GetState()             [simulation.go]
  └─ sim.AddVehicle()           [simulation.go]
  └─ models.SimConfig           [models.go]
  └─ models.SimState            [models.go]      ← Yeni alanlar buraya giriyor
  └─ hub.broadcast(JSON)        [main.go]        ← DOKUNULMAYACAK
  
frontend/App.tsx
  └─ socket.onmessage           ← DOKUNULMAYACAK
  └─ SimState interface         ← Opsiyonel küçük ekleme
```

---

## 6. Uygulama Sırası

1. `models/models.go` → Yeni alanları ekle (5 dk)
2. `simulation/simulation.go` → 5 yeni fonksiyon yaz (ana iş)
3. `simulation/simulation.go` → `Tick()` içine entegre et
4. Backend'i yeniden derle & test et
5. (Opsiyonel) Frontend'de `deadlockCount` göster

> [!IMPORTANT]
> `WebSocket`, `hub`, `broadcast`, `main.go` endpoint'leri ve `models.SimState`'in mevcut alanlarına **hiç dokunulmayacak**.  
> Sadece ekleme yapılacak, silme/değiştirme yok.

> [!NOTE]
> `reserveSpot()` mekanizması, iki araç aynı tick'te aynı cep için karar verdiğinde oluşan **race condition**'ı engellemek için kritiktir.  
> `occupyWaitingSpot()` zaten var; `reserveSpot()` onun üstüne bir katman ekler.
