import { useState } from 'react'
import type { ReplicaStatus } from '../types'
import { apiPost } from '../hooks/useReplica'

interface Props {
  url: string
  label: string
  status: ReplicaStatus | null
  color: string
  isSelected: boolean
  onSelect: () => void
}

export default function ReplicaPanel({ url, label, status, color, isSelected, onSelect }: Props) {
  const [latencyMs, setLatencyMs] = useState('0')
  const [lossRate, setLossRate] = useState('0')
  const [simUsers, setSimUsers] = useState('10')
  const [simOps, setSimOps] = useState('5')
  const [busy, setBusy] = useState(false)

  const call = async (path: string, body?: object) => {
    setBusy(true)
    await apiPost(`${url}${path}`, body)
    setBusy(false)
  }

  const offline = !status || !!status.error
  const s = status

  return (
    <div
      className={`replica-panel ${isSelected ? 'selected' : ''} ${offline ? 'panel-offline' : ''}`}
      style={{ '--accent': color, '--client-a': color } as React.CSSProperties}
    >
      <div className="panel-header" onClick={onSelect}>
        <span className="panel-title">{label}</span>
        <span className={`dot ${offline ? 'dot-offline' : 'dot-online'}`} />
      </div>

      {offline ? (
        <div className="panel-offline-msg">Offline — is Docker running?</div>
      ) : (
        <>
          <div className="stats-row">
            <div className="stat">
              <div className="stat-val">{s!.nodes}</div>
              <div className="stat-lbl">nodes</div>
            </div>
            <div className="stat">
              <div className="stat-val">{s!.edges}</div>
              <div className="stat-lbl">edges</div>
            </div>
            <div className="stat">
              <div className="stat-val">{s!.opLogLen}</div>
              <div className="stat-lbl">ops</div>
            </div>
            <div className="stat">
              <div className="stat-val mono" title={s!.stateHash} style={{ fontSize: 11 }}>{s!.stateHash.slice(0, 7)}</div>
              <div className="stat-lbl">hash</div>
            </div>
          </div>

          <div className="clock-row">
            {Object.entries(s!.vectorClock).map(([k, v]) => (
              <span key={k} className="clock-chip">
                {k.replace('replica-', '')}:{v}
              </span>
            ))}
          </div>

          <div className="badge-row">
            {s!.chaos.isolated && <span className="badge badge-red">ISOLATED</span>}
            {s!.chaos.lossRate > 0 && (
              <span className="badge badge-yellow">LOSS {Math.round(s!.chaos.lossRate * 100)}%</span>
            )}
            {s!.chaos.latencyMs > 0 && (
              <span className="badge badge-yellow">{s!.chaos.latencyMs}ms LAG</span>
            )}
            {s!.sim.running && (
              <span className="badge badge-blue">
                {s!.sim.users}u · {s!.sim.totalOps} ops
              </span>
            )}
          </div>

          <div className="controls">
            <div className="ctrl-row">
              <button
                className={`btn ${s!.chaos.isolated ? 'btn-green' : 'btn-red'}`}
                disabled={busy}
                onClick={() => call(s!.chaos.isolated ? '/sim/recover' : '/sim/isolate')}
                style={{ flex: 1 }}
              >
                {s!.chaos.isolated ? 'Recover' : 'Isolate'}
              </button>
            </div>

            <div className="ctrl-row">
              <span className="ctrl-label">Latency</span>
              <input
                className="ctrl-input"
                value={latencyMs}
                onChange={e => setLatencyMs(e.target.value)}
                placeholder="ms"
              />
              <button
                className="btn btn-ghost"
                disabled={busy}
                onClick={() => call('/sim/latency', { ms: parseInt(latencyMs) || 0 })}
              >
                Set
              </button>
            </div>

            <div className="ctrl-row">
              <span className="ctrl-label">Loss</span>
              <input
                className="ctrl-input"
                value={lossRate}
                onChange={e => setLossRate(e.target.value)}
                placeholder="0–1"
              />
              <button
                className="btn btn-ghost"
                disabled={busy}
                onClick={() => call('/sim/loss', { rate: parseFloat(lossRate) || 0 })}
              >
                Set
              </button>
            </div>

            <div className="ctrl-row sim-row">
              <span className="ctrl-label">Sim</span>
              <input
                className="ctrl-input"
                value={simUsers}
                onChange={e => setSimUsers(e.target.value)}
                placeholder="users"
              />
              <input
                className="ctrl-input"
                value={simOps}
                onChange={e => setSimOps(e.target.value)}
                placeholder="ops/s"
              />
              {s!.sim.running ? (
                <button className="btn btn-yellow" disabled={busy} onClick={() => call('/sim/users/stop')}>
                  Stop
                </button>
              ) : (
                <button
                  className="btn btn-blue"
                  disabled={busy}
                  onClick={() => call('/sim/users/start', {
                    users: parseInt(simUsers) || 10,
                    opsPerSec: parseFloat(simOps) || 5.0,
                  })}
                >
                  Start
                </button>
              )}
            </div>
          </div>
        </>
      )}
    </div>
  )
}
