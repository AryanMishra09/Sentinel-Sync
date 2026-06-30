import type { Operation } from '../types'

const OP_CLASS: Record<string, string> = {
  create_node: 'op-green',
  rename_node: 'op-blue',
  move_node: 'op-blue',
  delete_node: 'op-red',
  create_edge: 'op-green',
  delete_edge: 'op-red',
}

function fmtType(t: string) {
  return t.replace(/_/g, ' ')
}

function fmtTime(physicalMs: number) {
  if (!physicalMs) return '—'
  const d = new Date(physicalMs)
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

interface Props {
  ops: Operation[]
}

export default function Timeline({ ops }: Props) {
  if (ops.length === 0) {
    return <div className="timeline-empty">No operations yet</div>
  }

  return (
    <div className="timeline">
      {ops.slice(0, 40).map((op) => (
        <div key={op.id} className={`timeline-row ${OP_CLASS[op.type] ?? ''}`}>
          <span className="tl-time">{fmtTime(op.hlc.physical)}</span>
          <span className="tl-type">{fmtType(op.type)}</span>
          <span className="tl-replica">
            {op.replicaId === 'replica-a' ? 'Browser A' : op.replicaId === 'replica-b' ? 'Browser B' : 'Browser C'}
          </span>
          <span className="tl-id mono">{op.id}</span>
        </div>
      ))}
    </div>
  )
}
