import { useState, useEffect, useRef } from 'react'
import type { ReplicaStatus, GraphSnapshot, Operation } from '../types'
import { REPLICAS } from '../types'

export function useReplicas(interval = 1500) {
  const [statuses, setStatuses] = useState<(ReplicaStatus | null)[]>(
    REPLICAS.map(() => null),
  )

  useEffect(() => {
    const poll = async () => {
      const results = await Promise.all(
        REPLICAS.map(async ({ id, url }) => {
          try {
            const res = await fetch(`${url}/status`, {
              signal: AbortSignal.timeout(1000),
            })
            if (!res.ok) throw new Error(`HTTP ${res.status}`)
            return (await res.json()) as ReplicaStatus
          } catch (e) {
            return { replicaId: id, error: String(e) } as ReplicaStatus
          }
        }),
      )
      setStatuses(results)
    }

    poll()
    const id = setInterval(poll, interval)
    return () => clearInterval(id)
  }, [interval])

  return statuses
}

export function useGraph(replicaUrl: string, interval = 1500) {
  const [graph, setGraph] = useState<GraphSnapshot>({ nodes: [], edges: [] })

  useEffect(() => {
    const poll = async () => {
      try {
        const res = await fetch(`${replicaUrl}/graph`, {
          signal: AbortSignal.timeout(1000),
        })
        if (!res.ok) return
        setGraph(await res.json())
      } catch {
        // replica offline — keep stale graph
      }
    }

    poll()
    const id = setInterval(poll, interval)
    return () => clearInterval(id)
  }, [replicaUrl, interval])

  return graph
}

export function useOps(replicaUrl: string, interval = 1500) {
  const [ops, setOps] = useState<Operation[]>([])

  useEffect(() => {
    const poll = async () => {
      try {
        const res = await fetch(`${replicaUrl}/ops`, {
          signal: AbortSignal.timeout(1000),
        })
        if (!res.ok) return
        setOps(await res.json())
      } catch {
        // replica offline — keep stale list
      }
    }

    poll()
    const id = setInterval(poll, interval)
    return () => clearInterval(id)
  }, [replicaUrl, interval])

  return ops
}

// useReplay fetches GET /replay?upto=<index> with a 150 ms debounce so dragging
// the scrubber quickly does not flood the backend. Returns null while loading or
// when index is null (live mode).
export function useReplay(replicaUrl: string, index: number | null) {
  const [graph, setGraph] = useState<GraphSnapshot | null>(null)
  const [loading, setLoading] = useState(false)
  const cancelRef = useRef<boolean>(false)

  useEffect(() => {
    if (index === null) {
      setGraph(null)
      setLoading(false)
      return
    }

    cancelRef.current = false
    setLoading(true)

    const timer = setTimeout(async () => {
      try {
        const res = await fetch(`${replicaUrl}/replay?upto=${index}`, {
          signal: AbortSignal.timeout(2000),
        })
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
        const data = (await res.json()) as GraphSnapshot
        if (!cancelRef.current) {
          setGraph(data)
          setLoading(false)
        }
      } catch {
        if (!cancelRef.current) setLoading(false)
      }
    }, 150)

    return () => {
      cancelRef.current = true
      clearTimeout(timer)
    }
  }, [replicaUrl, index])

  return { graph, loading }
}

// api — fire a POST and return the response body (or null on error).
export async function apiPost(url: string, body?: object): Promise<unknown> {
  try {
    const res = await fetch(url, {
      method: 'POST',
      headers: body ? { 'Content-Type': 'application/json' } : {},
      body: body ? JSON.stringify(body) : undefined,
    })
    if (!res.ok) return null
    return await res.json()
  } catch {
    return null
  }
}
