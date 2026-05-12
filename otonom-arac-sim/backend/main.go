package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"otonom-arac-sim/models"
	"otonom-arac-sim/simulation"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Geliştirme için tüm originlere izin ver
	},
}

// Hub tüm WebSocket bağlantılarını yönetir
type Hub struct {
	mu      sync.Mutex
	clients map[*websocket.Conn]bool
}

func newHub() *Hub {
	return &Hub{clients: make(map[*websocket.Conn]bool)}
}

func (h *Hub) register(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[conn] = true
}

func (h *Hub) unregister(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, conn)
}

func (h *Hub) broadcast(data []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for conn := range h.clients {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			conn.Close()
			delete(h.clients, conn)
		}
	}
}

// Varsayılan konfigürasyon
func defaultConfig() models.SimConfig {
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

func main() {
	cfg := defaultConfig()
	sim := simulation.New(cfg)
	hub := newHub()

	// Simülasyon döngüsü: her 500ms bir tick
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			sim.Tick()
			state := sim.GetState()
			data, err := json.Marshal(state)
			if err == nil {
				hub.broadcast(data)
			}
		}
	}()

	// CORS middleware
	cors := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}
			next(w, r)
		}
	}

	// WebSocket endpoint
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("WebSocket upgrade hatası:", err)
			return
		}
		hub.register(conn)
		defer func() {
			hub.unregister(conn)
			conn.Close()
		}()

		// İlk bağlantıda mevcut durumu gönder
		state := sim.GetState()
		data, _ := json.Marshal(state)
		conn.WriteMessage(websocket.TextMessage, data)

		// Bağlantıyı açık tut
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	})

	// REST API: Durum al
	http.HandleFunc("/api/state", cors(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		state := sim.GetState()
		json.NewEncoder(w).Encode(state)
	}))

	// REST API: Simülasyonu başlat/durdur
	http.HandleFunc("/api/control", cors(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST gerekli", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Action string `json:"action"` // "start", "stop", "reset"
			Config *models.SimConfig `json:"config,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Geçersiz JSON", http.StatusBadRequest)
			return
		}
		switch body.Action {
		case "start":
			sim.SetRunning(true)
		case "stop":
			sim.SetRunning(false)
		case "reset":
			newCfg := cfg
			if body.Config != nil {
				newCfg = *body.Config
			}
			sim.Reset(newCfg)
			cfg = newCfg
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))

	// REST API: Araç ekle
	http.HandleFunc("/api/vehicle/add", cors(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST gerekli", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			TargetDepo int  `json:"targetDepo"`
			Priority   bool `json:"priority"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Geçersiz JSON", http.StatusBadRequest)
			return
		}
		id, err := sim.AddVehicle(body.TargetDepo, body.Priority)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]int{"id": id})
	}))

	// REST API: Konfigürasyon bilgisi
	http.HandleFunc("/api/config", cors(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cfg)
	}))

	fmt.Println("🚗 Otonom Araç Simülasyonu başlatıldı")
	fmt.Println("   Backend: http://localhost:8080")
	fmt.Println("   WebSocket: ws://localhost:8080/ws")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
