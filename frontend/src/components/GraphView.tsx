import { useEffect } from 'react'
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  useNodesState,
  useEdgesState,
  type Node,
  type Edge,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import type { GraphSnapshot } from '../types'

type AppNode = Node<{ label: string }>
type AppEdge = Edge

interface Props {
  graph: GraphSnapshot
  color: string
}

export default function GraphView({ graph, color }: Props) {
  const [nodes, setNodes, onNodesChange] = useNodesState<AppNode>([])
  const [edges, setEdges, onEdgesChange] = useEdgesState<AppEdge>([])

  useEffect(() => {
    setNodes(
      graph.nodes.map(n => ({
        id: n.id,
        position: { x: n.x, y: n.y },
        data: { label: n.title || n.id },
        style: {
          background: '#ffffff',
          border: `1.5px solid ${color}50`,
          color: '#0f172a',
          borderRadius: 8,
          fontSize: 11,
          padding: '5px 12px',
          boxShadow: `0 2px 8px rgba(0,0,0,0.08), 0 0 0 0px ${color}`,
        },
      })),
    )
    setEdges(
      graph.edges.map(e => ({
        id: e.id,
        source: e.source,
        target: e.target,
        style: { stroke: color + '80', strokeWidth: 1.5 },
      })),
    )
  }, [graph, color, setNodes, setEdges])

  if (graph.nodes.length === 0) {
    return (
      <div className="graph-empty">
        No nodes — create one in Graph Editor below, or run a Scenario
      </div>
    )
  }

  return (
    <ReactFlow
      nodes={nodes}
      edges={edges}
      onNodesChange={onNodesChange}
      onEdgesChange={onEdgesChange}
      fitView
      fitViewOptions={{ padding: 0.25 }}
      colorMode="light"
      proOptions={{ hideAttribution: true }}
    >
      <Background color="rgba(148,163,184,0.2)" gap={24} />
      <Controls />
      <MiniMap
        nodeColor={color + '99'}
        maskColor="rgba(241,245,249,0.85)"
        style={{ background: '#ffffff', border: '1px solid #e2e8f0' }}
      />
    </ReactFlow>
  )
}
