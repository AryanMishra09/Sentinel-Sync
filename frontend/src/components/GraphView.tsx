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
      graph.nodes.map((n) => ({
        id: n.id,
        position: { x: n.x, y: n.y },
        data: { label: n.title || n.id },
        style: {
          background: '#1e2235',
          border: `1px solid ${color}55`,
          color: '#e2e8f0',
          borderRadius: 8,
          fontSize: 12,
          padding: '4px 10px',
        },
      })),
    )
    setEdges(
      graph.edges.map((e) => ({
        id: e.id,
        source: e.source,
        target: e.target,
        style: { stroke: color + '88' },
      })),
    )
  }, [graph, color, setNodes, setEdges])

  if (graph.nodes.length === 0) {
    return (
      <div className="graph-empty">
        No nodes — start the sim or POST /node to create some
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
      fitViewOptions={{ padding: 0.2 }}
      colorMode="dark"
      proOptions={{ hideAttribution: true }}
    >
      <Background color="#2a2d3e" />
      <Controls />
      <MiniMap
        nodeColor={color + 'aa'}
        maskColor="#0f111880"
        style={{ background: '#1a1d2e' }}
      />
    </ReactFlow>
  )
}
