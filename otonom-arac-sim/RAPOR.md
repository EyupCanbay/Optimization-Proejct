# Tek Şeritli Yolda Otonom Araç Hareketinin Optimizasyonu

**Ders:** Optimizasyon  
**Proje konusu:** Tek şeritli yolda otonom araçların çarpışmadan ve kilitlenmeden hareketinin sağlanması  
**Proje türü:** Simülasyon + optimizasyon algoritması + gerçek zamanlı görselleştirme  
**Dönem:** 2025/2026  

---

## Özet

Bu projede 240 metre uzunluğunda tek şeritli bir yolda hareket eden otonom araçların güvenli biçimde depo noktalarına gidip yük alması ve başlangıç noktasına dönmesi simüle edilmiştir. Yol tek şeritli olduğu için aynı anda iki yönlü geçiş mümkün değildir. Bu nedenle araçların çarpışmadan ilerlemesi, depo ve cep kapasitelerine uyması, karşılaşma durumlarında uygun aracın bekleme alanına çekilmesi ve oluşabilecek deadlock durumlarının çözülmesi gerekir.

Projede Go dili ile bir simülasyon motoru geliştirilmiş, React + TypeScript ile gerçek zamanlı bir arayüz hazırlanmıştır. Backend her simülasyon adımında araçların durumunu günceller; frontend ise WebSocket üzerinden gelen güncel durumu yol, depo, cep, sensör ve araç görselleştirmesi olarak gösterir.

Optimizasyon yaklaşımı güvenliği birinci öncelik kabul eder. Amaç, çarpışmayı tamamen engellemek, deadlock durumlarını çözmek, depo/cep kapasite kısıtlarını korumak ve toplam bekleme süresini azaltmaktır.

---

## 1. Problem Tanımı

### 1.1 Sistem Tanımı

Sistemde araçlar 0 metre başlangıç noktasından yola çıkar. Her araç kendisine atanmış olan bir depoya gider, depoda yük alır ve başlangıç noktasına geri döner. Yol tek şeritli olduğundan karşı yönlü araçlar aynı yol kesitinde aynı anda güvenli şekilde geçemez. Bu nedenle bazı araçların cep veya depo alanlarında beklemesi gerekir.

Sistem elemanları:

| Eleman | Açıklama |
|---|---|
| Yol | 240 metre uzunluğunda tek şeritli yol |
| Araçlar | Depoya gidip yük alarak başlangıca dönen otonom araçlar |
| Depolar | 70m, 120m ve 240m konumlarında yük alma noktaları |
| Cepler | 80m ve 200m konumlarında araçların bekleyebileceği alanlar |
| Sensörler | Her 40 metrede bir bulunan yol algılama noktaları |
| Garaj kuyruğu | Fazla araçların yola çıkmadan önce beklediği başlangıç kuyruğu |

### 1.2 Ödev Kısıtları

| Kısıt | Değer / Kural |
|---|---|
| Yol uzunluğu | 240 m |
| Araç sayısı | En fazla 20 araç |
| Aktif trafik limiti | Aynı anda en fazla 7 aktif araç |
| Araç hızı | 18 km/h, maksimum 20 km/h sınırının altında |
| Minimum araç mesafesi | 20 m |
| Depo konumları | 70m, 120m, 240m |
| Depo kapasitesi | Her depoda aynı anda en fazla 3 araç |
| Cep konumları | 80m, 200m |
| Cep kapasitesi | Her cepte aynı anda en fazla 1 araç |
| Sensör aralığı | 40 m |
| Sensör algılama menzili | ±20 m |
| Güvenlik hedefi | Çarpışma olmaması ve deadlock çözümü |

Aktif trafik limiti, tek şeritli yolun ve bekleme alanlarının kapasitesine uygun olarak uygulanmıştır. Kullanıcı 20 araca kadar araç ekleyebilir; ancak sistem hepsini aynı anda dar tek şeritli hatta sokmaz. Fazla araçlar garaj kuyruğunda bekletilir. Bu, optimizasyonun bir kapasite kontrolü olarak değerlendirilmiştir.

---

## 2. Matematiksel Model

### 2.1 Kümeler

| Sembol | Açıklama |
|---|---|
| \(V\) | Araçlar kümesi |
| \(D = \{70, 120, 240\}\) | Depo konumları |
| \(P = \{80, 200\}\) | Cep konumları |
| \(T\) | Ayrık simülasyon zaman adımları |

### 2.2 Parametreler

| Sembol | Değer | Açıklama |
|---|---:|---|
| \(L\) | 240 | Yol uzunluğu |
| \(d_{min}\) | 20 | Minimum güvenli mesafe |
| \(v_{max}\) | 20 | Maksimum hız, km/h |
| \(v_i\) | 18 | Uygulamadaki araç hızı, km/h |
| \(C_D\) | 3 | Depo kapasitesi |
| \(C_P\) | 1 | Cep kapasitesi |
| \(A_{max}\) | 7 | Aynı anda aktif trafik limiti |

### 2.3 Karar Değişkenleri

Her araç \(i\) ve her zaman adımı \(t\) için:

| Değişken | Açıklama |
|---|---|
| \(x_i(t)\) | Aracın yol üzerindeki konumu |
| \(dir_i(t)\) | Aracın yönü: sağa, sola veya hareketsiz |
| \(s_i(t)\) | Aracın durumu: queued, moving, waiting, retreating, loading, exiting, done |
| \(r_i(t)\) | Aracın rezerve ettiği cep/depo noktası |
| \(w_i(t)\) | Aracın bekleme tick sayısı |

### 2.4 Amaç Fonksiyonu

Projenin amacı yalnızca araçları hareket ettirmek değil, güvenli ve verimli hareket ettirmektir. Bu nedenle amaç fonksiyonu:

\[
\min Z =
\alpha \cdot W +
\beta \cdot R +
\gamma \cdot H +
\delta \cdot Q
\]

Burada:

| Terim | Açıklama |
|---|---|
| \(W\) | Toplam bekleme süresi |
| \(R\) | Geri çekilme ve rota değiştirme maliyeti |
| \(H\) | Deadlock sayısı |
| \(Q\) | Garaj kuyruğunda bekleme maliyeti |

Güvenlik kısıtları yumuşak amaç değil, sert kısıt olarak ele alınmıştır. Yani çarpışma kabul edilebilir bir maliyet değil, kesin olarak engellenmesi gereken durumdur.

### 2.5 Kısıtlar

**Minimum mesafe kısıtı**

\[
|x_i(t) - x_j(t)| \geq d_{min}
\]

Aynı yol şeridinde aktif olan iki araç arasında en az 20m mesafe korunur. Cepte veya depoda güvenli bekleyen araçlar bu şerit mesafe kontrolünden ayrık değerlendirilir.

**Depo kapasitesi kısıtı**

\[
\sum_{i \in V} y_{i,d}(t) \leq 3 \quad \forall d \in D
\]

Bir depoda aynı anda en fazla 3 araç bulunabilir.

**Cep kapasitesi kısıtı**

\[
\sum_{i \in V} z_{i,p}(t) \leq 1 \quad \forall p \in P
\]

Bir cepte aynı anda yalnızca 1 araç bekleyebilir.

**Aktif araç kapasitesi**

\[
\sum_{i \in V} active_i(t) \leq 7
\]

Çok fazla aracın aynı anda tek şeride çıkması deadlock riskini artırdığı için fazla araçlar garaj kuyruğunda tutulur.

**Yol sınırı**

\[
0 \leq x_i(t) \leq 240
\]

Araçlar yol sınırlarının dışına çıkamaz.

---

## 3. Araç Durum Makinesi

Araçlar aşağıdaki durumlar arasında geçiş yapar:

```text
queued -> moving -> loading -> exiting -> moving -> done
             |                      ^
             v                      |
        retreating -> waiting -------
```

| Durum | Açıklama |
|---|---|
| `queued` | Araç garaj kuyruğunda bekler |
| `moving` | Araç yol üzerinde hareket eder |
| `loading` | Araç depoda yük alır |
| `exiting` | Araç depodan çıkış yapar |
| `retreating` | Araç güvenli cep/depo noktasına çekilir |
| `waiting` | Araç cepte, depoda veya başlangıçta bekler |
| `done` | Araç görevini tamamlamıştır |

Kısa süreli bekleme hata değildir. Tek şeritli yol modelinde güvenlik için araçların bazen durması gerekir. Hata kabul edilen durum, aracın uzun süre aynı yerde kalması ve olay geçmişinde sürekli aynı deadlock mesajlarının üretilmesidir.

---

## 4. Optimizasyon Algoritması

### 4.1 Genel Akış

Her simülasyon tick'inde şu işlemler yapılır:

1. Depo ve cep dolulukları gerçek araç durumlarından yeniden hesaplanır.
2. Sensörler güncellenir.
3. Bekleyen araçların `WaitTickCount` değerleri artırılır.
4. Deadlock tespiti yapılır.
5. Her araç için durumuna göre karar verilir.
6. Çarpışma riski varsa araç hareket ettirilmez veya güvenli noktaya çekilir.
7. Yeni simülasyon durumu WebSocket ile arayüze gönderilir.

### 4.2 Ağırlıklı Araç Skoru

Karşılaşma durumlarında hangi aracın öncelikli olarak devam edeceği ağırlıklı skor ile belirlenir:

\[
score(i) = progress_i + loadBonus_i + priorityBonus_i
\]

| Bileşen | Değer |
|---|---|
| `progress` | 0 ile 1 arasında tamamlanan görev oranı |
| `loadBonus` | Yüklü araç için +50 |
| `priorityBonus` | Öncelikli araç için +100 |

Yüklü araçlara yüksek bonus verilmesinin nedeni, görevin ikinci aşamasına geçmiş olmalarıdır. Bu araçların gereksiz yere geri çekilmesi toplam tamamlanma süresini artırır.

Karar kuralı:

```text
Düşük skorlu araç geri çekilir.
Yüksek skorlu araç devam eder.
Skor eşitse küçük ID'li araç geri çekilir.
```

### 4.3 Cep ve Depo Rezervasyonu

İki aracın aynı anda aynı cebi seçmesini engellemek için rezervasyon sistemi kullanılmıştır.

```text
reserveSpot(vehicleID, position)
```

Bir araç cep veya depoya yönlendirildiğinde önce o nokta rezerve edilir. Araç noktaya ulaştığında fiziksel doluluk güncellenir ve rezervasyon temizlenir. Böylece aynı tick içinde iki aracın aynı bekleme alanını hedeflemesi engellenir.

### 4.4 Deadlock Tespiti

Bir araç uzun süre bekliyorsa ve karşı yönden gelen araçlar nedeniyle ilerleyemiyorsa deadlock adayı kabul edilir.

```text
WaitTickCount > 15 ve karşı yön trafiği varsa -> deadlock
```

Deadlock çözümü:

1. Önce gerideki en yakın uygun bekleme noktası aranır.
2. Bulunamazsa ilerideki uygun bekleme noktası aranır.
3. Seçilen noktaya kadar yolun boş olup olmadığı kontrol edilir.
4. Nokta rezerve edilir.
5. Araç `retreating` durumuna geçirilir.

### 4.5 Aktif Trafik Limiti

12 ve üzeri araçlı stres testlerinde, tüm araçların aynı anda tek şeritli yola çıkarılması cep/depo alanlarını doldurmakta ve sistemin boş spot bulamamasına yol açmaktadır. Bu nedenle aktif araç sayısı sınırlandırılmıştır.

```go
const maxActiveVehicles = 7
```

Bu limit, araç eklemeyi engellemez. Araçlar sisteme eklenir ancak sırayla yola çıkar. Böylece garaj kuyruğu, tek şeritli yolun kapasitesini yöneten bir bekleme tamponu gibi çalışır.

---

## 5. Yazılım Mimarisi

### 5.1 Backend

Backend Go dili ile geliştirilmiştir.

| Dosya | Görev |
|---|---|
| `backend/main.go` | REST API, WebSocket ve simülasyon döngüsü |
| `backend/models/models.go` | Araç, depo, cep, sensör ve state modelleri |
| `backend/simulation/simulation.go` | Ana simülasyon ve optimizasyon motoru |
| `backend/simulation/simulation_test.go` | Senaryo ve regresyon testleri |

Backend her 500ms'de bir `Tick()` çağırır ve güncel `SimState` bilgisini WebSocket üzerinden frontend'e gönderir.

### 5.2 Frontend

Frontend React + TypeScript ile geliştirilmiştir.

| Dosya | Görev |
|---|---|
| `frontend/src/App.tsx` | Ana arayüz, WebSocket bağlantısı ve SVG görselleştirme |
| `frontend/src/App.css` | Arayüz tasarımı |
| `frontend/package.json` | Frontend bağımlılıkları |

Arayüzde yol, depolar, cepler, sensörler, araçlar, olay geçmişi ve optimizasyon istatistikleri görüntülenir.

### 5.3 API

| Endpoint | Metod | Açıklama |
|---|---|---|
| `/api/state` | GET | Anlık simülasyon durumu |
| `/api/config` | GET | Sistem konfigürasyonu |
| `/api/control` | POST | `start`, `stop`, `reset` |
| `/api/vehicle/add` | POST | Yeni araç ekleme |
| `/ws` | WebSocket | Gerçek zamanlı durum yayını |

---

## 6. Test Senaryoları

Testler Go'nun `testing` paketi ile yazılmıştır. Ana doğrulama komutu:

```bash
cd backend
go test ./...
```

### 6.1 Test Kapsamı

| Test | Amaç | Sonuç |
|---|---|---|
| `TestSingleVehicleCompletesJourney` | Tek aracın depoya gidip geri dönmesi | Başarılı |
| `TestHeadOnEncounterTriggersRetreat` | Karşılaşmada çarpışma olmaması | Başarılı |
| `TestMultiVehicleDeadlockResolution` | 5 araçlı karma senaryoda deadlock çözümü | Başarılı |
| `TestReverseDensityScenarioDoesNotCollide` | `70,240,70,240,120` sırası | Başarılı |
| `TestAlternatingEndsScenarioDoesNotCollide` | `240,70,240,70,240,70` sırası | Başarılı |
| `TestDepotCapacityNeverExceededForSameFarTarget` | `240,240,240,240,240` senaryosunda depo kapasitesi | Başarılı |
| `TestTwelveVehicleMixedStressDoesNotDeadlock` | 12 araçlı stres testi | Başarılı |
| `TestQueuedVehiclesDoNotBlockReturningVehicle` | Kuyruktaki aracın yoldaki aracı yanlış bloklamaması | Başarılı |
| `TestPriorityVehicleHasHigherScore` | Öncelikli aracın skor avantajı | Başarılı |
| `TestReserveSpotPreventsDoubleBooking` | Aynı cep/depo için çift rezervasyonun engellenmesi | Başarılı |
| `TestVehicleScoreLoadBonus` | Yüklü araç skor bonusu | Başarılı |
| `TestDepotCapacityEnforced` | Depo kapasite kuralı | Başarılı |
| `TestNewVehicleAssignedSpotIsMinusOne` | Yeni araç rezervasyon başlangıç değeri | Başarılı |

### 6.2 Önemli Senaryo Sonuçları

| Senaryo | Sonuç |
|---|---|
| `70, 240, 70, 240, 120` | Tüm araçlar tamamlandı |
| `240, 70, 240, 70, 240, 70` | Tüm araçlar 255 tick içinde tamamlandı |
| `240, 240, 240, 240, 240` | 240m deposunda kapasite 3'ü aşmadı |
| `70,120,240,70,120,240,70,120,240,70,120,240` | 12 araç 648 tick içinde tamamlandı |

### 6.3 Regresyon Olarak Yakalanan Hatalar

Geliştirme sırasında kullanıcı testleriyle şu problemler yakalanmış ve düzeltilmiştir:

| Problem | Çözüm |
|---|---|
| Kuyruktaki araçların yoldaymış gibi hesaba katılması | Trafik katılımcısı kontrolü eklendi |
| Araçların 0m başlangıç noktasında üst üste binmesi | Başlangıç bölgesi aktif araçla doluyken yeni araç yola çıkarılmadı |
| Depo/cep sayaçlarının gerçek durumdan sapması | Doluluklar her tick gerçek araçlardan yeniden hesaplandı |
| Aynı depoda 3'ten fazla araç görünmesi | Depo kapasitesi gerçek araç sayımı ile kontrol edildi |
| 12 araçta boş spot bulunamayıp deadlock döngüsüne girilmesi | Aktif trafik limiti ve garaj kuyruğu kapasite kontrolü eklendi |
| Geri çekilen aracın başka araçla yaklaşması | Retreating hareketi için ayrı mesafe kontrolü eklendi |

---

## 7. Güvenlik ve Optimizasyon Değerlendirmesi

Projenin güvenlik açısından sağladığı kurallar:

1. Aktif yol üzerindeki araçlar arasında minimum 20m mesafe korunur.
2. Depoda aynı anda en fazla 3 araç bulunur.
3. Cepte aynı anda en fazla 1 araç bulunur.
4. Karşı yönlü araçlar karşılaştığında düşük skorlu araç güvenli noktaya çekilir.
5. Deadlock oluşursa bekleme süresi izlenir ve araç uygun spotlara yönlendirilir.
6. Fazla yoğunluk oluşursa araçlar yola alınmadan garaj kuyruğunda bekletilir.

Optimizasyon açısından:

- Çarpışma riski sert kısıt olarak ele alınmıştır.
- Bekleme süresi `OptStats.TotalWaitTime` ile izlenmiştir.
- Çözülen deadlock sayısı `DeadlockCount` ve `OptStats.DeadlockResolved` ile ölçülmüştür.
- Önlenen çarpışma sayısı `OptStats.CollisionAvoided` ile takip edilmiştir.

---

## 8. Kısa Bekleme Davranışının Açıklaması

Simülasyonda araçların bazen 2-5 saniye beklemesi normaldir. Çünkü tek şeritli yolda sürekli hareket hedefi güvenlik açısından doğru değildir. Bir araç karşıdan gelen araç, dolu depo, dolu cep veya takip mesafesi nedeniyle kısa süre bekleyebilir.

Bu projede hedef:

```text
Sürekli hareket değil,
güvenli, kapasiteli ve deadlock'suz hareket.
```

Bu nedenle kısa beklemeler sistemin doğru çalıştığını gösterir. Ancak uzun süreli bekleme ve sürekli aynı deadlock mesajının oluşması hata kabul edilmiştir; bu durumlar test senaryoları ile yakalanıp düzeltilmiştir.

---

## 9. Çalıştırma

Backend:

```bash
cd backend
go run main.go
```

Backend adresi:

```text
http://localhost:8080
```

Frontend:

```bash
cd frontend
npm install
npm run dev
```

Frontend adresi:

```text
http://localhost:5173
```

Testler:

```bash
cd backend
go test ./...
```

---

## 10. Sonuç

Bu projede tek şeritli yolda çalışan otonom araçlar için güvenli trafik yönetimi ve optimizasyon tabanlı karar verme sistemi geliştirilmiştir. Sistem araçları depo hedeflerine yönlendirmekte, yük alma ve geri dönüş süreçlerini yönetmekte, karşılaşma durumlarında uygun aracı güvenli bekleme noktasına çekmekte ve oluşabilecek deadlock durumlarını çözmektedir.

Proje yalnızca görsel simülasyon değil, kısıt tabanlı bir optimizasyon problemidir. Araçlar arası minimum mesafe, depo kapasitesi, cep kapasitesi, aktif araç kapasitesi ve hedef tamamlama kuralları birlikte ele alınmıştır. Yapılan testlerde tek araç, çoklu araç, karşılaşma, aynı hedef yoğunluğu, ters yoğunluk, alternatif uçlar ve 12 araçlı stres senaryoları başarıyla tamamlanmıştır.

Sonuç olarak sistem:

- Çarpışmaları önler.
- Depo ve cep kapasite kısıtlarını korur.
- Deadlock durumlarını tespit eder ve çözer.
- Fazla yoğunlukta garaj kuyruğu ile sistemi dengeler.
- Gerçek zamanlı olarak izlenebilir ve test edilebilir.

Bu özellikleriyle proje, verilen optimizasyon problemi için çalışır ve savunulabilir bir çözüm sunmaktadır.
