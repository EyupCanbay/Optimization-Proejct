package simulation

import (
	"fmt"
	"math"
	"otonom-arac-sim/models"
	"sync"
)

const maxActiveVehicles = 7

// Simulation simülasyon motorunu temsil eder
type Simulation struct {
	mu     sync.Mutex
	state  models.SimState
	nextID int
}

// New yeni bir simülasyon oluşturur
func New(cfg models.SimConfig) *Simulation {
	sim := &Simulation{
		nextID: 1,
	}

	// Sensörleri oluştur
	sensors := []models.Sensor{}
	for pos := 0; pos <= cfg.RoadLength; pos += cfg.SensorSpacing {
		sensors = append(sensors, models.Sensor{Position: pos, Red: false})
	}

	// Cepleri oluştur
	pockets := []models.Pocket{}
	for _, pos := range cfg.PocketPositions {
		pockets = append(pockets, models.Pocket{Position: pos, Occupied: false})
	}

	// Depoları oluştur
	depots := []models.Depot{}
	for _, pos := range cfg.DepotPositions {
		depots = append(depots, models.Depot{
			Position:     pos,
			VehicleCount: 0,
			MaxCapacity:  cfg.MaxDepotCap,
		})
	}

	sim.state = models.SimState{
		Tick:     0,
		Vehicles: []models.Vehicle{},
		Pockets:  pockets,
		Depots:   depots,
		Sensors:  sensors,
		Config:   cfg,
		Running:  false,
		Events:   []string{},
	}

	return sim
}

// GetState mevcut durumu döndürür (thread-safe)
func (s *Simulation) GetState() models.SimState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// AddVehicle yeni araç ekler
func (s *Simulation) AddVehicle(targetDepo int, priority bool) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.state.Vehicles) >= 20 {
		return 0, fmt.Errorf("maksimum araç sayısına ulaşıldı (20)")
	}

	// Hedef depo geçerli mi kontrol et
	validDepo := false
	for _, d := range s.state.Depots {
		if d.Position == targetDepo {
			validDepo = true
			break
		}
	}
	if !validDepo {
		return 0, fmt.Errorf("geçersiz depo konumu: %d", targetDepo)
	}

	id := s.nextID
	s.nextID++

	vehicle := models.Vehicle{
		ID:           id,
		Position:     -1, // Garajda (yol dışı)
		Direction:    models.DirRight,
		Speed:        18,                  // Varsayılan hız: 18 km/h
		Status:       models.StatusQueued, // Kuyrukta bekliyor
		TargetDepo:   targetDepo,
		HasLoad:      false,
		Priority:     priority,
		WaitingAt:    -1,
		AssignedSpot: -1, // Rezervasyon yok (0 geçerli bir yol pozisyonu olduğundan -1 kullanılmalı)
	}

	s.state.Vehicles = append(s.state.Vehicles, vehicle)
	s.state.Events = append(s.state.Events,
		fmt.Sprintf("Araç #%d kuyrukta → Hedef depo: %dm", id, targetDepo))

	return id, nil
}

// SetRunning simülasyonu başlatır/durdurur
func (s *Simulation) SetRunning(running bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Running = running
	if running {
		s.state.Events = append(s.state.Events, "Simülasyon başlatıldı")
	} else {
		s.state.Events = append(s.state.Events, "Simülasyon durduruldu")
	}
}

// Reset simülasyonu sıfırlar
func (s *Simulation) Reset(cfg models.SimConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sensors := []models.Sensor{}
	for pos := 0; pos <= cfg.RoadLength; pos += cfg.SensorSpacing {
		sensors = append(sensors, models.Sensor{Position: pos, Red: false})
	}

	pockets := []models.Pocket{}
	for _, pos := range cfg.PocketPositions {
		pockets = append(pockets, models.Pocket{Position: pos, Occupied: false})
	}

	depots := []models.Depot{}
	for _, pos := range cfg.DepotPositions {
		depots = append(depots, models.Depot{
			Position:     pos,
			VehicleCount: 0,
			MaxCapacity:  cfg.MaxDepotCap,
		})
	}

	s.nextID = 1
	s.state = models.SimState{
		Tick:     0,
		Vehicles: []models.Vehicle{},
		Pockets:  pockets,
		Depots:   depots,
		Sensors:  sensors,
		Config:   cfg,
		Running:  false,
		Events:   []string{"Simülasyon sıfırlandı"},
	}
}

// Tick simülasyonu bir adım ilerletir
func (s *Simulation) Tick() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.state.Running {
		return
	}

	s.state.Tick++
	events := []string{}

	cfg := s.state.Config
	// dt: her tick 1.0 saniye = 1.0/3600 saat
	dt := 1.0 / 3600.0

	// Önceki ticklerden kalan sayaç sapmalarını temizle
	s.syncSpotOccupancy()

	// Sensörleri güncelle
	s.updateSensors()

	// Bekleme sayaçlarını artır (deadlock tespiti için gerekli)
	s.incrementWaitCounters()

	// Deadlock tespiti: eşiği aşan araçlar varsa zorla çöz
	if deadlocked := s.detectDeadlock(); len(deadlocked) > 0 {
		dlEvents := s.resolveDeadlock(deadlocked)
		events = append(events, dlEvents...)
	}

	// Her aracı işle
	for i := range s.state.Vehicles {
		v := &s.state.Vehicles[i]

		if v.Status == models.StatusDone {
			continue
		}

		// Kuyrukta bekliyor mu?
		if v.Status == models.StatusQueued {
			// Önünde başka KUYRUKTA araç var mı?
			canEnterRoad := true
			if s.countActiveVehicles() >= maxActiveVehicles {
				canEnterRoad = false
			}
			for _, other := range s.state.Vehicles {
				if other.ID == v.ID || other.Status == models.StatusDone {
					continue
				}
				// Diğer araç daha önce mi eklendi VE hala kuyrukta mı?
				if other.ID < v.ID && other.Status == models.StatusQueued {
					canEnterRoad = false
					break
				}
				// VEYA başlangıç bölgesinde aktif araç var mı?
				if s.blocksRoadEntry(&other) {
					canEnterRoad = false
					break
				}
			}

			if canEnterRoad {
				// Yola çık!
				v.Position = 0
				v.Status = models.StatusMoving
				events = append(events, fmt.Sprintf("Araç #%d yola çıktı!", v.ID))
			}
			continue
		}

		// Depoda yükleme yapıyor mu?
		if v.Status == models.StatusLoading {
			// Yükleme tamamlandı, depodan çıkmaya başla
			v.HasLoad = true
			v.Status = models.StatusExiting
			v.Direction = models.DirLeft
			events = append(events, fmt.Sprintf("Araç #%d yük aldı, depodan çıkıyor", v.ID))
			continue
		}

		// Depodan çıkıyor mu?
		if v.Status == models.StatusExiting {
			// Bu depoda başka "exiting" durumunda araç var mı?
			// Varsa ve onun ID'si daha küçükse (daha önce eklendi), bu araç beklemeli
			canExitNow := true
			for _, other := range s.state.Vehicles {
				if other.ID == v.ID {
					continue
				}
				// Aynı depoda mı?
				if math.Abs(other.Position-v.Position) < 1 && other.Status == models.StatusExiting {
					// Diğer araç daha önce mi eklendi? (ID küçükse daha önce eklendi)
					if other.ID < v.ID {
						canExitNow = false
						break
					}
				}
			}

			if !canExitNow {
				// Sırada bekle
				continue
			}

			// Yol açık mı kontrol et
			if !s.wouldCollide(v, dt) {
				v.Status = models.StatusMoving
				v.WaitingAt = -1
				s.syncSpotOccupancy()
				events = append(events, fmt.Sprintf("Araç #%d depodan çıktı, yola girdi", v.ID))
			}
			// Yol kapalıysa depoda bekle
			continue
		}

		// Bekliyor mu?
		if v.Status == models.StatusWaiting {
			// Aynı cep/depoda başka bekleyen araç var mı?
			// Varsa ve onun ID'si daha küçükse, bu araç sırada beklemeli
			canCheckExit := true
			if v.WaitingAt >= 0 {
				for _, other := range s.state.Vehicles {
					if other.ID == v.ID {
						continue
					}
					// Aynı cep/depoda mı?
					if other.WaitingAt == v.WaitingAt && other.Status == models.StatusWaiting {
						// Diğer araç daha önce mi eklendi?
						if other.ID < v.ID {
							canCheckExit = false
							break
						}
					}
				}
			}

			if !canCheckExit {
				// Sırada bekle
				continue
			}

			// Yol açık mı kontrol et (hem karşı yön hem de aynı yön)
			canExit := true

			if s.hasNearbyRoadVehicle(v, float64(cfg.MinDistance)*2) {
				canExit = false
			}

			// Karşı yönden araç var mı?
			if canExit && s.hasOncomingVehicle(v) {
				canExit = false
			}

			// Aynı yönde çok yakın araç var mı?
			if canExit {
				frontVehicle := s.getFrontVehicle(v)
				if frontVehicle != nil {
					dist := math.Abs(frontVehicle.Position - v.Position)
					if dist < float64(cfg.MinDistance)*2 {
						canExit = false
					}
				}
			}

			if canExit {
				v.Status = models.StatusMoving
				// Cep/depo doluluğunu güncelle
				s.releaseWaitingSpot(v)
				v.WaitingAt = -1

				// ÖNEMLI: Yönü yük durumuna göre ayarla
				if v.HasLoad {
					v.Direction = models.DirLeft // Yüklü araç geri dönüyor
				} else {
					v.Direction = models.DirRight // Boş araç depoya gidiyor
				}

				events = append(events, fmt.Sprintf("Araç #%d yol açıldı, %s yönünde hareket ediyor", v.ID, v.Direction.String()))
			}
			continue
		}

		// Geri çekiliyor mu?
		if v.Status == models.StatusRetreating {
			// Hedef cep/depoya doğru geri git
			if v.WaitingAt < 0 {
				// Hata: hedef belirlenmemiş, normal harekete geç
				v.Status = models.StatusMoving
				continue
			}

			speedMs := v.Speed * 1000.0 / 3600.0
			delta := speedMs * dt * 3600

			targetPos := float64(v.WaitingAt)

			// Hedefe doğru hareket et
			if v.Position < targetPos {
				nextPos := math.Min(v.Position+delta, targetPos)
				if nextPos != targetPos && s.wouldRetreatMoveCollide(v, nextPos) {
					continue
				}
				v.Position = nextPos
				if v.Position >= targetPos {
					v.Position = targetPos
					v.Status = models.StatusWaiting
					s.occupyWaitingSpot(v, v.WaitingAt)
					// Fiziksel işgal tamamlandı → reservasyon artık gerekmiyor
					s.releaseReservation(v.ID)
					events = append(events, fmt.Sprintf("Araç #%d %dm'de beklemeye başladı", v.ID, v.WaitingAt))
				}
			} else {
				nextPos := math.Max(v.Position-delta, targetPos)
				if nextPos != targetPos && s.wouldRetreatMoveCollide(v, nextPos) {
					continue
				}
				v.Position = nextPos
				if v.Position <= targetPos {
					v.Position = targetPos
					v.Status = models.StatusWaiting
					s.occupyWaitingSpot(v, v.WaitingAt)
					// Fiziksel işgal tamamlandı → reservasyon artık gerekmiyor
					s.releaseReservation(v.ID)
					events = append(events, fmt.Sprintf("Araç #%d %dm'de beklemeye başladı", v.ID, v.WaitingAt))
				}
			}
			continue
		}

		// Hareket eden araç
		if v.Status == models.StatusMoving {
			// Önce aynı yönde giden araçları kontrol et (takip mesafesi)
			frontVehicle := s.getFrontVehicle(v)
			if frontVehicle != nil {
				// Önündeki araçla arasındaki mesafe
				dist := math.Abs(frontVehicle.Position - v.Position)
				if dist < float64(cfg.MinDistance)*1.5 {
					// Çok yakın, yavaşla veya dur
					if dist < float64(cfg.MinDistance) {
						// Çok yakın! Geri çekil
						spot := s.findNearestWaitingSpot(v)
						if spot >= 0 && s.reserveSpot(v.ID, spot) {
							v.Status = models.StatusRetreating
							v.WaitingAt = spot
							events = append(events, fmt.Sprintf("Araç #%d önündeki araca çok yakın! %dm'ye geri çekiliyor", v.ID, spot))
						}
						continue
					}
					// Yakın, yavaşla (bu tick hareket etme)
					continue
				}
			}

			// Karşı yönden gelen araçları kontrol et: ağırlıklı skor ile optimal karar ver
			// (Eski manuel wouldCollide bloğunun yerini alan calculateOptimalRetreat)
			if shouldRet, retSpot, retEvents := s.calculateOptimalRetreat(v); shouldRet {
				events = append(events, retEvents...)
				if retSpot >= 0 {
					// Geri çekilme kararı verildi ve geçerli bir spot bulundu
					v.Status = models.StatusRetreating
					v.WaitingAt = retSpot
					// Not: reserveSpot zaten calculateOptimalRetreat içinde çağrıldı
				} else {
					// Yer bulunamadı: aracı yerinde beklet, deadlock mekanizması devreye girecek
					v.Status = models.StatusWaiting
					v.WaitingAt = -1
				}
				continue
			} else if len(retEvents) > 0 {
				// shouldRetreat=false ama bilgi mesajı var (karşıdaki geri çekildi)
				events = append(events, retEvents...)
			}

			// Çarpışma riski hala devam ediyorsa (karşıdaki araç henüz çekilmediyse) hareket etme
			if s.wouldCollide(v, dt) {
				continue
			}

			// Normal hareket
			speedMs := v.Speed * 1000.0 / 3600.0 // m/s
			delta := speedMs * dt * 3600         // tick başına metre (dt=1.0s)

			if v.Direction == models.DirRight {
				v.Position += delta
				// Hedefe ulaştı mı?
				if v.Position >= float64(v.TargetDepo) {
					v.Position = float64(v.TargetDepo)
					// Depo dolu mu kontrol et
					depotFull := false
					for j := range s.state.Depots {
						if s.state.Depots[j].Position == v.TargetDepo {
							if s.countVehiclesInDepot(v.TargetDepo, v.ID) >= s.state.Depots[j].MaxCapacity {
								depotFull = true
							}
							break
						}
					}
					if depotFull {
						// Depo dolu, geri çekil
						v.Position -= delta
						spot := s.findNearestWaitingSpot(v)
						if spot >= 0 && s.reserveSpot(v.ID, spot) {
							v.Status = models.StatusRetreating
							v.WaitingAt = spot
							events = append(events, fmt.Sprintf("Araç #%d depo dolu! %dm'ye geri çekiliyor", v.ID, spot))
						}
					} else {
						v.Status = models.StatusLoading
						events = append(events, fmt.Sprintf("Araç #%d depoya ulaştı (%dm)", v.ID, v.TargetDepo))
					}
				}
			} else if v.Direction == models.DirLeft {
				v.Position -= delta
				// Başlangıca döndü mü?
				if v.Position <= 0 {
					v.Position = 0
					v.Status = models.StatusDone
					events = append(events, fmt.Sprintf("Araç #%d görevi tamamladı, başlangıca döndü", v.ID))
				}
			}

			// Yol sınırları
			v.Position = math.Max(0, math.Min(float64(cfg.RoadLength), v.Position))
		}
	}

	// Tamamlanan araçları temizle (opsiyonel: tutmak için yorum satırı)
	// s.removeCompletedVehicles()

	if len(events) > 0 {
		s.state.Events = events
	} else {
		s.state.Events = []string{}
	}
	s.syncSpotOccupancy()
}

// updateSensors sensör durumlarını günceller
func (s *Simulation) updateSensors() {
	cfg := s.state.Config
	for i := range s.state.Sensors {
		sensor := &s.state.Sensors[i]
		sensor.Red = false
		for _, v := range s.state.Vehicles {
			if v.Status == models.StatusDone {
				continue
			}

			// Depoda olan araçları sensör algılamasın
			if v.Status == models.StatusLoading || v.Status == models.StatusExiting {
				continue
			}

			// Depoda bekleyen araçları sensör algılamasın
			if v.Status == models.StatusWaiting {
				inDepot := false
				for _, d := range s.state.Depots {
					if math.Abs(v.Position-float64(d.Position)) < 1 {
						inDepot = true
						break
					}
				}
				if inDepot {
					continue
				}
			}

			dist := math.Abs(v.Position - float64(sensor.Position))
			if dist <= float64(cfg.SensorRange) {
				sensor.Red = true
				break
			}
		}
	}
}

// isVehicleInSafeSpot aracın cep veya depo içinde güvenli bekleyip beklemediğini döndürür.
func (s *Simulation) isVehicleInSafeSpot(v *models.Vehicle) bool {
	if v.Status == models.StatusWaiting && v.WaitingAt == 0 && math.Abs(v.Position) < 1 {
		return true
	}
	for _, d := range s.state.Depots {
		if math.Abs(v.Position-float64(d.Position)) < 1 {
			return true
		}
	}
	for _, p := range s.state.Pockets {
		if math.Abs(v.Position-float64(p.Position)) < 1 {
			return true
		}
	}
	return false
}

// isTrafficParticipant yalnızca tek şerit üzerinde kararları etkileyen araçları hesaba katar.
func (s *Simulation) isTrafficParticipant(v *models.Vehicle) bool {
	if v.Position < 0 {
		return false
	}
	switch v.Status {
	case models.StatusDone, models.StatusQueued, models.StatusLoading, models.StatusExiting:
		return false
	case models.StatusWaiting:
		return !s.isVehicleInSafeSpot(v)
	default:
		return true
	}
}

// blocksRoadEntry garajdan çıkan aracın başlangıç bölgesine güvenle girip giremeyeceğini belirler.
func (s *Simulation) blocksRoadEntry(v *models.Vehicle) bool {
	if v.Position < 0 || v.Status == models.StatusDone || v.Status == models.StatusQueued ||
		v.Status == models.StatusLoading || v.Status == models.StatusExiting {
		return false
	}
	return v.Position >= 0 && v.Position < 50
}

func (s *Simulation) hasNearbyRoadVehicle(v *models.Vehicle, minClearance float64) bool {
	for i := range s.state.Vehicles {
		other := &s.state.Vehicles[i]
		if other.ID == v.ID || !s.isTrafficParticipant(other) {
			continue
		}
		if math.Abs(v.Position-other.Position) < minClearance {
			return true
		}
	}
	return false
}

func (s *Simulation) countActiveVehicles() int {
	count := 0
	for _, v := range s.state.Vehicles {
		if v.Status != models.StatusDone && v.Status != models.StatusQueued {
			count++
		}
	}
	return count
}

// isPathToSpotClear ileri bir bekleme noktasına giderken karşı yöndeki araca temas edilip edilmeyeceğini kontrol eder.
func (s *Simulation) isPathToSpotClear(v *models.Vehicle, spot int) bool {
	cfg := s.state.Config
	target := float64(spot)
	minPos := math.Min(v.Position, target) - float64(cfg.MinDistance)
	maxPos := math.Max(v.Position, target) + float64(cfg.MinDistance)
	for i := range s.state.Vehicles {
		other := &s.state.Vehicles[i]
		if other.ID == v.ID || !s.isTrafficParticipant(other) {
			continue
		}
		if other.Position >= minPos && other.Position <= maxPos {
			return false
		}
	}
	return true
}

// hasOncomingVehicle karşı yönden gelen araç var mı kontrol eder
func (s *Simulation) hasOncomingVehicle(v *models.Vehicle) bool {
	for _, other := range s.state.Vehicles {
		if other.ID == v.ID || !s.isTrafficParticipant(&other) {
			continue
		}

		// Karşı yön mü?
		if other.Direction != v.Direction {
			// Yakında mı? (Karşıdan gelen araç varsa çıkma, mesafeyi 100m'ye çıkarıyoruz)
			dist := math.Abs(v.Position - other.Position)
			if dist < 120 {
				// Ancak araç bizden uzaklaşıyorsa (yani hedefimiz arkasındaysa) sorun yok
				// Bunu basitçe mesafeye göre yapıyoruz: 120m içinde bize doğru gelen araç varsa bekle.
				return true
			}
		}
	}
	return false
}

// wouldCollide çarpışma riski var mı kontrol eder
func (s *Simulation) wouldCollide(v *models.Vehicle, dt float64) bool {
	speedMs := v.Speed * 1000.0 / 3600.0
	delta := speedMs * dt * 3600
	nextPos := v.Position
	if v.Direction == models.DirRight {
		nextPos += delta
	} else {
		nextPos -= delta
	}

	return s.wouldCollideAt(v, nextPos)
}

func (s *Simulation) wouldCollideAt(v *models.Vehicle, nextPos float64) bool {
	cfg := s.state.Config
	for _, other := range s.state.Vehicles {
		if other.ID == v.ID || !s.isTrafficParticipant(&other) {
			continue
		}

		// Karşı yönden geliyor mu?
		if other.Direction != v.Direction {
			dist := math.Abs(nextPos - other.Position)
			otherDelta := other.Speed * 1000.0 / 3600.0
			if dist <= float64(cfg.MinDistance)+otherDelta {
				return true
			}
		}
		// Aynı yönde çok yakın mı?
		if other.Direction == v.Direction {
			isFront := false
			if v.Direction == models.DirRight {
				if other.Position > v.Position {
					isFront = true
				} else if other.Position == v.Position && other.ID < v.ID {
					isFront = true
				}
			} else {
				if other.Position < v.Position {
					isFront = true
				} else if other.Position == v.Position && other.ID < v.ID {
					isFront = true
				}
			}

			if isFront {
				dist := math.Abs(nextPos - other.Position)
				if dist <= float64(cfg.MinDistance) {
					return true
				}
			}
		}
	}
	return false
}

func (s *Simulation) wouldRetreatMoveCollide(v *models.Vehicle, nextPos float64) bool {
	cfg := s.state.Config
	for i := range s.state.Vehicles {
		other := &s.state.Vehicles[i]
		if other.ID == v.ID || !s.isTrafficParticipant(other) {
			continue
		}
		otherDelta := other.Speed * 1000.0 / 3600.0
		if math.Abs(nextPos-other.Position) <= float64(cfg.MinDistance)+otherDelta {
			return true
		}
	}
	return false
}

// findNearestWaitingSpot en yakın uygun cep/depo konumunu bulur (GERİDE KALAN)
func (s *Simulation) findNearestWaitingSpot(v *models.Vehicle) int {
	bestDist := math.MaxFloat64
	bestPos := -1

	// Başlangıca yakın sağa giden araçlar için 0m güvenli geri çekilme noktasıdır.
	if v.Direction == models.DirRight && v.Position > 0 && !s.isSpotReserved(0) && s.isPathToSpotClear(v, 0) {
		bestDist = v.Position
		bestPos = 0
	}

	// Cepleri kontrol et (sadece geride kalanlar)
	for _, p := range s.state.Pockets {
		if p.Occupied {
			continue
		}
		// Başka araç tarafından rezerve edilmiş mi?
		if s.isSpotReserved(p.Position) {
			continue
		}

		// Araç sağa gidiyorsa, cep solda olmalı (geride)
		// Araç sola gidiyorsa, cep sağda olmalı (geride)
		if v.Direction == models.DirRight && float64(p.Position) >= v.Position {
			continue // Bu cep ileride, atla
		}
		if v.Direction == models.DirLeft && float64(p.Position) <= v.Position {
			continue // Bu cep ileride, atla
		}
		if !s.isPathToSpotClear(v, p.Position) {
			continue
		}

		dist := math.Abs(v.Position - float64(p.Position))
		if dist < bestDist {
			bestDist = dist
			bestPos = p.Position
		}
	}

	// Depoları kontrol et (sadece geride kalanlar)
	for _, d := range s.state.Depots {
		if d.VehicleCount >= d.MaxCapacity {
			continue
		}
		// Başka araç tarafından rezerve edilmiş mi?
		if s.isSpotReserved(d.Position) {
			continue
		}

		// Araç sağa gidiyorsa, depo solda olmalı (geride)
		// Araç sola gidiyorsa, depo sağda olmalı (geride)
		if v.Direction == models.DirRight && float64(d.Position) >= v.Position {
			continue // Bu depo ileride, atla
		}
		if v.Direction == models.DirLeft && float64(d.Position) <= v.Position {
			continue // Bu depo ileride, atla
		}
		if !s.isPathToSpotClear(v, d.Position) {
			continue
		}

		dist := math.Abs(v.Position - float64(d.Position))
		if dist < bestDist {
			bestDist = dist
			bestPos = d.Position
		}
	}

	return bestPos
}

// occupyWaitingSpot cep/depo doluluğunu işaretle
func (s *Simulation) occupyWaitingSpot(v *models.Vehicle, pos int) {
	for i := range s.state.Pockets {
		if s.state.Pockets[i].Position == pos {
			s.state.Pockets[i].Occupied = true
			return
		}
	}
	for i := range s.state.Depots {
		if s.state.Depots[i].Position == pos {
			s.state.Depots[i].VehicleCount++
			return
		}
	}
}

// syncSpotOccupancy cep/depo doluluklarını araçların gerçek durumundan yeniden üretir.
func (s *Simulation) syncSpotOccupancy() {
	for i := range s.state.Pockets {
		s.state.Pockets[i].Occupied = false
	}
	for i := range s.state.Depots {
		s.state.Depots[i].VehicleCount = 0
	}

	for _, v := range s.state.Vehicles {
		if v.Status == models.StatusDone || v.Status == models.StatusQueued || v.Position < 0 {
			continue
		}

		if v.Status == models.StatusWaiting {
			for i := range s.state.Pockets {
				if math.Abs(v.Position-float64(s.state.Pockets[i].Position)) < 1 {
					s.state.Pockets[i].Occupied = true
					break
				}
			}
		}

		if v.Status == models.StatusWaiting || v.Status == models.StatusLoading || v.Status == models.StatusExiting {
			for i := range s.state.Depots {
				if math.Abs(v.Position-float64(s.state.Depots[i].Position)) < 1 {
					s.state.Depots[i].VehicleCount++
					break
				}
			}
		}
	}
}

func (s *Simulation) countVehiclesInDepot(depotPos int, excludeVehicleID int) int {
	count := 0
	for i := range s.state.Vehicles {
		v := &s.state.Vehicles[i]
		if v.ID == excludeVehicleID {
			continue
		}
		if v.Status != models.StatusWaiting && v.Status != models.StatusLoading && v.Status != models.StatusExiting {
			continue
		}
		if math.Abs(v.Position-float64(depotPos)) < 1 {
			count++
		}
	}
	return count
}

// releaseWaitingSpot cep/depo doluluğunu serbest bırak
func (s *Simulation) releaseWaitingSpot(v *models.Vehicle) {
	if v.WaitingAt < 0 {
		return
	}
	for i := range s.state.Pockets {
		if s.state.Pockets[i].Position == v.WaitingAt {
			s.state.Pockets[i].Occupied = false
			return
		}
	}
	for i := range s.state.Depots {
		if s.state.Depots[i].Position == v.WaitingAt {
			if s.state.Depots[i].VehicleCount > 0 {
				s.state.Depots[i].VehicleCount--
			}
			return
		}
	}
}

// getFrontVehicle aynı yönde giden ve önünde olan en yakın aracı bulur
func (s *Simulation) getFrontVehicle(v *models.Vehicle) *models.Vehicle {
	var frontVehicle *models.Vehicle
	minDist := math.MaxFloat64

	for i := range s.state.Vehicles {
		other := &s.state.Vehicles[i]

		if other.ID == v.ID || !s.isTrafficParticipant(other) {
			continue
		}

		// Aynı yön mü?
		if other.Direction != v.Direction {
			continue
		}

		// Önünde mi?
		var dist float64
		if v.Direction == models.DirRight {
			// Sağa gidiyorsa, diğer araç sağda olmalı
			if other.Position > v.Position {
				dist = other.Position - v.Position
			} else if other.Position == v.Position && other.ID < v.ID {
				dist = 0
			} else {
				continue
			}
		} else {
			// Sola gidiyorsa, diğer araç solda olmalı
			if other.Position < v.Position {
				dist = v.Position - other.Position
			} else if other.Position == v.Position && other.ID < v.ID {
				dist = 0
			} else {
				continue
			}
		}

		if dist < minDist {
			minDist = dist
			frontVehicle = other
		}
	}

	return frontVehicle
}

// =============================================================================
// OPTİMİZASYON & DEADLOCK ÇÖZME FONKSİYONLARI
// =============================================================================

// DEADLOCK EŞİĞİ: Bir araç bu kadar tick beklerse deadlock şüphesi başlar.
// 500ms tick aralığında 15 tick = 7.5 saniye bekleme.
const deadlockThreshold = 15

// incrementWaitCounters her tick başında çağrılır.
// Bekleyen ve geri çekilen araçların WaitTickCount sayacını artırır.
// Hareket eden araçların sayacını sıfırlar (deadlock çözüldükten sonra temizlenir).
func (s *Simulation) incrementWaitCounters() {
	for i := range s.state.Vehicles {
		v := &s.state.Vehicles[i]
		if v.Status == models.StatusWaiting || v.Status == models.StatusRetreating {
			v.WaitTickCount++
			// OptStats: toplam bekleme süresini biriktir
			s.state.OptStats.TotalWaitTime++
		} else {
			// Araç hareket etmeye başladıysa sayacı sıfırla
			v.WaitTickCount = 0
		}
	}
}

// detectDeadlock deadlock'taki araçların ID listesini döndürür.
// Kriter: Araç deadlockThreshold'dan fazla tick bekledi VE hala karşı yönden araç geliyor.
// Bu iki koşul birlikte sağlanıyorsa araç kendi başına çıkamıyor demektir.
func (s *Simulation) detectDeadlock() []int {
	var deadlocked []int
	for _, v := range s.state.Vehicles {
		// Sadece bekleyen veya geri çekilen araçları kontrol et
		if v.Status != models.StatusWaiting && v.Status != models.StatusRetreating {
			continue
		}
		// Eşiği aştı mı?
		if v.WaitTickCount <= deadlockThreshold {
			continue
		}
		// Hala karşı yönden araç geliyor mu? (sorun devam ediyor mu?)
		vCopy := v
		if s.hasOncomingVehicle(&vCopy) {
			deadlocked = append(deadlocked, v.ID)
		}
	}
	return deadlocked
}

// resolveDeadlock, tespit edilmiş deadlock araçlarını zorla çözer.
// Ağırlıklı skor hesabıyla hangi aracın geri çekileceğine karar verir.
// Normal findNearestWaitingSpot yerine findForwardWaitingSpot'u da dener (ileriye bakış).
func (s *Simulation) resolveDeadlock(vehicleIDs []int) []string {
	var events []string

	for _, id := range vehicleIDs {
		// Araç hala deadlock durumunda mı? (önceki iterasyonda çözülmüş olabilir)
		idx := -1
		for i, v := range s.state.Vehicles {
			if v.ID == id {
				idx = i
				break
			}
		}
		if idx < 0 {
			continue
		}
		v := &s.state.Vehicles[idx]

		// Önce geride spot ara (normal yön)
		spot := s.findNearestWaitingSpot(v)

		// Geride spot bulunamadıysa ileride ara (deadlock'a özgü çözüm)
		if spot < 0 {
			spot = s.findForwardWaitingSpot(v)
		}

		if spot < 0 {
			// Hiçbir yerde yer yok → bu tick çözülemiyor, bir sonraki tick'e bırak
			events = append(events,
				fmt.Sprintf("Araç #%d deadlock! Boş spot bulunamadı, bekleniyor...", v.ID))
			continue
		}

		// Zorunlu geri çekme: WaitTickCount'u sıfırla, spot rezerve et
		ok := s.reserveSpot(v.ID, spot)
		if !ok {
			// Spot başka araç tarafından az önce rezerve edildi
			events = append(events,
				fmt.Sprintf("Araç #%d deadlock çözümü: spot %dm zaten rezerve, yeniden deneniyor", v.ID, spot))
			continue
		}

		v.Status = models.StatusRetreating
		v.WaitingAt = spot
		v.WaitTickCount = 0 // Sayacı sıfırla

		// İstatistikleri güncelle
		s.state.OptStats.DeadlockResolved++
		s.state.DeadlockCount++

		events = append(events,
			fmt.Sprintf("⚠️ DEADLOCK çözüldü: Araç #%d zorla %dm'ye yönlendirildi", v.ID, spot))
	}
	return events
}

// findForwardWaitingSpot, aracın hareket yönündeki (ilerideki) en yakın boş cep/depoyu bulur.
// Yalnızca deadlock çözümünde kullanılır: normal durumda araç ileri değil geri çekilir.
// Mantık: Karşı araç geçebilsin diye bu araç bir adım ileri çekilir.
func (s *Simulation) findForwardWaitingSpot(v *models.Vehicle) int {
	bestDist := math.MaxFloat64
	bestPos := -1

	// Cepleri kontrol et (sadece ileridekiler)
	for _, p := range s.state.Pockets {
		// Zaten dolu veya başka araç tarafından rezerve edilmiş mi?
		if p.Occupied {
			continue
		}
		if s.isSpotReserved(p.Position) {
			continue
		}

		// Araç sağa gidiyorsa, cep sağda (ileride) olmalı
		// Araç sola gidiyorsa, cep solda (ileride) olmalı
		if v.Direction == models.DirRight && float64(p.Position) <= v.Position {
			continue // Bu cep geride, atla
		}
		if v.Direction == models.DirLeft && float64(p.Position) >= v.Position {
			continue // Bu cep geride, atla
		}
		if !s.isPathToSpotClear(v, p.Position) {
			continue
		}

		dist := math.Abs(v.Position - float64(p.Position))
		if dist < bestDist {
			bestDist = dist
			bestPos = p.Position
		}
	}

	// Depoları kontrol et (sadece ileridekiler)
	for _, d := range s.state.Depots {
		if d.VehicleCount >= d.MaxCapacity {
			continue
		}
		if s.isSpotReserved(d.Position) {
			continue
		}

		if v.Direction == models.DirRight && float64(d.Position) <= v.Position {
			continue
		}
		if v.Direction == models.DirLeft && float64(d.Position) >= v.Position {
			continue
		}
		if !s.isPathToSpotClear(v, d.Position) {
			continue
		}

		dist := math.Abs(v.Position - float64(d.Position))
		if dist < bestDist {
			bestDist = dist
			bestPos = d.Position
		}
	}

	return bestPos
}

// isSpotReserved, verilen pozisyonun herhangi bir araç tarafından rezerve edilip
// edilmediğini kontrol eder. Çift rezervasyonu önlemek için kullanılır.
func (s *Simulation) isSpotReserved(pos int) bool {
	for _, v := range s.state.Vehicles {
		if v.AssignedSpot == pos {
			return true
		}
	}
	return false
}

// reserveSpot, belirtilen aracı (vehicleID) belirtilen spot'a rezerve eder.
// Aynı spot başka bir araç tarafından zaten rezerve edildiyse false döner.
// Bu fonksiyon retreating kararı verildiği anda çağrılmalıdır (fiziksel işgalden önce).
func (s *Simulation) reserveSpot(vehicleID int, pos int) bool {
	// Spot zaten başka araç tarafından rezerve mi?
	for _, v := range s.state.Vehicles {
		if v.ID != vehicleID && v.AssignedSpot == pos {
			return false // Çakışma var, rezervasyon başarısız
		}
	}
	// Rezervasyonu yap
	for i := range s.state.Vehicles {
		if s.state.Vehicles[i].ID == vehicleID {
			s.state.Vehicles[i].AssignedSpot = pos
			return true
		}
	}
	return false
}

// releaseReservation, bir aracın spot rezervasyonunu iptal eder.
// Araç cepe/depoya fiilen ulaştığında veya durumu değiştiğinde çağrılır.
func (s *Simulation) releaseReservation(vehicleID int) {
	for i := range s.state.Vehicles {
		if s.state.Vehicles[i].ID == vehicleID {
			s.state.Vehicles[i].AssignedSpot = -1
			return
		}
	}
}

// vehicleScore, iki araç arasında kimin geri çekileceğini belirlemek için
// ağırlıklı bir öncelik skoru hesaplar.
// Yüksek skor = Bu araç devam etmeli (geri çekilmemeli).
//
// Skor bileşenleri:
//   - Tamamlanan yol oranı: Hedefe ne kadar yaklaştı? (0.0 - 1.0)
//   - Yük bonusu: Yüklü araç geri dönüş yapıyor, görevi yarıda bırakmamalı (+50.0)
//   - Öncelik bonusu: Priority=true araç her zaman önce geçer (+100.0)
func vehicleScore(v *models.Vehicle) float64 {
	var score float64

	// Tamamlanan yol oranı: Hedef depoya ne kadar yaklaştı?
	// DirRight: position/targetDepo → 0.0 (başlangıç) ile 1.0 (depo) arası
	// DirLeft: yüklü araç dönüyor → (targetDepo - position) / targetDepo
	if v.TargetDepo > 0 {
		if v.Direction == models.DirRight {
			score = v.Position / float64(v.TargetDepo)
		} else {
			// Sola gidiyorsa: ne kadar geri döndü?
			score = (float64(v.TargetDepo) - v.Position) / float64(v.TargetDepo)
		}
		score = math.Max(0, math.Min(1, score))
	}

	// Yük bonusu: Yüklü araç mission-critical, geri dönmeli
	if v.HasLoad {
		score += 50.0
	}

	// Öncelik bonusu: Öncelikli araç asla geri çekilmez (çok yüksek bonus)
	if v.Priority {
		score += 100.0
	}

	return score
}

// calculateOptimalRetreat, iki karşı yön araç karşılaştığında kimin geri çekileceğine
// ağırlıklı skor ile karar verir ve retreating işlemini başlatır.
//
// Döndürür:
//   - shouldRetreat: Bu araç (v) geri çekilmeli mi?
//   - spot:          Geri çekileceği konum (-1 = yer bulunamadı)
//   - events:        Olay açıklamaları (loglama için)
//
// Eski wouldCollide bloğunun yerini alır.
func (s *Simulation) calculateOptimalRetreat(v *models.Vehicle) (shouldRetreat bool, spot int, events []string) {
	cfg := s.state.Config
	dt := 1.0 / 3600.0

	// Çarpışma riski yok → geri çekilmeye gerek yok
	if !s.wouldCollide(v, dt) {
		return false, -1, nil
	}

	// Bu araç için en yakın uygun spot (geride)
	mySpot := s.findNearestWaitingSpot(v)

	// Bu aracın öncelik skoru
	myScore := vehicleScore(v)

	// Karşı yöndeki araçları tara, en yakın olanla karşılaştır
	for _, other := range s.state.Vehicles {
		if other.ID == v.ID || !s.isTrafficParticipant(&other) {
			continue
		}
		// Karşı yönde ve yeterince yakın mı?
		if other.Direction == v.Direction || other.Status != models.StatusMoving {
			continue
		}
		dist := math.Abs(v.Position - other.Position)
		if dist >= float64(cfg.SensorRange)*2+float64(cfg.MinDistance) {
			// Çok uzakta, henüz ilgisi yok
			continue
		}

		otherCopy := other
		otherScore := vehicleScore(&otherCopy)

		// Karar: Yüksek skorlu araç devam eder, düşük skorlu geri çekilir.
		// Eşit skorda deterministik davranış için küçük ID geri çekilir.
		iShouldRetreat := false
		if myScore < otherScore {
			iShouldRetreat = true
		} else if myScore == otherScore && v.ID < other.ID {
			iShouldRetreat = true
		}

		if !iShouldRetreat {
			// Bu araç devam ediyor, karşıdaki geri çekilsin
			return false, -1, nil
		}

		// Bu araç geri çekilmeli: spot bul ve rezerve et
		// Önce geride ara, bulamazsa ileride ara
		if mySpot < 0 || s.isSpotReserved(mySpot) {
			// Geride uygun spot yok → ileride ara (deadlock'a girmeden çöz)
			fwdSpot := s.findForwardWaitingSpot(v)
			if fwdSpot >= 0 {
				mySpot = fwdSpot
			} else {
				// Hiçbir yerde yer yok → araç dursun, deadlock mekanizması devreye girecek
				events = append(events,
					fmt.Sprintf("Araç #%d geri çekilmek istiyor ama boş spot yok", v.ID))
				return true, -1, events
			}
		}

		ok := s.reserveSpot(v.ID, mySpot)
		if !ok {
			// Spot az önce başka araç tarafından alındı → alternatif ara
			altSpot := s.findNearestWaitingSpot(v)
			if altSpot >= 0 {
				s.reserveSpot(v.ID, altSpot)
				mySpot = altSpot
			} else {
				events = append(events,
					fmt.Sprintf("Araç #%d için tüm spotlar rezerve, deadlock olabilir", v.ID))
				return true, -1, events
			}
		}

		// OptStats: kaçınılan çarpışma sayısını artır
		s.state.OptStats.CollisionAvoided++

		events = append(events,
			fmt.Sprintf("Araç #%d (skor=%.2f) < Araç #%d (skor=%.2f) → %dm'ye geri çekiliyor",
				v.ID, myScore, other.ID, otherScore, mySpot))
		return true, mySpot, events
	}

	// Karşıdan hareket eden araç bulunamadı ama wouldCollide true dedi →
	// Güvenli olmak için eski basit davranışa dön
	if mySpot >= 0 && !s.isSpotReserved(mySpot) {
		s.reserveSpot(v.ID, mySpot)
		s.state.OptStats.CollisionAvoided++
		events = append(events,
			fmt.Sprintf("Araç #%d çarpışma riski, %dm'ye ihtiyaten geri çekiliyor", v.ID, mySpot))
		return true, mySpot, events
	}

	return false, -1, nil
}
