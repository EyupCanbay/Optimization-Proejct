package simulation

import (
	"fmt"
	"math"
	"otonom-arac-sim/models"
	"sync"
)

// Simulation simülasyon motorunu temsil eder
type Simulation struct {
	mu      sync.Mutex
	state   models.SimState
	nextID  int
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
			Position:    pos,
			VehicleCount: 0,
			MaxCapacity: cfg.MaxDepotCap,
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
		ID:         id,
		Position:   -1, // Garajda (yol dışı)
		Direction:  models.DirRight,
		Speed:      18, // Varsayılan hız: 18 km/h
		Status:     models.StatusQueued, // Kuyrukta bekliyor
		TargetDepo: targetDepo,
		HasLoad:    false,
		Priority:   priority,
		WaitingAt:  -1,
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
			Position:    pos,
			VehicleCount: 0,
			MaxCapacity: cfg.MaxDepotCap,
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

	// Sensörleri güncelle
	s.updateSensors()

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
			for _, other := range s.state.Vehicles {
				if other.ID == v.ID || other.Status == models.StatusDone {
					continue
				}
				// Diğer araç daha önce mi eklendi VE hala kuyrukta mı?
				if other.ID < v.ID && other.Status == models.StatusQueued {
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
			// Depo sayacını azalt
			for j := range s.state.Depots {
				if s.state.Depots[j].Position == int(v.Position) {
					s.state.Depots[j].VehicleCount--
					break
				}
			}
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
			
			// Karşı yönden araç var mı?
			if s.hasOncomingVehicle(v) {
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
				v.Position += delta
				if v.Position >= targetPos {
					v.Position = targetPos
					v.Status = models.StatusWaiting
					s.occupyWaitingSpot(v, v.WaitingAt)
					events = append(events, fmt.Sprintf("Araç #%d %dm'de beklemeye başladı", v.ID, v.WaitingAt))
				}
			} else {
				v.Position -= delta
				if v.Position <= targetPos {
					v.Position = targetPos
					v.Status = models.StatusWaiting
					s.occupyWaitingSpot(v, v.WaitingAt)
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
						if spot >= 0 {
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
			
			// Karşı yönden gelen araçları kontrol et
			if s.wouldCollide(v, dt) {
				// Cep veya depo bul
				spot := s.findNearestWaitingSpot(v)
				if spot >= 0 {
					v.Status = models.StatusRetreating
					v.WaitingAt = spot
					events = append(events, fmt.Sprintf("Araç #%d karşıdan araç geliyor! %dm'ye geri çekiliyor", v.ID, spot))
				}
				continue
			}

			// Normal hareket
			speedMs := v.Speed * 1000.0 / 3600.0 // m/s
			delta := speedMs * dt * 3600          // tick başına metre (dt=1.0s)

			if v.Direction == models.DirRight {
				v.Position += delta
				// Hedefe ulaştı mı?
				if v.Position >= float64(v.TargetDepo) {
					v.Position = float64(v.TargetDepo)
					// Depo dolu mu kontrol et
					depotFull := false
					for j := range s.state.Depots {
						if s.state.Depots[j].Position == v.TargetDepo {
							if s.state.Depots[j].VehicleCount >= s.state.Depots[j].MaxCapacity {
								depotFull = true
							} else {
								s.state.Depots[j].VehicleCount++
							}
							break
						}
					}
					if depotFull {
						// Depo dolu, geri çekil
						v.Position -= delta
						spot := s.findNearestWaitingSpot(v)
						if spot >= 0 {
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

// hasOncomingVehicle karşı yönden gelen araç var mı kontrol eder
func (s *Simulation) hasOncomingVehicle(v *models.Vehicle) bool {
	for _, other := range s.state.Vehicles {
		if other.ID == v.ID || other.Status == models.StatusDone {
			continue
		}
		
		// Depoda olan araçları yok say
		if other.Status == models.StatusLoading || other.Status == models.StatusExiting {
			continue
		}
		
		// Depoda bekleyen araçları yok say
		if other.Status == models.StatusWaiting {
			inDepot := false
			for _, d := range s.state.Depots {
				if math.Abs(other.Position-float64(d.Position)) < 1 {
					inDepot = true
					break
				}
			}
			if inDepot {
				continue
			}
		}
		
		// Karşı yön mü?
		if other.Direction != v.Direction {
			// Yakında mı? (50m içinde)
			dist := math.Abs(v.Position - other.Position)
			if dist < 50 {
				return true
			}
		}
	}
	return false
}

// wouldCollide çarpışma riski var mı kontrol eder
func (s *Simulation) wouldCollide(v *models.Vehicle, dt float64) bool {
	cfg := s.state.Config
	speedMs := v.Speed * 1000.0 / 3600.0
	delta := speedMs * dt * 3600
	nextPos := v.Position
	if v.Direction == models.DirRight {
		nextPos += delta
	} else {
		nextPos -= delta
	}

	for _, other := range s.state.Vehicles {
		if other.ID == v.ID || other.Status == models.StatusDone {
			continue
		}
		
		// Depoda olan araçları yok say
		if other.Status == models.StatusLoading || other.Status == models.StatusExiting {
			continue
		}
		
		// Depoda bekleyen araçları yok say
		if other.Status == models.StatusWaiting {
			inDepot := false
			for _, d := range s.state.Depots {
				if math.Abs(other.Position-float64(d.Position)) < 1 {
					inDepot = true
					break
				}
			}
			if inDepot {
				continue
			}
		}
		
		// Karşı yönden geliyor mu?
		if other.Direction != v.Direction {
			dist := math.Abs(nextPos - other.Position)
			if dist < float64(cfg.MinDistance) {
				return true
			}
		}
		// Aynı yönde çok yakın mı?
		if other.Direction == v.Direction {
			dist := math.Abs(nextPos - other.Position)
			if dist < float64(cfg.MinDistance) && dist > 0 {
				return true
			}
		}
	}
	return false
}

// findNearestWaitingSpot en yakın uygun cep/depo konumunu bulur (GERİDE KALAN)
func (s *Simulation) findNearestWaitingSpot(v *models.Vehicle) int {
	bestDist := math.MaxFloat64
	bestPos := -1

	// Cepleri kontrol et (sadece geride kalanlar)
	for _, p := range s.state.Pockets {
		if p.Occupied {
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
		
		// Araç sağa gidiyorsa, depo solda olmalı (geride)
		// Araç sola gidiyorsa, depo sağda olmalı (geride)
		if v.Direction == models.DirRight && float64(d.Position) >= v.Position {
			continue // Bu depo ileride, atla
		}
		if v.Direction == models.DirLeft && float64(d.Position) <= v.Position {
			continue // Bu depo ileride, atla
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
		
		if other.ID == v.ID || other.Status == models.StatusDone {
			continue
		}
		
		// Depoda olan araçları yok say
		if other.Status == models.StatusLoading || other.Status == models.StatusExiting {
			continue
		}
		
		// Depoda bekleyen araçları yok say
		if other.Status == models.StatusWaiting {
			inDepot := false
			for _, d := range s.state.Depots {
				if math.Abs(other.Position-float64(d.Position)) < 1 {
					inDepot = true
					break
				}
			}
			if inDepot {
				continue
			}
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
			} else {
				continue
			}
		} else {
			// Sola gidiyorsa, diğer araç solda olmalı
			if other.Position < v.Position {
				dist = v.Position - other.Position
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
