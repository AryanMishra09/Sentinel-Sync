import { useState, useEffect, useRef } from 'react'
import { useReplicas, useGraph, useOps, useReplay } from '../hooks/useReplica'
import ReplicaPanel from '../components/ReplicaPanel'
import GraphView from '../components/GraphView'
import Timeline from '../components/Timeline'
import ConvergenceChart from '../components/ConvergenceChart'
import ReplayBar from '../components/ReplayBar'
import GraphEditor from '../components/GraphEditor'
import ScenarioRunner from '../components/ScenarioRunner'
import { CLIENTS, CLIENT_COLORS } from '../types'
import type { HistoryPoint } from '../types'

interface Props {
  onBack: () => void
}

export default function DashboardPage({ onBack }: Props) {
  const statuses = useReplicas(1500)
  const [selectedIdx, setSelectedIdx] = useState(0)
  const [replayIndex, setReplayIndex] = useState<number | null>(null)
  const [history, setHistory] = useState<HistoryPoint[]>([])
  const [controlTab, setControlTab] = useState<'editor' | 'scenarios'>('editor')
  const historyRef = useRef<HistoryPoint[]>([])

  const selectedUrl = CLIENTS[selectedIdx].url
  const selectedColor = CLIENT_COLORS[selectedIdx]
  const liveGraph = useGraph(selectedUrl, 1500)
  const ops = useOps(selectedUrl, 1500)
  const { graph: replayGraph, loading: replayLoading } = useReplay(selectedUrl, replayIndex)

  useEffect(() => {
    const hasAny = statuses.some(s => s && !s.error)
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

  useEffect(() => { setReplayIndex(null) }, [selectedIdx])

  const online = statuses.filter(s => s && !s.error && s.stateHash)
  const allConverged = online.length > 1 && online.every(s => s!.stateHash === online[0]!.stateHash)
  const anyDiverged = online.length > 1 && !allConverged

  const opLogLen = statuses[selectedIdx]?.opLogLen ?? 0
  const displayGraph = replayIndex !== null && replayGraph ? replayGraph : liveGraph

  return (
    <div className="dashboard">
      {/* ── Header ──────────────────────────────────────────────────── */}
      <header className="dash-header">
        <div className="dash-header-left">
          <button className="btn-back" onClick={onBack}>← Overview</button>
          <div className="dash-divider" />
          <div className="dash-logo">
            <span className="dash-logo-icon">⬡</span>
            <span className="dash-logo-name">SentinelSync</span>
          </div>
        </div>
        <div className="dash-header-right">
          {online.length > 0 && (
            <div className={`conv-badge ${anyDiverged ? 'badge-diverged' : 'badge-converged'}`}>
              <span className="conv-dot" />
              {anyDiverged ? '⚠ Out of Sync' : '✓ All Synced'}
            </div>
          )}
          <span className="dash-online">{online.length}/{CLIENTS.length} browsers online</span>
        </div>
      </header>

      {/* ── Browser panels ──────────────────────────────────────────── */}
      <div className="replica-row">
        {CLIENTS.map((rep, i) => (
          <ReplicaPanel
            key={rep.id}
            url={rep.url}
            label={rep.label}
            status={statuses[i]}
            color={CLIENT_COLORS[i]}
            isSelected={selectedIdx === i}
            onSelect={() => setSelectedIdx(i)}
          />
        ))}
      </div>

      {/* ── Convergence chart ───────────────────────────────────────── */}
      <section className="chart-section">
        <div className="chart-section-header">
          <span>Sync History — Op Count per Browser</span>
          <span className="section-meta">Red band = browsers out of sync · {history.length} samples · 1.5 s interval</span>
        </div>
        <ConvergenceChart history={history} />
      </section>

      {/* ── Main grid (graph + timeline) ────────────────────────────── */}
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
              {CLIENTS.map((rep, i) => (
                <button
                  key={rep.id}
                  className={`tab ${selectedIdx === i ? 'tab-active' : ''}`}
                  style={selectedIdx === i ? { borderColor: CLIENT_COLORS[i], color: CLIENT_COLORS[i] } : {}}
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
            <span>Event Log</span>
            <span className="section-meta">{CLIENTS[selectedIdx].label} · operations received</span>
          </div>
          <Timeline ops={ops} />
        </section>
      </div>

      {/* ── Control center ──────────────────────────────────────────── */}
      <div className="control-center">
        <div className="control-center-header">
          <button
            className={`cc-tab ${controlTab === 'editor' ? 'cc-tab-active' : ''}`}
            onClick={() => setControlTab('editor')}
          >
            Graph Editor
          </button>
          <button
            className={`cc-tab ${controlTab === 'scenarios' ? 'cc-tab-active' : ''}`}
            onClick={() => setControlTab('scenarios')}
          >
            Scenarios
          </button>
          <span className="cc-tab-desc">
            {controlTab === 'editor'
              ? `Editing on ${CLIENTS[selectedIdx].label} — syncs to all other browsers automatically`
              : 'Automated scenarios — watch real-time sync, partition, and recovery'}
          </span>
        </div>

        {controlTab === 'editor' ? (
          <GraphEditor
            replicaUrl={selectedUrl}
            replicaLabel={CLIENTS[selectedIdx].label}
            graph={liveGraph}
          />
        ) : (
          <ScenarioRunner />
        )}
      </div>
    </div>
  )
}
