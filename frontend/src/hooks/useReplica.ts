import { useState, useEffect, useRef } from 'react'
import type { ReplicaStatus, GraphSnapshot, Operation } from '../types'
import { CLIENTS } from '../types'

export function useReplicas(interval = 1500) {
  const [statuses, setStatuses] = useState<(ReplicaStatus | null)[]>(
    CLIENTS.map(() => null),
  )

  useEffect(() => {
    const poll = async () => {
      const results = await Promise.all(
        CLIENTS.map(async ({ id, url }) => {
          try {
            const res = await fetch(`${url}/status`, { signal: AbortSignal.timeout(1000) })
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
        const res = await fetch(`${replicaUrl}/graph`, { signal: AbortSignal.timeout(1000) })
        if (!res.ok) return
        setGraph(await res.json())
      } catch {
        // keep stale graph
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
        const res = await fetch(`${replicaUrl}/ops`, { signal: AbortSignal.timeout(1000) })
        if (!res.ok) return
        setOps(await res.json())
      } catch {
        // keep stale list
      }
    }

    poll()
    const id = setInterval(poll, interval)
    return () => clearInterval(id)
  }, [replicaUrl, interval])

  return ops
}

export function useReplay(replicaUrl: string, index: number | null) {
  const [graph, setGraph] = useState<GraphSnapshot | null>(null)
  const [loading, setLoading] = useState(false)
  const cancelRef = useRef<boolean>(false)

  useEffect(() => {
    if (index === null) { setGraph(null); setLoading(false); return }
    cancelRef.current = false
    setLoading(true)
    const timer = setTimeout(async () => {
      try {
        const res = await fetch(`${replicaUrl}/replay?upto=${index}`, { signal: AbortSignal.timeout(2000) })
        if (!res.ok) throw new Error()
        const data = (await res.json()) as GraphSnapshot
        if (!cancelRef.current) { setGraph(data); setLoading(false) }
      } catch {
        if (!cancelRef.current) setLoading(false)
      }
    }, 150)
    return () => { cancelRef.current = true; clearTimeout(timer) }
  }, [replicaUrl, index])

  return { graph, loading }
}

// ── Mutation helpers ────────────────────────────────────────────────────────

export async function apiFetch(
  url: string,
  method: string,
  body?: object,
): Promise<unknown> {
  try {
    const res = await fetch(url, {
      method,
      headers: body ? { 'Content-Type': 'application/json' } : {},
      body: body ? JSON.stringify(body) : undefined,
    })
    const text = await res.text()
    return text ? JSON.parse(text) : null
  } catch {
    return null
  }
}

export const apiPost = (url: string, body?: object) => apiFetch(url, 'POST', body)
export const apiDelete = (url: string) => apiFetch(url, 'DELETE')
export const apiPatch = (url: string, body: object) => apiFetch(url, 'PATCH', body)

export const createNode = (baseUrl: string, id: string, title: string, x: number, y: number) =>
  apiPost(`${baseUrl}/node`, { id, title, x, y })

export const deleteNode = (baseUrl: string, id: string) =>
  apiDelete(`${baseUrl}/node/${id}`)

export const renameNode = (baseUrl: string, id: string, title: string) =>
  apiPatch(`${baseUrl}/node/${id}/title`, { title })

export const createEdge = (baseUrl: string, id: string, source: string, target: string) =>
  apiPost(`${baseUrl}/edge`, { id, source, target })

export const deleteEdge = (baseUrl: string, id: string) =>
  apiDelete(`${baseUrl}/edge/${id}`)
