package models

// Direction araç yönünü temsil eder
type Direction int

const (
	DirRight Direction = iota // Soldan sağa (0 → 240)
	DirLeft                   // Sağdan sola (240 → 0)
	DirNone                   // Hareketsiz
)

func (d Direction) String() string {
	switch d {
	case DirRight:
		return "right"
	case DirLeft:
		return "left"
	default:
		return "none"
	}
}

// VehicleStatus araç durumunu temsil eder
type VehicleStatus string

const (
	StatusMoving     VehicleStatus = "moving"
	StatusRetreating VehicleStatus = "retreating" // Cep/depoya geri çekiliyor
	StatusWaiting    VehicleStatus = "waiting"    // Cep/depoda bekliyor
	StatusLoading    VehicleStatus = "loading"    // Depoda yük alıyor
	StatusExiting    VehicleStatus = "exiting"    // Depodan çıkıyor
	StatusQueued     VehicleStatus = "queued"     // Başlangıçta kuyrukta bekliyor
	StatusDone       VehicleStatus = "done"       // Görevi tamamladı
)

// Vehicle tek bir otonom aracı temsil eder
type Vehicle struct {
	ID         int           `json:"id"`
	Position   float64       `json:"position"`   // 0-240 metre arası
	Direction  Direction     `json:"direction"`  // right veya left
	Speed      float64       `json:"speed"`      // km/h (max 20)
	Status     VehicleStatus `json:"status"`
	TargetDepo int           `json:"targetDepo"` // Hedef depo pozisyonu
	HasLoad    bool          `json:"hasLoad"`    // Yük taşıyor mu
	Priority   bool          `json:"priority"`   // Öncelikli araç mı
	WaitingAt  int           `json:"waitingAt"`  // Beklediği konum (-1 = beklemiyorsa)
}

// Pocket (Cep) araçların bekleyebileceği alanları temsil eder
type Pocket struct {
	Position int  `json:"position"`
	Occupied bool `json:"occupied"` // Dolu mu (max 1 araç)
}

// Depot (Ara Depo) yük alınan noktaları temsil eder
type Depot struct {
	Position     int  `json:"position"`
	VehicleCount int  `json:"vehicleCount"` // Şu an kaç araç var (max 3)
	MaxCapacity  int  `json:"maxCapacity"`  // Maksimum kapasite (3)
}

// Sensor yol üzerindeki sensörleri temsil eder
type Sensor struct {
	Position int  `json:"position"`
	Red      bool `json:"red"` // true = kırmızı (araç algılandı)
}

// SimConfig simülasyon konfigürasyonunu tutar
type SimConfig struct {
	RoadLength    int       `json:"roadLength"`    // Yol uzunluğu (240 m)
	SensorSpacing int       `json:"sensorSpacing"` // Sensör aralığı (40 m)
	SensorRange   int       `json:"sensorRange"`   // Sensör algılama mesafesi (±20 m)
	MinDistance   float64   `json:"minDistance"`   // Araçlar arası min mesafe (20 m)
	PocketPositions []int   `json:"pocketPositions"` // Cep konumları
	DepotPositions  []int   `json:"depotPositions"`  // Depo konumları
	MaxPocketCap    int     `json:"maxPocketCap"`    // Cep max araç (1)
	MaxDepotCap     int     `json:"maxDepotCap"`     // Depo max araç (3)
}

// SimState simülasyonun anlık durumunu temsil eder
type SimState struct {
	Tick     int        `json:"tick"`     // Simülasyon adımı
	Vehicles []Vehicle  `json:"vehicles"`
	Pockets  []Pocket   `json:"pockets"`
	Depots   []Depot    `json:"depots"`
	Sensors  []Sensor   `json:"sensors"`
	Config   SimConfig  `json:"config"`
	Running  bool       `json:"running"`
	Events   []string   `json:"events"` // Bu tick'te olan olaylar
}
