import { useState } from 'react'
import type { GraphSnapshot } from '../types'
import { createNode, deleteNode, createEdge, deleteEdge } from '../hooks/useReplica'

interface Props {
  replicaUrl: string
  replicaLabel: string
  graph: GraphSnapshot
}

function genId(prefix: string) {
  return `${prefix}-${Math.random().toString(36).slice(2, 7)}`
}

function randCoord() {
  return Math.floor(Math.random() * 600 + 50)
}

export default function GraphEditor({ replicaUrl, replicaLabel, graph }: Props) {
  // Node form
  const [nodeId, setNodeId] = useState(genId('n'))
  const [nodeTitle, setNodeTitle] = useState('')
  const [nodeX, setNodeX] = useState(String(randCoord()))
  const [nodeY, setNodeY] = useState(String(randCoord()))
  const [nodeMsg, setNodeMsg] = useState<{ text: string; ok: boolean } | null>(null)
  const [nodeLoading, setNodeLoading] = useState(false)

  // Edge form
  const [edgeId, setEdgeId] = useState(genId('e'))
  const [edgeSource, setEdgeSource] = useState('')
  const [edgeTarget, setEdgeTarget] = useState('')
  const [edgeMsg, setEdgeMsg] = useState<{ text: string; ok: boolean } | null>(null)
  const [edgeLoading, setEdgeLoading] = useState(false)

  const handleCreateNode = async () => {
    if (!nodeId.trim() || !nodeTitle.trim()) return
    setNodeLoading(true)
    setNodeMsg(null)
    const res = await createNode(replicaUrl, nodeId.trim(), nodeTitle.trim(), Number(nodeX) || 0, Number(nodeY) || 0)
    if (res && (res as { id?: string }).id) {
      setNodeMsg({ text: `"${nodeTitle.trim()}" created — syncing to other browsers…`, ok: true })
      setNodeId(genId('n'))
      setNodeTitle('')
      setNodeX(String(randCoord()))
      setNodeY(String(randCoord()))
    } else {
      setNodeMsg({ text: 'Create failed — ID may already exist', ok: false })
    }
    setNodeLoading(false)
  }

  const handleDeleteNode = async (id: string) => {
    await deleteNode(replicaUrl, id)
  }

  const handleCreateEdge = async () => {
    if (!edgeId.trim() || !edgeSource || !edgeTarget) return
    setEdgeLoading(true)
    setEdgeMsg(null)
    const res = await createEdge(replicaUrl, edgeId.trim(), edgeSource, edgeTarget)
    if (res && (res as { id?: string }).id) {
      setEdgeMsg({ text: `Connection created — syncing to other browsers…`, ok: true })
      setEdgeId(genId('e'))
      setEdgeSource('')
      setEdgeTarget('')
    } else {
      setEdgeMsg({ text: 'Create failed — ID may exist or endpoints invalid', ok: false })
    }
    setEdgeLoading(false)
  }

  const handleDeleteEdge = async (id: string) => {
    await deleteEdge(replicaUrl, id)
  }

  const nodes = graph.nodes
  const edges = graph.edges

  return (
    <div className="graph-editor">
      {/* ── Create Node ─────────────────────────────────────────────── */}
      <div className="editor-panel">
        <div className="editor-panel-title">
          Create Node
          <span style={{ fontSize: 10, color: 'var(--text-dim)', fontWeight: 400 }}>on {replicaLabel}</span>
        </div>
        <div className="editor-form">
          <div className="editor-field">
            <label className="editor-label">Node ID</label>
            <input className="editor-input" value={nodeId} onChange={e => setNodeId(e.target.value)} placeholder="n-abc12" />
          </div>
          <div className="editor-field">
            <label className="editor-label">Title</label>
            <input className="editor-input" value={nodeTitle} onChange={e => setNodeTitle(e.target.value)} placeholder="Node title" />
          </div>
          <div className="editor-row-xy">
            <div className="editor-field">
              <label className="editor-label">X</label>
              <input className="editor-input" type="number" value={nodeX} onChange={e => setNodeX(e.target.value)} />
            </div>
            <div className="editor-field">
              <label className="editor-label">Y</label>
              <input className="editor-input" type="number" value={nodeY} onChange={e => setNodeY(e.target.value)} />
            </div>
          </div>
        </div>
        <button
          className="btn-editor-submit"
          disabled={nodeLoading || !nodeId.trim() || !nodeTitle.trim()}
          onClick={handleCreateNode}
        >
          {nodeLoading ? 'Creating…' : '+ Create Node'}
        </button>
        {nodeMsg && <div className={nodeMsg.ok ? 'editor-success' : 'editor-error'}>{nodeMsg.text}</div>}
      </div>

      {/* ── Nodes List ──────────────────────────────────────────────── */}
      <div className="editor-panel">
        <div className="editor-panel-title">
          Nodes
          <span className="editor-count">{nodes.length}</span>
        </div>
        {nodes.length === 0 ? (
          <div className="editor-empty">No nodes yet</div>
        ) : (
          <div className="editor-list">
            {nodes.map(n => (
              <div key={n.id} className="editor-item">
                <span className="editor-item-id">{n.id}</span>
                <span className="editor-item-title">{n.title || '—'}</span>
                <button className="editor-item-del" onClick={() => handleDeleteNode(n.id)}>✕</button>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* ── Edges ───────────────────────────────────────────────────── */}
      <div className="editor-panel">
        <div className="editor-panel-title">
          Edges
          <span className="editor-count">{edges.length}</span>
        </div>
        <div className="editor-form">
          <div className="editor-field">
            <label className="editor-label">Edge ID</label>
            <input className="editor-input" value={edgeId} onChange={e => setEdgeId(e.target.value)} placeholder="e-abc12" />
          </div>
          <div className="editor-field">
            <label className="editor-label">Source</label>
            <select className="editor-input" value={edgeSource} onChange={e => setEdgeSource(e.target.value)}>
              <option value="">— select node —</option>
              {nodes.map(n => <option key={n.id} value={n.id}>{n.id} ({n.title})</option>)}
            </select>
          </div>
          <div className="editor-field">
            <label className="editor-label">Target</label>
            <select className="editor-input" value={edgeTarget} onChange={e => setEdgeTarget(e.target.value)}>
              <option value="">— select node —</option>
              {nodes.map(n => <option key={n.id} value={n.id}>{n.id} ({n.title})</option>)}
            </select>
          </div>
        </div>
        <button
          className="btn-editor-submit"
          disabled={edgeLoading || !edgeId.trim() || !edgeSource || !edgeTarget}
          onClick={handleCreateEdge}
        >
          {edgeLoading ? 'Creating…' : '+ Create Edge'}
        </button>
        {edgeMsg && <div className={edgeMsg.ok ? 'editor-success' : 'editor-error'}>{edgeMsg.text}</div>}
        {edges.length > 0 && (
          <div className="editor-list" style={{ marginTop: 12 }}>
            {edges.map(e => (
              <div key={e.id} className="editor-item">
                <span className="editor-item-id">{e.source}</span>
                <span style={{ color: 'var(--text-dim)', fontSize: 10 }}>→</span>
                <span className="editor-item-id">{e.target}</span>
                <button className="editor-item-del" onClick={() => handleDeleteEdge(e.id)}>✕</button>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
