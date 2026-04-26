import { useEffect, useState } from 'react';
import './App.css';

interface Vehicle {
  id: number;
  position: number;
  direction: string;
  speed: number;
  status: string;
  targetDepo: number;
  hasLoad: boolean;
  priority: boolean;
  waitingAt: number;
}

interface Pocket {
  position: number;
  occupied: boolean;
}

interface Depot {
  position: number;
  vehicleCount: number;
  maxCapacity: number;
}

interface Sensor {
  position: number;
  red: boolean;
}

interface SimConfig {
  roadLength: number;
  sensorSpacing: number;
  sensorRange: number;
  minDistance: number;
  pocketPositions: number[];
  depotPositions: number[];
  maxPocketCap: number;
  maxDepotCap: number;
}

interface SimState {
  tick: number;
  vehicles: Vehicle[];
  pockets: Pocket[];
  depots: Depot[];
  sensors: Sensor[];
  config: SimConfig;
  running: boolean;
  events: string[];
}

const API_URL = 'http://localhost:8080';
const WS_URL = 'ws://localhost:8080/ws';

function App() {
  const [state, setState] = useState<SimState | null>(null);
  const [ws, setWs] = useState<WebSocket | null>(null);
  const [connected, setConnected] = useState(false);

  // WebSocket bağlantısı
  useEffect(() => {
    const socket = new WebSocket(WS_URL);
    
    socket.onopen = () => {
      console.log('WebSocket bağlandı');
      setConnected(true);
    };
    
    socket.onmessage = (event) => {
      const data = JSON.parse(event.data);
      setState(data);
    };
    
    socket.onerror = (error) => {
      console.error('WebSocket hatası:', error);
      setConnected(false);
    };
    
    socket.onclose = () => {
      console.log('WebSocket kapandı');
      setConnected(false);
    };
    
    setWs(socket);
    
    return () => {
      socket.close();
    };
  }, []);

  const handleControl = async (action: string) => {
    try {
      await fetch(`${API_URL}/api/control`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action }),
      });
    } catch (error) {
      console.error('Kontrol hatası:', error);
    }
  };

  const handleAddVehicle = async (targetDepo: number) => {
    try {
      await fetch(`${API_URL}/api/vehicle/add`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ targetDepo, priority: false }),
      });
    } catch (error) {
      console.error('Araç ekleme hatası:', error);
    }
  };

  if (!state) {
    return (
      <div className="app">
        <div className="loading">
          {connected ? 'Simülasyon yükleniyor...' : 'Backend bağlantısı bekleniyor...'}
        </div>
      </div>
    );
  }

  const roadLength = state.config.roadLength;
  const scale = 800 / roadLength; // 800px canvas genişliği

  return (
    <div className="app">
      <header>
        <h1>🚗 Otonom Araç Simülasyonu</h1>
        <div className="status">
          <span className={connected ? 'connected' : 'disconnected'}>
            {connected ? '● Bağlı' : '○ Bağlantı Yok'}
          </span>
          <span>Tick: {state.tick}</span>
          <span>Araç: {state.vehicles.length}</span>
        </div>
      </header>

      <div className="controls">
        <button onClick={() => handleControl('start')} disabled={state.running}>
          ▶ Başlat
        </button>
        <button onClick={() => handleControl('stop')} disabled={!state.running}>
          ⏸ Durdur
        </button>
        <button onClick={() => handleControl('reset')}>
          🔄 Sıfırla
        </button>
        <div className="divider"></div>
        {state.config.depotPositions.map((depo) => (
          <button key={depo} onClick={() => handleAddVehicle(depo)}>
            + Araç Ekle → {depo}m
          </button>
        ))}
      </div>

      <div className="simulation">
        <svg width="900" height="300" viewBox="0 0 900 300">
          {/* Yol */}
          <rect x="50" y="140" width="800" height="20" fill="#333" />
          <line x1="50" y1="150" x2="850" y2="150" stroke="#fff" strokeWidth="1" strokeDasharray="10,10" />

          {/* Sensörler */}
          {state.sensors.map((sensor) => (
            <g key={sensor.position}>
              <circle
                cx={50 + sensor.position * scale}
                cy={150}
                r="8"
                fill={sensor.red ? '#ff4444' : '#44ff44'}
                stroke="#000"
                strokeWidth="1"
              />
              <text
                x={50 + sensor.position * scale}
                y="185"
                textAnchor="middle"
                fontSize="10"
                fill="#666"
              >
                {sensor.position}m
              </text>
            </g>
          ))}

          {/* Cepler */}
          {state.pockets.map((pocket) => (
            <g key={pocket.position}>
              <rect
                x={50 + pocket.position * scale - 15}
                y="120"
                width="30"
                height="15"
                fill={pocket.occupied ? '#ff9900' : '#90EE90'}
                stroke="#000"
                strokeWidth="1"
              />
              <text
                x={50 + pocket.position * scale}
                y="115"
                textAnchor="middle"
                fontSize="10"
                fill="#000"
              >
                Cep
              </text>
            </g>
          ))}

          {/* Depolar */}
          {state.depots.map((depot) => {
            const depotX = 50 + depot.position * scale;
            // Depodaki araçları bul
            const vehiclesInDepot = state.vehicles.filter(v => 
              (v.status === 'loading' || v.status === 'exiting' || v.status === 'waiting') && 
              Math.abs(v.position - depot.position) < 1
            );
            
            return (
              <g key={depot.position}>
                {/* Depo binası (yan tarafta, büyük) */}
                <rect
                  x={depotX - 25}
                  y="50"
                  width="50"
                  height="60"
                  fill="#4169E1"
                  stroke="#000"
                  strokeWidth="2"
                  rx="5"
                />
                {/* Depo kapısı */}
                <rect
                  x={depotX - 10}
                  y="90"
                  width="20"
                  height="20"
                  fill="#2c4a7c"
                  stroke="#000"
                  strokeWidth="1"
                />
                {/* Depo etiketi */}
                <text
                  x={depotX}
                  y="75"
                  textAnchor="middle"
                  fontSize="11"
                  fill="#fff"
                  fontWeight="bold"
                >
                  DEPO
                </text>
                <text
                  x={depotX}
                  y="88"
                  textAnchor="middle"
                  fontSize="10"
                  fill="#fff"
                >
                  {depot.position}m
                </text>
                {/* Araç sayısı */}
                <text
                  x={depotX}
                  y="125"
                  textAnchor="middle"
                  fontSize="9"
                  fill="#666"
                  fontWeight="bold"
                >
                  {depot.vehicleCount}/{depot.maxCapacity}
                </text>
                
                {/* Depodaki araçları göster */}
                {vehiclesInDepot.slice(0, 3).map((v, idx) => (
                  <g key={v.id}>
                    <rect
                      x={depotX - 18 + (idx * 12)}
                      y="60"
                      width="10"
                      height="8"
                      fill={v.hasLoad ? '#00ff00' : '#ff4444'}
                      stroke="#000"
                      strokeWidth="1"
                      rx="2"
                    />
                    <text
                      x={depotX - 13 + (idx * 12)}
                      y="66"
                      textAnchor="middle"
                      fontSize="6"
                      fill="#fff"
                      fontWeight="bold"
                    >
                      {v.id}
                    </text>
                  </g>
                ))}
                
                {/* Yol üzerinde depo işareti (küçük) */}
                <rect
                  x={depotX - 15}
                  y="165"
                  width="30"
                  height="15"
                  fill="#4169E1"
                  stroke="#000"
                  strokeWidth="1"
                  opacity="0.5"
                />
                <text
                  x={depotX}
                  y="175"
                  textAnchor="middle"
                  fontSize="8"
                  fill="#fff"
                  fontWeight="bold"
                >
                  ↑
                </text>
              </g>
            );
          })}

          {/* Araçlar (sadece yol üzerindekiler) */}
          {state.vehicles
            .filter(vehicle => {
              // Kuyrukta bekleyen araçları gösterme
              if (vehicle.status === 'queued' || vehicle.position < 0) return false;
              // Depoda olan araçları gösterme
              if (vehicle.status === 'loading' || vehicle.status === 'exiting') return false;
              // Depoda bekleyen araçları gösterme
              if (vehicle.status === 'waiting') {
                const inDepot = state.depots.some(d => 
                  Math.abs(vehicle.position - d.position) < 1
                );
                if (inDepot) return false;
              }
              return true;
            })
            .map((vehicle) => {
            const x = 50 + vehicle.position * scale;
            const y = 150;
            const color = vehicle.status === 'done' ? '#888' : 
                         vehicle.status === 'waiting' ? '#ff9900' :
                         vehicle.status === 'retreating' ? '#ffaa00' :
                         vehicle.status === 'loading' ? '#4169E1' :
                         vehicle.hasLoad ? '#00ff00' : '#ff4444';
            
            return (
              <g key={vehicle.id}>
                {/* Araç gövdesi */}
                <rect
                  x={x - 12}
                  y={y - 8}
                  width="24"
                  height="16"
                  fill={color}
                  stroke="#000"
                  strokeWidth="2"
                  rx="3"
                />
                {/* Yön oku */}
                {vehicle.direction === 'right' && (
                  <polygon
                    points={`${x + 8},${y} ${x + 14},${y - 4} ${x + 14},${y + 4}`}
                    fill="#000"
                  />
                )}
                {vehicle.direction === 'left' && (
                  <polygon
                    points={`${x - 8},${y} ${x - 14},${y - 4} ${x - 14},${y + 4}`}
                    fill="#000"
                  />
                )}
                {/* Araç ID */}
                <text
                  x={x}
                  y={y + 4}
                  textAnchor="middle"
                  fontSize="10"
                  fill="#fff"
                  fontWeight="bold"
                >
                  {vehicle.id}
                </text>
                {/* Pozisyon bilgisi */}
                <text
                  x={x}
                  y={y - 15}
                  textAnchor="middle"
                  fontSize="9"
                  fill="#000"
                >
                  {Math.round(vehicle.position)}m
                </text>
              </g>
            );
          })}

          {/* Başlangıç ve bitiş işaretleri */}
          <text x="50" y="130" textAnchor="middle" fontSize="12" fill="#000" fontWeight="bold">
            Başlangıç
          </text>
          <text x="850" y="130" textAnchor="middle" fontSize="12" fill="#000" fontWeight="bold">
            Bitiş
          </text>
          
          {/* Kuyrukta bekleyen araçlar (garaj) */}
          {state.vehicles.filter(v => v.status === 'queued').map((v, idx) => (
            <g key={v.id}>
              <rect
                x={10}
                y={150 + (idx * 25)}
                width="30"
                height="20"
                fill="#888"
                stroke="#000"
                strokeWidth="2"
                rx="3"
              />
              <text
                x={25}
                y={163 + (idx * 25)}
                textAnchor="middle"
                fontSize="10"
                fill="#fff"
                fontWeight="bold"
              >
                #{v.id}
              </text>
              <text
                x={25}
                y={185 + (idx * 25)}
                textAnchor="middle"
                fontSize="8"
                fill="#666"
              >
                Bekliyor
              </text>
            </g>
          ))}
        </svg>
      </div>

      <div className="info-panel">
        <div className="info-section">
          <h3>Araç Durumları</h3>
          {state.vehicles.length === 0 ? (
            <p>Henüz araç yok. Yukarıdan araç ekleyin.</p>
          ) : (
            <table>
              <thead>
                <tr>
                  <th>ID</th>
                  <th>Pozisyon</th>
                  <th>Durum</th>
                  <th>Hedef</th>
                  <th>Yük</th>
                </tr>
              </thead>
              <tbody>
                {state.vehicles.map((v) => (
                  <tr key={v.id}>
                    <td>#{v.id}</td>
                    <td>{Math.round(v.position)}m</td>
                    <td>{v.status}</td>
                    <td>{v.targetDepo}m</td>
                    <td>{v.hasLoad ? '✓' : '✗'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>

        <div className="info-section">
          <h3>Olaylar</h3>
          <div className="events">
            {state.events.length === 0 ? (
              <p>Henüz olay yok.</p>
            ) : (
              state.events.map((event, i) => (
                <div key={i} className="event">
                  {event}
                </div>
              ))
            )}
          </div>
        </div>

        <div className="info-section">
          <h3>Sistem Bilgisi</h3>
          <ul>
            <li>Yol Uzunluğu: {state.config.roadLength}m</li>
            <li>Min Mesafe: {state.config.minDistance}m</li>
            <li>Sensör Aralığı: {state.config.sensorSpacing}m</li>
            <li>Cep Sayısı: {state.pockets.length}</li>
            <li>Depo Sayısı: {state.depots.length}</li>
          </ul>
        </div>
      </div>

      <div className="legend">
        <h4>Renk Açıklaması:</h4>
        <div className="legend-items">
          <div><span className="color-box" style={{background: '#ff4444'}}></span> Boş araç (gidiyor)</div>
          <div><span className="color-box" style={{background: '#00ff00'}}></span> Yüklü araç (dönüyor)</div>
          <div><span className="color-box" style={{background: '#ffaa00'}}></span> Geri çekiliyor</div>
          <div><span className="color-box" style={{background: '#ff9900'}}></span> Bekliyor</div>
          <div><span className="color-box" style={{background: '#4169E1'}}></span> Yükleniyor</div>
          <div><span className="color-box" style={{background: '#888'}}></span> Tamamlandı</div>
        </div>
      </div>
    </div>
  );
}

export default App;
