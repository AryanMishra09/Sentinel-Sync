import type { HistoryPoint } from '../types'
import { REPLICA_COLORS } from '../types'

interface Props {
  history: HistoryPoint[]
}

function isDiverged(h: HistoryPoint): boolean {
  const active = h.hashes.filter((hash) => hash !== '')
  return active.length > 1 && !active.every((hash) => hash === active[0])
}

export default function ConvergenceChart({ history }: Props) {
  if (history.length === 0) {
    return (
      <div className="chart-empty">Collecting convergence data...</div>
    )
  }

  const W = 800
  const H = 60
  const PAD_T = 8
  const PAD_B = 4

  const allNodes = history.flatMap((p) => p.nodes)
  const maxNodes = Math.max(...allNodes, 1)

  const xOf = (i: number) =>
    history.length < 2 ? W / 2 : (i / (history.length - 1)) * W

  const yOf = (n: number) =>
    PAD_T + H - (n / maxNodes) * H

  // Build polyline points per replica
  const lines = [0, 1, 2].map((ri) =>
    history.map((p, i) => `${xOf(i).toFixed(1)},${yOf(p.nodes[ri]).toFixed(1)}`).join(' '),
  )

  // Divergence bands: contiguous ranges of diverged points → rects
  const bands: { x1: number; x2: number }[] = []
  let bandStart: number | null = null
  for (let i = 0; i < history.length; i++) {
    if (isDiverged(history[i])) {
      if (bandStart === null) bandStart = i
    } else {
      if (bandStart !== null) {
        bands.push({ x1: xOf(bandStart), x2: xOf(i - 1) })
        bandStart = null
      }
    }
  }
  if (bandStart !== null) {
    bands.push({ x1: xOf(bandStart), x2: xOf(history.length - 1) })
  }

  const totalH = PAD_T + H + PAD_B

  return (
    <div className="convergence-chart">
      <div className="chart-label">Nodes over time</div>
      <svg
        viewBox={`0 0 ${W} ${totalH}`}
        preserveAspectRatio="none"
        className="chart-svg"
      >
        {/* divergence bands */}
        {bands.map((b, i) => (
          <rect
            key={i}
            x={b.x1}
            y={0}
            width={Math.max(b.x2 - b.x1, 4)}
            height={totalH}
            fill="#ef444418"
          />
        ))}

        {/* zero line */}
        <line
          x1={0}
          y1={yOf(0)}
          x2={W}
          y2={yOf(0)}
          stroke="#252840"
          strokeWidth={1}
        />

        {/* series lines */}
        {lines.map((pts, ri) => (
          <polyline
            key={ri}
            points={pts}
            fill="none"
            stroke={REPLICA_COLORS[ri]}
            strokeWidth={1.5}
            strokeLinejoin="round"
            strokeLinecap="round"
            opacity={0.85}
          />
        ))}

        {/* dots for latest value */}
        {[0, 1, 2].map((ri) => {
          const last = history[history.length - 1]
          return (
            <circle
              key={ri}
              cx={xOf(history.length - 1)}
              cy={yOf(last.nodes[ri])}
              r={3}
              fill={REPLICA_COLORS[ri]}
            />
          )
        })}
      </svg>

      <div className="chart-legend">
        {['A', 'B', 'C'].map((label, i) => (
          <span key={i} className="chart-legend-item">
            <span className="chart-legend-dot" style={{ background: REPLICA_COLORS[i] }} />
            {label}
          </span>
        ))}
        {bands.length > 0 && (
          <span className="chart-legend-item chart-legend-diverged">
            <span className="chart-legend-dot" style={{ background: '#ef4444' }} />
            diverged
          </span>
        )}
      </div>
    </div>
  )
}
