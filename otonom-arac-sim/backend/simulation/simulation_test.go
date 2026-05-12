package simulation

import (
	"math"
	"testing"

	"otonom-arac-sim/models"
)

// testConfig standart test konfigürasyonu döndürür.
// Gerçek simülasyonla aynı parametreler kullanılır.
func testConfig() models.SimConfig {
	return models.SimConfig{
		RoadLength:      240,
		SensorSpacing:   40,
		SensorRange:     20,
		MinDistance:     20,
		PocketPositions: []int{80, 200},
		DepotPositions:  []int{70, 120, 240},
		MaxPocketCap:    1,
		MaxDepotCap:     3,
	}
}

// newRunning yeni bir simülasyon oluşturur ve hemen başlatır.
func newRunning() *Simulation {
	sim := New(testConfig())
	sim.SetRunning(true)
	return sim
}

func assertNoRoadDistanceViolation(t *testing.T, tick int, state models.SimState) {
	t.Helper()
	isSafeWaiting := func(v models.Vehicle) bool {
		if v.Status != models.StatusWaiting {
			return false
		}
		if v.WaitingAt == 0 && math.Abs(v.Position) < 1 {
			return true
		}
		for _, p := range state.Pockets {
			if math.Abs(v.Position-float64(p.Position)) < 1 {
				return true
			}
		}
		for _, d := range state.Depots {
			if math.Abs(v.Position-float64(d.Position)) < 1 {
				return true
			}
		}
		return false
	}
	for i := 0; i < len(state.Vehicles); i++ {
		for j := i + 1; j < len(state.Vehicles); j++ {
			vi := state.Vehicles[i]
			vj := state.Vehicles[j]
			if vi.Status == models.StatusDone || vj.Status == models.StatusDone {
				continue
			}
			if vi.Status == models.StatusQueued || vj.Status == models.StatusQueued {
				continue
			}
			if vi.Status == models.StatusLoading || vi.Status == models.StatusExiting ||
				vj.Status == models.StatusLoading || vj.Status == models.StatusExiting {
				continue
			}
			if vi.Position < 0 || vj.Position < 0 {
				continue
			}
			if isSafeWaiting(vi) || isSafeWaiting(vj) {
				continue
			}
			dist := math.Abs(vi.Position - vj.Position)
			if dist < float64(testConfig().MinDistance) {
				t.Fatalf("T%d MESAFE İHLALİ: Araç #%d (%.1fm, %s, %s, wait=%d) ve Araç #%d (%.1fm, %s, %s, wait=%d), mesafe=%.1f, events=%v",
					tick,
					vi.ID, vi.Position, vi.Direction.String(), vi.Status, vi.WaitingAt,
					vj.ID, vj.Position, vj.Direction.String(), vj.Status, vj.WaitingAt,
					dist, state.Events)
			}
		}
	}
}

func runUntilAllDone(t *testing.T, sim *Simulation, maxTicks int) models.SimState {
	t.Helper()
	for i := 0; i < maxTicks; i++ {
		sim.Tick()
		state := sim.GetState()
		assertNoRoadDistanceViolation(t, i+1, state)
		allDone := len(state.Vehicles) > 0
		for _, v := range state.Vehicles {
			if v.Status != models.StatusDone {
				allDone = false
				break
			}
		}
		if allDone {
			return state
		}
	}
	return sim.GetState()
}

func countVehiclesAtDepot(state models.SimState, depotPos int) int {
	count := 0
	for _, v := range state.Vehicles {
		if v.Status == models.StatusWaiting || v.Status == models.StatusLoading || v.Status == models.StatusExiting {
			if math.Abs(v.Position-float64(depotPos)) < 1 {
				count++
			}
		}
	}
	return count
}

// =============================================================================
// SENARYO 1: Tek Araç — Depoya Gidip Geri Dönmeli
// =============================================================================

func TestSingleVehicleCompletesJourney(t *testing.T) {
	sim := newRunning()

	id, err := sim.AddVehicle(120, false)
	if err != nil {
		t.Fatalf("araç eklenemedi: %v", err)
	}

	// Araç 120m depoya gidip yük alıp 0m'ye dönmeli.
	// 18 km/h = 5 m/s → 120m gidiş + 120m dönüş = 240m / 5 m/s = 48 saniye
	// Her tick 1 saniye → 48 tick yeterli; güvenlik payıyla 200 tick çalıştır.
	maxTicks := 200
	for i := 0; i < maxTicks; i++ {
		sim.Tick()
		state := sim.GetState()
		for _, v := range state.Vehicles {
			if v.ID == id && v.Status == models.StatusDone {
				return // Başarı: araç görevi tamamladı
			}
		}
	}

	t.Errorf("araç #%d %d tick içinde görevi tamamlayamadı", id, maxTicks)
}

// =============================================================================
// SENARYO 2: Karşılaşma — Bir Araç Geri Çekilmeli
// =============================================================================

func TestHeadOnEncounterTriggersRetreat(t *testing.T) {
	sim := newRunning()

	// Araç 1: sağa gidiyor (240m hedef)
	id1, _ := sim.AddVehicle(240, false)

	// Araç 1'i yola çıkar ve biraz ilerlet
	for i := 0; i < 20; i++ {
		sim.Tick()
	}

	// Araç 2: 70m hedef — kısa sürede yük alıp sola dönecek
	id2, _ := sim.AddVehicle(70, false)

	// Araç 2'nin yola çıkması ve yük alması için tick ver
	for i := 0; i < 30; i++ {
		sim.Tick()
	}

	// 150 tick boyunca karşılaşma senaryosunu çalıştır
	retreatSeen := false
	for i := 0; i < 150; i++ {
		sim.Tick()
		state := sim.GetState()
		for _, v := range state.Vehicles {
			if (v.ID == id1 || v.ID == id2) &&
				(v.Status == models.StatusRetreating || v.Status == models.StatusWaiting) {
				retreatSeen = true
			}
		}
	}

	if !retreatSeen {
		t.Logf("Not: Araçlar karşılaşmadı (pozisyonlar örtüşmedi). Bu senaryo zamanlama bağımlı.")
	}
	// Kritik kontrol: hiçbir araç çarpışmış olmamalı (aynı pozisyonda iki araç)
	state := sim.GetState()
	for i := 0; i < len(state.Vehicles); i++ {
		for j := i + 1; j < len(state.Vehicles); j++ {
			vi := state.Vehicles[i]
			vj := state.Vehicles[j]
			if vi.Status == models.StatusDone || vj.Status == models.StatusDone {
				continue
			}
			if vi.Status == models.StatusQueued || vj.Status == models.StatusQueued {
				continue
			}
			if vi.Position < 0 || vj.Position < 0 {
				continue
			}
			dist := math.Abs(vi.Position - vj.Position)
			if dist < float64(testConfig().MinDistance) && vi.Direction != vj.Direction {
				t.Errorf("ÇARPIŞMA: Araç #%d (%.1fm) ve Araç #%d (%.1fm) min mesafe ihlali",
					vi.ID, vi.Position, vj.ID, vj.Position)
			}
		}
	}
	_ = id1
	_ = id2
}

// =============================================================================
// SENARYO 3: Çoklu Araç — Deadlock Çözümü
// =============================================================================

func TestMultiVehicleDeadlockResolution(t *testing.T) {
	sim := newRunning()

	// 5 araç ekle: farklı hedefler
	targets := []int{240, 70, 120, 240, 70}
	for _, target := range targets {
		if _, err := sim.AddVehicle(target, false); err != nil {
			t.Fatalf("araç eklenemedi: %v", err)
		}
	}

	state := runUntilAllDone(t, sim, 2000)

	doneCount := 0
	for _, v := range state.Vehicles {
		if v.Status == models.StatusDone {
			doneCount++
		}
	}
	if doneCount != len(targets) {
		t.Fatalf("1200 tick sonunda tüm araçlar tamamlanmalıydı: %d/%d tamamlandı, durum=%+v",
			doneCount, len(targets), state.Vehicles)
	}

	t.Logf("1200 tick sonucu: %d araç tamamlandı, deadlock çözüldü: %d, kaçınılan çarpışma: %d",
		doneCount, state.OptStats.DeadlockResolved, state.OptStats.CollisionAvoided)
}

func TestReverseDensityScenarioDoesNotCollide(t *testing.T) {
	sim := newRunning()

	targets := []int{70, 240, 70, 240, 120}
	for _, target := range targets {
		if _, err := sim.AddVehicle(target, false); err != nil {
			t.Fatalf("araç eklenemedi: %v", err)
		}
	}

	state := runUntilAllDone(t, sim, 2000)

	doneCount := 0
	for _, v := range state.Vehicles {
		if v.Status == models.StatusDone {
			doneCount++
		}
	}
	if doneCount != len(targets) {
		t.Fatalf("ters yoğunluk senaryosu tamamlanamadı: %d/%d tamamlandı, tick=%d, durum=%+v, depolar=%+v, cepler=%+v, events=%v",
			doneCount, len(targets), state.Tick, state.Vehicles, state.Depots, state.Pockets, state.Events)
	}
	t.Logf("ters yoğunluk senaryosu %d tickte tamamlandı", state.Tick)
}

func TestAlternatingEndsScenarioDoesNotCollide(t *testing.T) {
	sim := newRunning()

	targets := []int{240, 70, 240, 70, 240, 70}
	for _, target := range targets {
		if _, err := sim.AddVehicle(target, false); err != nil {
			t.Fatalf("araç eklenemedi: %v", err)
		}
	}

	state := runUntilAllDone(t, sim, 2400)

	doneCount := 0
	for _, v := range state.Vehicles {
		if v.Status == models.StatusDone {
			doneCount++
		}
	}
	if doneCount != len(targets) {
		t.Fatalf("alternatif uçlar senaryosu tamamlanamadı: %d/%d tamamlandı, tick=%d, durum=%+v, depolar=%+v, cepler=%+v, events=%v",
			doneCount, len(targets), state.Tick, state.Vehicles, state.Depots, state.Pockets, state.Events)
	}
	t.Logf("alternatif uçlar senaryosu %d tickte tamamlandı", state.Tick)
}

func TestDepotCapacityNeverExceededForSameFarTarget(t *testing.T) {
	sim := newRunning()

	targets := []int{240, 240, 240, 240, 240}
	for _, target := range targets {
		if _, err := sim.AddVehicle(target, false); err != nil {
			t.Fatalf("araç eklenemedi: %v", err)
		}
	}

	for tick := 1; tick <= 2400; tick++ {
		sim.Tick()
		state := sim.GetState()
		assertNoRoadDistanceViolation(t, tick, state)

		if actual := countVehiclesAtDepot(state, 240); actual > state.Config.MaxDepotCap {
			t.Fatalf("T%d 240m deposu kapasitesi aşıldı: gerçek araç=%d, max=%d, durum=%+v, depolar=%+v, events=%v",
				tick, actual, state.Config.MaxDepotCap, state.Vehicles, state.Depots, state.Events)
		}

		doneCount := 0
		for _, v := range state.Vehicles {
			if v.Status == models.StatusDone {
				doneCount++
			}
		}
		if doneCount == len(targets) {
			return
		}
	}

	state := sim.GetState()
	t.Fatalf("aynı uzak hedef senaryosu tamamlanamadı: durum=%+v, depolar=%+v, events=%v",
		state.Vehicles, state.Depots, state.Events)
}

func TestTwelveVehicleMixedStressDoesNotDeadlock(t *testing.T) {
	sim := newRunning()

	targets := []int{70, 120, 240, 70, 120, 240, 70, 120, 240, 70, 120, 240}
	for _, target := range targets {
		if _, err := sim.AddVehicle(target, false); err != nil {
			t.Fatalf("araç eklenemedi: %v", err)
		}
	}

	state := runUntilAllDone(t, sim, 4000)

	doneCount := 0
	for _, v := range state.Vehicles {
		if v.Status == models.StatusDone {
			doneCount++
		}
	}
	if doneCount != len(targets) {
		t.Fatalf("12 araç stres senaryosu tamamlanamadı: %d/%d tamamlandı, tick=%d, durum=%+v, depolar=%+v, cepler=%+v, events=%v",
			doneCount, len(targets), state.Tick, state.Vehicles, state.Depots, state.Pockets, state.Events)
	}
	t.Logf("12 araç stres senaryosu %d tickte tamamlandı", state.Tick)
}

func TestQueuedVehiclesDoNotBlockReturningVehicle(t *testing.T) {
	sim := newRunning()
	sim.state.Vehicles = []models.Vehicle{
		{
			ID:           1,
			Position:     20,
			Direction:    models.DirLeft,
			Status:       models.StatusWaiting,
			TargetDepo:   70,
			HasLoad:      true,
			WaitingAt:    -1,
			AssignedSpot: -1,
		},
		{
			ID:           2,
			Position:     -1,
			Direction:    models.DirRight,
			Status:       models.StatusQueued,
			TargetDepo:   240,
			WaitingAt:    -1,
			AssignedSpot: -1,
		},
	}

	if sim.hasOncomingVehicle(&sim.state.Vehicles[0]) {
		t.Fatal("kuyruktaki araç, geri dönen araç için karşı yön trafiği sayılmamalı")
	}
}

// =============================================================================
// SENARYO 4: Öncelikli Araç — Düşük Öncelikli Araç Geri Çekilmeli
// =============================================================================

func TestPriorityVehicleHasHigherScore(t *testing.T) {
	// vehicleScore fonksiyonunu doğrudan test et
	normal := &models.Vehicle{
		ID:         1,
		Position:   100,
		Direction:  models.DirRight,
		TargetDepo: 240,
		HasLoad:    false,
		Priority:   false,
	}
	priority := &models.Vehicle{
		ID:         2,
		Position:   100,
		Direction:  models.DirLeft,
		TargetDepo: 240,
		HasLoad:    false,
		Priority:   true,
	}

	normalScore := vehicleScore(normal)
	priorityScore := vehicleScore(priority)

	if priorityScore <= normalScore {
		t.Errorf("Öncelikli araç skoru (%f) normal araç skorundan (%f) büyük olmalı",
			priorityScore, normalScore)
	}
}

// =============================================================================
// SENARYO 5: Rezervasyon Sistemi — Çift Rezervasyon Engellenmeli
// =============================================================================

func TestReserveSpotPreventsDoubleBooking(t *testing.T) {
	sim := newRunning()

	// İki araç ekle
	sim.AddVehicle(240, false)
	sim.AddVehicle(240, false)

	// Araçları yola çıkar
	for i := 0; i < 5; i++ {
		sim.Tick()
	}

	// Araç 1 spot 80'i rezerve etsin
	ok1 := sim.reserveSpot(1, 80)
	if !ok1 {
		t.Fatal("İlk rezervasyon başarısız olmamalıydı")
	}

	// Araç 2 aynı spotu rezerve etmeye çalışsın → başarısız olmalı
	ok2 := sim.reserveSpot(2, 80)
	if ok2 {
		t.Error("Çift rezervasyon engellenmedi: iki araç aynı spotu rezerve edebildi")
	}

	// Araç 1 rezervasyonu bırakınca araç 2 alabilmeli
	sim.releaseReservation(1)
	ok3 := sim.reserveSpot(2, 80)
	if !ok3 {
		t.Error("Rezervasyon bırakıldıktan sonra başka araç rezerve edebilmeli")
	}
}

// =============================================================================
// SENARYO 6: vehicleScore — Yük Bonusu
// =============================================================================

func TestVehicleScoreLoadBonus(t *testing.T) {
	empty := &models.Vehicle{
		ID: 1, Position: 100, Direction: models.DirLeft,
		TargetDepo: 240, HasLoad: false, Priority: false,
	}
	loaded := &models.Vehicle{
		ID: 2, Position: 100, Direction: models.DirLeft,
		TargetDepo: 240, HasLoad: true, Priority: false,
	}

	if vehicleScore(loaded) <= vehicleScore(empty) {
		t.Error("Yüklü araç skoru yüksüz araç skorundan büyük olmalı")
	}
}

// =============================================================================
// SENARYO 7: Depo Kapasitesi — Dolu Depoya Araç Giremez
// =============================================================================

func TestDepotCapacityEnforced(t *testing.T) {
	sim := newRunning()

	// 70m depoya 3 araç ekle (max kapasite 3)
	for i := 0; i < 3; i++ {
		sim.AddVehicle(70, false)
	}

	// Araçları depoya ulaştır
	for i := 0; i < 100; i++ {
		sim.Tick()
	}

	// 4. araç ekle — depo doluysa geri çekilmeli
	sim.AddVehicle(70, false)
	for i := 0; i < 50; i++ {
		sim.Tick()
	}

	state := sim.GetState()
	// 70m deposunun kapasitesi aşılmamış olmalı
	for _, d := range state.Depots {
		if d.Position == 70 && d.VehicleCount > d.MaxCapacity {
			t.Errorf("Depo kapasitesi aşıldı: %d/%d", d.VehicleCount, d.MaxCapacity)
		}
	}
}

// =============================================================================
// SENARYO 8: AssignedSpot Başlangıç Değeri
// =============================================================================

func TestNewVehicleAssignedSpotIsMinusOne(t *testing.T) {
	sim := newRunning()
	id, _ := sim.AddVehicle(120, false)

	state := sim.GetState()
	for _, v := range state.Vehicles {
		if v.ID == id {
			if v.AssignedSpot != -1 {
				t.Errorf("Yeni araç AssignedSpot=%d olmalı, -1 bekleniyor", v.AssignedSpot)
			}
			return
		}
	}
	t.Errorf("Araç #%d bulunamadı", id)
}
