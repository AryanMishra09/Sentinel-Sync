export interface ReplicaStatus {
  replicaId: string
  peers: { id: string; address: string }[]
  nodes: number
  edges: number
  vectorClock: Record<string, number>
  opLogLen: number
  tombstones: number
  stateHash: string
  chaos: {
    latencyMs: number
    lossRate: number
    isolated: boolean
  }
  sim: {
    running: boolean
    users: number
    opsPerSec: number
    totalOps: number
  }
  error?: string
}

export interface GraphNode {
  id: string
  title: string
  x: number
  y: number
  createdAt: number
}

export interface GraphEdge {
  id: string
  source: string
  target: string
}

export interface GraphSnapshot {
  nodes: GraphNode[]
  edges: GraphEdge[]
}

export interface HLCTimestamp {
  physical: number
  logical: number
  replicaId: string
}

export interface Operation {
  id: string
  replicaId: string
  type: string
  hlc: HLCTimestamp
  vectorClock: Record<string, number>
}

export interface HistoryPoint {
  nodes: [number, number, number]
  hashes: [string, string, string]
}

export const REPLICAS = [
  { id: 'replica-a', label: 'Replica A', url: 'http://localhost:8080' },
  { id: 'replica-b', label: 'Replica B', url: 'http://localhost:8081' },
  { id: 'replica-c', label: 'Replica C', url: 'http://localhost:8082' },
]

export const REPLICA_COLORS = ['#3b82f6', '#22c55e', '#f97316'] // blue, green, orange
