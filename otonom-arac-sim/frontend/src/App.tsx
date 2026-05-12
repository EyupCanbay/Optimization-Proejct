import { useEffect, useState, useRef } from 'react';
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
  waitTickCount: number;
  assignedSpot: number;
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

interface OptStats {
  totalWaitTime: number;
  collisionAvoided: number;
  deadlockResolved: number;
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
  deadlockCount: number;
  optStats: OptStats;
}

interface EventEntry {
  tick: number;
  msg: string;
}

const API_URL = 'http://localhost:8080';
const WS_URL = 'ws://localhost:8080/ws';

function App() {
  const [state, setState] = useState<SimState | null>(null);
  const [eventHistory, setEventHistory] = useState<EventEntry[]>([]);
  const [connected, setConnected] = useState(false);
  const eventsEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const socket = new WebSocket(WS_URL);

    socket.onopen = () => {
      setConnected(true);
    };

    socket.onmessage = (event) => {
      const data: SimState = JSON.parse(event.data);
      setState(data);
      if (data.events && data.events.length > 0) {
        const newEntries: EventEntry[] = data.events.map(msg => ({ tick: data.tick, msg }));
        setEventHistory(prev => [...newEntries, ...prev].slice(0, 150));
      }
    };

    socket.onerror = () => setConnected(false);
    socket.onclose = () => setConnected(false);

    return () => socket.close();
  }, []);

  const handleControl = async (action: string) => {
    if (action === 'reset') setEventHistory([]);
    try {
      await fetch(`${API_URL}/api/control`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action }),
      });
    } catch (e) {
      console.error(e);
    }
  };

  const handleAddVehicle = async (targetDepo: number, priority = false) => {
    try {
      await fetch(`${API_URL}/api/vehicle/add`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ targetDepo, priority }),
      });
    } catch (e) {
      console.error(e);
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
  const scale = 800 / roadLength;

  const activeVehicles = state.vehicles.filter(v => v.status !== 'done');
  const doneVehicles   = state.vehicles.filter(v => v.status === 'done');

  const statusColor = (v: Vehicle) => {
    if (v.status === 'done')       return '#888';
    if (v.status === 'waiting')    return '#ff9900';
    if (v.status === 'retreating') return '#ffaa00';
    if (v.status === 'loading')    return '#4169E1';
    if (v.status === 'exiting')    return '#4169E1';
    return v.hasLoad ? '#00cc44' : '#ff4444';
  };

  // Yol üzerinde gösterilecek araçlar
  const roadVehicles = state.vehicles.filter(v => {
    if (v.status === 'queued' || v.position < 0) return false;
    if (v.status === 'loading' || v.status === 'exiting') return false;
    if (v.status === 'done') return false;
    if (v.status === 'waiting') {
      const inDepot = state.depots.some(d => Math.abs(v.position - d.position) < 1);
      const inPocket = state.pockets.some(p => Math.abs(v.position - p.position) < 1);
      if (inDepot || inPocket) return false;
    }
    return true;
  });

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
          <span title="Toplam çözülen deadlock sayısı">🔒 Deadlock: {state.deadlockCount ?? 0}</span>
        </div>
      </header>

      <div className="controls">
        <button onClick={() => handleControl('start')} disabled={state.running}>▶ Başlat</button>
        <button onClick={() => handleControl('stop')}  disabled={!state.running}>⏸ Durdur</button>
        <button onClick={() => handleControl('reset')}>🔄 Sıfırla</button>
        <div className="divider"></div>
        {state.config.depotPositions.map(depo => (
          <span key={depo} style={{ display: 'inline-flex', gap: '2px', marginRight: '6px' }}>
            <button onClick={() => handleAddVehicle(depo)}>+ Araç → {depo}m</button>
            <button
              onClick={() => handleAddVehicle(depo, true)}
              style={{ background: '#c0392b', padding: '0 8px', fontSize: '11px' }}
              title={`Öncelikli araç ekle (${depo}m)`}
            >⭐</button>
          </span>
        ))}
      </div>

      {/* SVG Simülasyon Görünümü */}
      <div className="simulation">
        <svg width="900" height="300" viewBox="0 0 900 300">
          {/* Yol */}
          <rect x="50" y="140" width="800" height="20" fill="#333" />
          <line x1="50" y1="150" x2="850" y2="150" stroke="#fff" strokeWidth="1" strokeDasharray="10,10" />

          {/* Sensörler */}
          {state.sensors.map(sensor => (
            <g key={sensor.position}>
              <circle
                cx={50 + sensor.position * scale} cy={150} r="8"
                fill={sensor.red ? '#ff4444' : '#44ff44'} stroke="#000" strokeWidth="1"
              />
              <text x={50 + sensor.position * scale} y="185" textAnchor="middle" fontSize="10" fill="#666">
                {sensor.position}m
              </text>
            </g>
          ))}

          {/* Cepler */}
          {state.pockets.map(pocket => {
            const px = 50 + pocket.position * scale;
            const vehiclesInPocket = state.vehicles.filter(v => 
              v.status === 'waiting' && Math.abs(v.position - pocket.position) < 1
            );
            return (
              <g key={pocket.position}>
                {/* Cebin Ana Kasası */}
                <rect x={px - 20} y="180" width="40" height="50" fill="#2E8B57" stroke="#000" strokeWidth="2" rx="4" />
                <rect x={px - 10} y="165" width="20" height="15" fill="#1b5233" stroke="#000" strokeWidth="1" />
                
                <text x={px} y="205" textAnchor="middle" fontSize="10" fill="#fff" fontWeight="bold">CEP</text>
                <text x={px} y="215" textAnchor="middle" fontSize="8" fill="#fff">{pocket.position}m</text>
                <text x={px} y="225" textAnchor="middle" fontSize="8" fill="#ddd" fontWeight="bold">
                  {pocket.occupied ? '1' : '0'} / 1
                </text>

                {/* İçindeki Araç (varsa) */}
                {vehiclesInPocket.slice(0, 1).map((v) => (
                  <g key={v.id}>
                    <rect x={px - 10} y="185" width="20" height="10"
                      fill={v.hasLoad ? '#00cc44' : '#ff4444'} stroke="#fff" strokeWidth="1" rx="2" />
                    <text x={px} y="193" textAnchor="middle" fontSize="7" fill="#fff" fontWeight="bold">
                      {v.id}
                    </text>
                  </g>
                ))}
                
                {/* Yola Bağlantı Oku */}
                <text x={px} y="160" textAnchor="middle" fontSize="10" fill="#fff" fontWeight="bold">↓</text>
              </g>
            );
          })}

          {/* Depolar */}
          {state.depots.map(depot => {
            const dx = 50 + depot.position * scale;
            const vehiclesInDepot = state.vehicles.filter(v =>
              (v.status === 'loading' || v.status === 'exiting' || v.status === 'waiting') &&
              Math.abs(v.position - depot.position) < 1
            );
            return (
              <g key={depot.position}>
                <rect x={dx - 25} y="50" width="50" height="60" fill="#4169E1" stroke="#000" strokeWidth="2" rx="5" />
                <rect x={dx - 10} y="90" width="20" height="20" fill="#2c4a7c" stroke="#000" strokeWidth="1" />
                <text x={dx} y="75" textAnchor="middle" fontSize="11" fill="#fff" fontWeight="bold">DEPO</text>
                <text x={dx} y="88" textAnchor="middle" fontSize="10" fill="#fff">{depot.position}m</text>
                <text x={dx} y="125" textAnchor="middle" fontSize="9" fill="#ccc" fontWeight="bold">
                  {depot.vehicleCount}/{depot.maxCapacity}
                </text>
                {vehiclesInDepot.slice(0, 3).map((v, idx) => (
                  <g key={v.id}>
                    <rect x={dx - 18 + idx * 12} y="60" width="10" height="8"
                      fill={v.hasLoad ? '#00cc44' : '#ff4444'} stroke="#000" strokeWidth="1" rx="2" />
                    <text x={dx - 13 + idx * 12} y="66" textAnchor="middle" fontSize="6" fill="#fff" fontWeight="bold">
                      {v.id}
                    </text>
                  </g>
                ))}
                <rect x={dx - 15} y="165" width="30" height="15" fill="#4169E1" stroke="#000" strokeWidth="1" opacity="0.5" />
                <text x={dx} y="175" textAnchor="middle" fontSize="8" fill="#fff" fontWeight="bold">↑</text>
              </g>
            );
          })}

          {/* Yol Üzerindeki Araçlar */}
          {roadVehicles.map(vehicle => {
            const x = 50 + vehicle.position * scale;
            const y = 150;
            const col = statusColor(vehicle);
            return (
              <g key={vehicle.id}>
                <rect x={x - 12} y={y - 8} width="24" height="16"
                  fill={col} stroke={vehicle.priority ? '#FFD700' : '#000'} strokeWidth={vehicle.priority ? 3 : 2} rx="3"
                />
                {vehicle.direction === 'right' && (
                  <polygon points={`${x + 8},${y} ${x + 14},${y - 4} ${x + 14},${y + 4}`} fill="#000" />
                )}
                {vehicle.direction === 'left' && (
                  <polygon points={`${x - 8},${y} ${x - 14},${y - 4} ${x - 14},${y + 4}`} fill="#000" />
                )}
                <text x={x} y={y + 4} textAnchor="middle" fontSize="10" fill="#fff" fontWeight="bold">{vehicle.id}</text>
                <text x={x} y={y - 15} textAnchor="middle" fontSize="9" fill="#333">{Math.round(vehicle.position)}m</text>
                {/* Bekleme sayacı uyarısı */}
                {vehicle.waitTickCount > 5 && (
                  <text x={x} y={y - 25} textAnchor="middle" fontSize="8" fill="#e74c3c" fontWeight="bold">
                    ⏱{vehicle.waitTickCount}
                  </text>
                )}
              </g>
            );
          })}

          {/* Başlangıç / Bitiş */}
          <text x="50" y="130" textAnchor="middle" fontSize="12" fill="#000" fontWeight="bold">Başlangıç</text>
          <text x="850" y="130" textAnchor="middle" fontSize="12" fill="#000" fontWeight="bold">Bitiş</text>

          {/* Kuyrukta Bekleyen Araçlar */}
          {state.vehicles.filter(v => v.status === 'queued').map((v, idx) => (
            <g key={v.id}>
              <rect x="10" y={150 + idx * 25} width="30" height="20" fill="#888" stroke="#000" strokeWidth="2" rx="3" />
              <text x="25" y={163 + idx * 25} textAnchor="middle" fontSize="10" fill="#fff" fontWeight="bold">#{v.id}</text>
              <text x="25" y={185 + idx * 25} textAnchor="middle" fontSize="8" fill="#666">Kuyruk</text>
            </g>
          ))}
        </svg>
      </div>

      {/* Alt Panel */}
      <div className="info-panel">

        {/* Araç Durumları */}
        <div className="info-section">
          <h3>Araç Durumları</h3>
          {state.vehicles.length === 0 ? (
            <p>Henüz araç yok.</p>
          ) : (
            <>
              <table>
                <thead>
                  <tr>
                    <th>ID</th>
                    <th>Pozisyon</th>
                    <th>Durum</th>
                    <th>Hedef</th>
                    <th>Yük</th>
                    <th>Önc.</th>
                    <th>Bekleme</th>
                  </tr>
                </thead>
                <tbody>
                  {activeVehicles.map(v => (
                    <tr
                      key={v.id}
                      style={{
                        background: v.waitTickCount > 10
                          ? '#ffe0e0'
                          : v.status === 'waiting' || v.status === 'retreating'
                          ? '#fff3cd'
                          : undefined
                      }}
                    >
                      <td>#{v.id}</td>
                      <td>{v.position < 0 ? 'Garaj' : `${Math.round(v.position)}m`}</td>
                      <td>
                        <span className={`status-badge status-${v.status}`}>{v.status}</span>
                      </td>
                      <td>{v.targetDepo}m</td>
                      <td>{v.hasLoad ? '✓' : '✗'}</td>
                      <td>{v.priority ? '⭐' : '-'}</td>
                      <td style={{ color: v.waitTickCount > 10 ? '#e74c3c' : undefined, fontWeight: v.waitTickCount > 10 ? 'bold' : undefined }}>
                        {v.waitTickCount}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
              {doneVehicles.length > 0 && (
                <details style={{ marginTop: '6px' }}>
                  <summary style={{ cursor: 'pointer', color: '#888', fontSize: '12px' }}>
                    ✅ Tamamlanan araçlar ({doneVehicles.length})
                  </summary>
                  <table>
                    <tbody>
                      {doneVehicles.map(v => (
                        <tr key={v.id} style={{ opacity: 0.45 }}>
                          <td>#{v.id}</td>
                          <td>0m</td>
                          <td>done</td>
                          <td>{v.targetDepo}m</td>
                          <td>✗</td>
                          <td>{v.priority ? '⭐' : '-'}</td>
                          <td>0</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </details>
              )}
            </>
          )}
        </div>

        {/* Olay Geçmişi */}
        <div className="info-section">
          <h3>
            Olay Geçmişi{' '}
            <span style={{ fontSize: '11px', color: '#888', fontWeight: 'normal' }}>
              (son {eventHistory.length})
            </span>
          </h3>
          <div className="events">
            {eventHistory.length === 0 ? (
              <p>Henüz olay yok.</p>
            ) : (
              eventHistory.map((entry, i) => (
                <div
                  key={i}
                  className={`event${entry.msg.includes('DEADLOCK') ? ' event-deadlock' : entry.msg.includes('geri çekili') ? ' event-retreat' : ''}`}
                >
                  <span className="event-tick">[T{entry.tick}]</span> {entry.msg}
                </div>
              ))
            )}
            <div ref={eventsEndRef} />
          </div>
        </div>

        {/* Sistem Bilgisi + OptStats */}
        <div className="info-section">
          <h3>Sistem Bilgisi</h3>
          <ul>
            <li>Yol: {state.config.roadLength}m</li>
            <li>Min Mesafe: {state.config.minDistance}m</li>
            <li>Sensör Aralığı: {state.config.sensorSpacing}m</li>
            <li>Cep Sayısı: {state.pockets.length}</li>
            <li>Depo Sayısı: {state.depots.length}</li>
          </ul>

          <h4 style={{ marginTop: '14px', marginBottom: '6px' }}>📊 Optimizasyon İstatistikleri</h4>
          <ul>
            <li>🛑 Kaçınılan Çarpışma: <strong>{state.optStats?.collisionAvoided ?? 0}</strong></li>
            <li>⚠️ Çözülen Deadlock: <strong>{state.optStats?.deadlockResolved ?? 0}</strong></li>
            <li>⏱ Toplam Bekleme: <strong>{state.optStats?.totalWaitTime ?? 0} tick</strong></li>
          </ul>
        </div>
      </div>

      {/* Renk Açıklaması */}
      <div className="legend">
        <h4>Renk Açıklaması:</h4>
        <div className="legend-items">
          <div><span className="color-box" style={{ background: '#ff4444' }}></span> Boş araç (gidiyor)</div>
          <div><span className="color-box" style={{ background: '#00cc44' }}></span> Yüklü araç (dönüyor)</div>
          <div><span className="color-box" style={{ background: '#ffaa00' }}></span> Geri çekiliyor</div>
          <div><span className="color-box" style={{ background: '#ff9900' }}></span> Bekliyor</div>
          <div><span className="color-box" style={{ background: '#4169E1' }}></span> Yükleniyor/Çıkıyor</div>
          <div><span className="color-box" style={{ background: '#888' }}></span> Tamamlandı</div>
        </div>
      </div>
    </div>
  );
}

export default App;
