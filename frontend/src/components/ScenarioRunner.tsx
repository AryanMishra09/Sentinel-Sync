import { useState, useRef } from 'react'
import { apiPost } from '../hooks/useReplica'
import { CLIENTS } from '../types'

type StepStatus = 'pending' | 'running' | 'done' | 'error'

interface Step {
  label: string
  run: () => Promise<void>
}

interface Scenario {
  id: string
  title: string
  desc: string
  steps: Step[]
}

const sleep = (ms: number) => new Promise<void>(r => setTimeout(r, ms))

function buildScenarios(): Scenario[] {
  const [a, b, c] = CLIENTS.map(r => r.url)
  return [
    {
      id: 'partition',
      title: 'Network Partition',
      desc: 'Browsers A and C keep editing. Browser B is disconnected. Watch the sync chart diverge, then reconnect B and watch it automatically catch up.',
      steps: [
        { label: 'Start virtual users on all 3 browsers (10 users, 3 ops/s)', run: async () => {
          await Promise.all([a, b, c].map(url => apiPost(`${url}/sim/users/start`, { users: 10, opsPerSec: 3 })))
        }},
        { label: 'Wait 3 s — browsers accumulate changes', run: () => sleep(3000) },
        { label: 'Disconnect Browser B (simulate network partition)', run: () => apiPost(`${b}/sim/isolate`).then(() => void 0) },
        { label: 'Wait 5 s — watch Browser B diverge in the chart', run: () => sleep(5000) },
        { label: 'Reconnect Browser B', run: () => apiPost(`${b}/sim/recover`).then(() => void 0) },
        { label: 'Wait 4 s — sync engine reconciles all changes', run: () => sleep(4000) },
        { label: 'Stop all virtual users', run: async () => {
          await Promise.all([a, b, c].map(url => apiPost(`${url}/sim/users/stop`)))
        }},
      ],
    },
    {
      id: 'loss',
      title: 'Packet Loss Recovery',
      desc: 'Drop 60% of Browser A\'s outgoing sync messages. Changes still reach all browsers eventually — the sync engine fills in the gaps automatically.',
      steps: [
        { label: 'Start virtual users on Browser A (5 users, 4 ops/s)', run: () => apiPost(`${a}/sim/users/start`, { users: 5, opsPerSec: 4 }).then(() => void 0) },
        { label: 'Drop 60% of Browser A\'s sync messages', run: () => apiPost(`${a}/sim/loss`, { rate: 0.6 }).then(() => void 0) },
        { label: 'Wait 5 s — ops dropping in flight', run: () => sleep(5000) },
        { label: 'Restore full connectivity', run: () => apiPost(`${a}/sim/loss`, { rate: 0 }).then(() => void 0) },
        { label: 'Wait 4 s — anti-entropy fills the gaps', run: () => sleep(4000) },
        { label: 'Stop virtual users', run: () => apiPost(`${a}/sim/users/stop`).then(() => void 0) },
      ],
    },
    {
      id: 'load',
      title: 'High-Load Stress Test',
      desc: '60 virtual users editing concurrently across all 3 browsers with 200ms network delay. When load stops, all browsers converge to identical state.',
      steps: [
        { label: 'Start 20 virtual users per browser (8 ops/s each)', run: async () => {
          await Promise.all([a, b, c].map(url => apiPost(`${url}/sim/users/start`, { users: 20, opsPerSec: 8 })))
        }},
        { label: 'Add 200 ms network delay to all browsers', run: async () => {
          await Promise.all([a, b, c].map(url => apiPost(`${url}/sim/latency`, { ms: 200 })))
        }},
        { label: 'Wait 8 s — all browsers editing under latency', run: () => sleep(8000) },
        { label: 'Clear network delay', run: async () => {
          await Promise.all([a, b, c].map(url => apiPost(`${url}/sim/latency`, { ms: 0 })))
        }},
        { label: 'Wait 4 s — final convergence', run: () => sleep(4000) },
        { label: 'Stop all virtual users', run: async () => {
          await Promise.all([a, b, c].map(url => apiPost(`${url}/sim/users/stop`)))
        }},
      ],
    },
  ]
}

interface ScenarioState {
  status: 'idle' | 'running' | 'done' | 'error'
  steps: StepStatus[]
  currentStep: number
}

const initState = (count: number): ScenarioState => ({
  status: 'idle',
  steps: Array(count).fill('pending') as StepStatus[],
  currentStep: -1,
})

export default function ScenarioRunner() {
  const scenarios = buildScenarios()
  const [states, setStates] = useState<Record<string, ScenarioState>>(
    Object.fromEntries(scenarios.map(s => [s.id, initState(s.steps.length)])),
  )
  const cancelRefs = useRef<Record<string, boolean>>({})

  const update = (id: string, patch: Partial<ScenarioState> | ((prev: ScenarioState) => ScenarioState)) => {
    setStates(prev => ({
      ...prev,
      [id]: typeof patch === 'function' ? patch(prev[id]) : { ...prev[id], ...patch },
    }))
  }

  const run = async (scenario: Scenario) => {
    const { id, steps } = scenario
    cancelRefs.current[id] = false
    const stepStatuses: StepStatus[] = Array(steps.length).fill('pending')

    update(id, { status: 'running', steps: [...stepStatuses], currentStep: 0 })

    for (let i = 0; i < steps.length; i++) {
      if (cancelRefs.current[id]) break
      stepStatuses[i] = 'running'
      update(id, { steps: [...stepStatuses], currentStep: i })
      try {
        await steps[i].run()
        stepStatuses[i] = 'done'
      } catch {
        stepStatuses[i] = 'error'
        update(id, { steps: [...stepStatuses], status: 'error', currentStep: -1 })
        return
      }
      update(id, { steps: [...stepStatuses] })
    }

    update(id, { status: cancelRefs.current[id] ? 'idle' : 'done', currentStep: -1 })
  }

  const stop = (id: string) => {
    cancelRefs.current[id] = true
  }

  const reset = (id: string, count: number) => {
    cancelRefs.current[id] = true
    update(id, initState(count))
  }

  return (
    <div className="scenario-runner">
      <div className="scenarios-row">
        {scenarios.map(scenario => {
          const state = states[scenario.id]
          const progress = state.steps.filter(s => s === 'done').length / state.steps.length * 100
          const isRunning = state.status === 'running'
          const isDone = state.status === 'done'

          return (
            <div
              key={scenario.id}
              className={`scenario-card-dash ${isRunning ? 'sc-running' : ''} ${isDone ? 'sc-done' : ''}`}
            >
              <div className="sc-header">
                <div className="sc-title">{scenario.title}</div>
              </div>
              <div className="sc-desc">{scenario.desc}</div>

              <div className="sc-steps">
                {scenario.steps.map((step, i) => {
                  const s = state.steps[i]
                  return (
                    <div key={i} className={`sc-step step-${s}`}>
                      <span className="sc-step-icon">
                        {s === 'pending' ? '○' : s === 'running' ? <span className="sc-spin">⟳</span> : s === 'done' ? '✓' : '✗'}
                      </span>
                      {step.label}
                    </div>
                  )
                })}
              </div>

              <div className="sc-progress">
                <div
                  className={`sc-progress-bar ${isDone ? 'sc-bar-done' : ''}`}
                  style={{ width: `${progress}%` }}
                />
              </div>

              <div style={{ display: 'flex', gap: 6 }}>
                {!isRunning ? (
                  <button
                    className={`btn-sc-run ${isDone ? 'sc-complete' : 'sc-idle'}`}
                    onClick={() => isDone ? reset(scenario.id, scenario.steps.length) : run(scenario)}
                  >
                    {isDone ? '↺ Run Again' : '▶ Run Scenario'}
                  </button>
                ) : (
                  <button
                    className="btn-sc-run sc-running"
                    onClick={() => stop(scenario.id)}
                    style={{ flex: 1 }}
                  >
                    ■ Stop
                  </button>
                )}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
