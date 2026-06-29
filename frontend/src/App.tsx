import { useState, useEffect, useRef } from 'react'
import { useReplicas, useGraph, useOps, useReplay } from './hooks/useReplica'
import ReplicaPanel from './components/ReplicaPanel'
import GraphView from './components/GraphView'
import Timeline from './components/Timeline'
import ConvergenceChart from './components/ConvergenceChart'
import ReplayBar from './components/ReplayBar'
import { REPLICAS, REPLICA_COLORS } from './types'
import type { HistoryPoint } from './types'

export default function App() {
  const statuses = useReplicas(1500)
  const [selectedIdx, setSelectedIdx] = useState(0)
  const [replayIndex, setReplayIndex] = useState<number | null>(null)
  const [history, setHistory] = useState<HistoryPoint[]>([])
  const historyRef = useRef<HistoryPoint[]>([])

  const selectedUrl = REPLICAS[selectedIdx].url
  const selectedColor = REPLICA_COLORS[selectedIdx]
  const liveGraph = useGraph(selectedUrl, 1500)
  const ops = useOps(selectedUrl, 1500)
  const { graph: replayGraph, loading: replayLoading } = useReplay(selectedUrl, replayIndex)

  // Accumulate history for the convergence chart (up to 60 samples = ~90 s).
  useEffect(() => {
    const hasAny = statuses.some((s) => s && !s.error)
    if (!hasAny) return
    const point: HistoryPoint = {
      nodes: [
        statuses[0]?.error ? 0 : (statuses[0]?.nodes ?? 0),
        statuses[1]?.error ? 0 : (statuses[1]?.nodes ?? 0),
        statuses[2]?.error ? 0 : (statuses[2]?.nodes ?? 0),
      ],
      hashes: [
        statuses[0]?.stateHash ?? '',
        statuses[1]?.stateHash ?? '',
        statuses[2]?.stateHash ?? '',
      ],
    }
    historyRef.current = [...historyRef.current.slice(-59), point]
    setHistory([...historyRef.current])
  }, [statuses])

  // Exit replay mode when the selected replica changes.
  useEffect(() => {
    setReplayIndex(null)
  }, [selectedIdx])

  // Convergence verdict.
  const online = statuses.filter((s) => s && !s.error && s.stateHash)
  const allConverged =
    online.length > 1 && online.every((s) => s!.stateHash === online[0]!.stateHash)
  const anyDiverged = online.length > 1 && !allConverged

  const opLogLen = statuses[selectedIdx]?.opLogLen ?? 0
  const displayGraph = replayIndex !== null && replayGraph ? replayGraph : liveGraph

  return (
    <div className="app">
      <header className="app-header">
        <div className="header-left">
          <span className="logo">⬡</span>
          <span className="app-name">SentinelSync</span>
          <span className="app-tagline">Distributed State Engine</span>
        </div>
        <div className="header-right">
          {online.length > 0 && (
            <div className={`conv-badge ${anyDiverged ? 'badge-diverged' : 'badge-converged'}`}>
              {anyDiverged ? '⚠ Diverged' : '✓ Converged'}
            </div>
          )}
          <div className="header-meta">
            {online.length}/{REPLICAS.length} replicas online
          </div>
        </div>
      </header>

      <div className="replica-row">
        {REPLICAS.map((rep, i) => (
          <ReplicaPanel
            key={rep.id}
            url={rep.url}
            label={rep.label}
            status={statuses[i]}
            color={REPLICA_COLORS[i]}
            isSelected={selectedIdx === i}
            onSelect={() => setSelectedIdx(i)}
          />
        ))}
      </div>

      <section className="chart-section">
        <div className="section-header">
          <span>Convergence History</span>
          <span className="section-meta">last {history.length} samples · 1.5 s interval</span>
        </div>
        <ConvergenceChart history={history} />
      </section>

      <div className="main-grid">
        <section className="graph-section">
          <div className="section-header">
            <span>
              Graph
              {replayIndex !== null && (
                <span className="replay-mode-label"> — REPLAY op {replayIndex + 1}/{opLogLen}</span>
              )}
            </span>
            <div className="tab-group">
              {REPLICAS.map((rep, i) => (
                <button
                  key={rep.id}
                  className={`tab ${selectedIdx === i ? 'tab-active' : ''}`}
                  style={selectedIdx === i ? { borderColor: REPLICA_COLORS[i], color: REPLICA_COLORS[i] } : {}}
                  onClick={() => setSelectedIdx(i)}
                >
                  {rep.label}
                </button>
              ))}
            </div>
          </div>

          <ReplayBar
            opLogLen={opLogLen}
            index={replayIndex}
            loading={replayLoading}
            onChange={setReplayIndex}
          />

          <div className="graph-wrap">
            <GraphView graph={displayGraph} color={selectedColor} />
          </div>
        </section>

        <section className="timeline-section">
          <div className="section-header">
            <span>Operation Timeline</span>
            <span className="section-meta">{REPLICAS[selectedIdx].label}</span>
          </div>
          <Timeline ops={ops} />
        </section>
      </div>
    </div>
  )
}
