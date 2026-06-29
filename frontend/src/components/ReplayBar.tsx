interface Props {
  opLogLen: number
  index: number | null
  loading: boolean
  onChange: (i: number | null) => void
}

export default function ReplayBar({ opLogLen, index, loading, onChange }: Props) {
  const max = Math.max(opLogLen - 1, 0)
  const isReplay = index !== null

  return (
    <div className={`replay-bar ${isReplay ? 'replay-active' : ''}`}>
      <button
        className={`btn ${isReplay ? 'btn-red' : 'btn-ghost'} btn-live`}
        onClick={() => onChange(null)}
        disabled={!isReplay}
        title="Return to live view"
      >
        {isReplay ? '● LIVE' : '● LIVE'}
      </button>

      <input
        type="range"
        className="replay-slider"
        min={0}
        max={max}
        value={index ?? max}
        disabled={opLogLen === 0}
        onChange={(e) => onChange(Number(e.target.value))}
      />

      <span className="replay-counter mono">
        {isReplay
          ? `op ${index! + 1} / ${opLogLen}`
          : opLogLen === 0
          ? 'no ops'
          : `${opLogLen} ops`}
      </span>

      {loading && <span className="replay-spinner">⟳</span>}

      {isReplay && (
        <span className="badge badge-red replay-badge">REPLAY</span>
      )}
    </div>
  )
}
