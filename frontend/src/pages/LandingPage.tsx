interface Props {
  onEnter: () => void
}

export default function LandingPage({ onEnter }: Props) {
  return (
    <div className="landing">
      {/* ── Nav ────────────────────────────────────────────────────── */}
      <nav className="landing-nav">
        <div className="nav-logo">
          <span className="nav-logo-icon">⬡</span>
          <span className="nav-logo-name">SentinelSync</span>
          <span className="nav-logo-tag">Go · CRDTs · React</span>
        </div>
        <div className="nav-cta">
          <a
            className="btn-nav-ghost"
            href="https://github.com/AryanMishra09/sentinel-sync"
            target="_blank"
            rel="noreferrer"
          >
            GitHub ↗
          </a>
          <button className="btn-nav-primary" onClick={onEnter}>
            Open Dashboard →
          </button>
        </div>
      </nav>

      {/* ── Hero ───────────────────────────────────────────────────── */}
      <section className="landing-hero">
        <div className="hero-bg" />
        <div className="hero-grid" />

        <div className="hero-badge">
          <span className="hero-badge-dot" />
          Distributed Systems · Eventual Consistency · CRDTs
        </div>

        <h1 className="hero-title">SentinelSync</h1>
        <p className="hero-subtitle">
          A real-time collaborative state synchronization engine. Multiple browsers
          independently edit shared data, stay available during network partitions,
          and automatically converge to identical state — proven by cryptographic hash.
        </p>

        <div className="hero-actions">
          <button className="btn-hero-primary" onClick={onEnter}>
            Open Dashboard →
          </button>
          <a
            className="btn-hero-ghost"
            href="https://github.com/AryanMishra09/sentinel-sync"
            target="_blank"
            rel="noreferrer"
          >
            View Source ↗
          </a>
        </div>

        {/* Stats bar */}
        <div className="hero-stats">
          <div className="hero-stat">
            <div className="hero-stat-val">3</div>
            <div className="hero-stat-lbl">Browsers</div>
          </div>
          <div className="hero-stat">
            <div className="hero-stat-val">8</div>
            <div className="hero-stat-lbl">Build Phases</div>
          </div>
          <div className="hero-stat">
            <div className="hero-stat-val">OR-Set</div>
            <div className="hero-stat-lbl">CRDT Type</div>
          </div>
          <div className="hero-stat">
            <div className="hero-stat-val">≤3s</div>
            <div className="hero-stat-lbl">Convergence</div>
          </div>
          <div className="hero-stat">
            <div className="hero-stat-val">HLC</div>
            <div className="hero-stat-lbl">Clock Type</div>
          </div>
        </div>

        {/* Network diagram */}
        <div className="hero-diagram">
          <NetworkDiagram />
        </div>
      </section>

      {/* ── Concepts ───────────────────────────────────────────────── */}
      <div className="landing-section-full">
        <div style={{ maxWidth: 1100, margin: '0 auto' }}>
          <div className="section-eyebrow">Core Concepts</div>
          <h2 className="section-title">The same techniques used by Google Docs, Figma, and Notion</h2>
          <p className="section-desc">
            Every design decision maps to a real-world collaborative system constraint. No shortcuts.
          </p>
          <div className="concepts-grid">
            <div className="concept-card" style={{ '--card-accent': '#6366f1' } as React.CSSProperties}>
              <span className="concept-icon">⊕</span>
              <div className="concept-title">Add-Wins OR-Set</div>
              <div className="concept-body">
                Node and edge presence is modelled as an Observed-Remove Set. A concurrent
                create + delete resolves to present — add-wins. Each create tags the item
                with a unique <code>(replicaID, counter)</code> pair; deletes name specific tags.
              </div>
              <span className="concept-tag">CRDT</span>
            </div>

            <div className="concept-card" style={{ '--card-accent': '#059669' } as React.CSSProperties}>
              <span className="concept-icon">⏱</span>
              <div className="concept-title">Hybrid Logical Clock</div>
              <div className="concept-body">
                Title and position use Last-Write-Wins ordered by HLC timestamps
                <code>(physicalMs, logical, replicaID)</code>. Unlike raw wall-clock, an HLC
                advances monotonically even if the system clock goes backward, and its
                tiebreaker makes the order deterministic for simultaneous writes.
              </div>
              <span className="concept-tag">Ordering</span>
            </div>

            <div className="concept-card" style={{ '--card-accent': '#d97706' } as React.CSSProperties}>
              <span className="concept-icon">⊗</span>
              <div className="concept-title">Gap-Aware Vector Clock</div>
              <div className="concept-body">
                Each replica tracks the highest <em>contiguous</em> sequence seen from every
                peer — not the max. If op #3 is dropped but #4 arrives, the clock stays at
                2. Anti-entropy then re-requests #3 rather than silently skipping the gap.
              </div>
              <span className="concept-tag">Causality</span>
            </div>

            <div className="concept-card" style={{ '--card-accent': '#f472b6' } as React.CSSProperties}>
              <span className="concept-icon">↺</span>
              <div className="concept-title">Anti-Entropy Reconciliation</div>
              <div className="concept-body">
                Every 3 s, replicas exchange vector clocks and replay any ops the peer has
                not seen. This is the reliability backstop when broadcast drops packets.
                Operations are idempotent by ID — duplicate delivery is harmless.
              </div>
              <span className="concept-tag">Reliability</span>
            </div>

            <div className="concept-card" style={{ '--card-accent': '#60a5fa' } as React.CSSProperties}>
              <span className="concept-icon">#</span>
              <div className="concept-title">Convergence Hash</div>
              <div className="concept-body">
                <code>stateHash</code> is SHA-256 of the canonicalized materialized graph.
                Equal hash on two replicas is a <em>proof</em> of identical state — not just
                a likely indicator. The hash is computed after sorting all node/edge IDs so
                the same logical state always produces the same hash.
              </div>
              <span className="concept-tag">Proof</span>
            </div>

            <div className="concept-card" style={{ '--card-accent': '#a78bfa' } as React.CSSProperties}>
              <span className="concept-icon">⚡</span>
              <div className="concept-title">WebSocket Broadcast</div>
              <div className="concept-body">
                Every locally generated operation is immediately broadcast to all connected
                peers over a persistent WebSocket connection — the fast path. Connections
                survive partition simulation because isolation is implemented by dropping
                messages, not closing sockets, so recovery is instant.
              </div>
              <span className="concept-tag">Transport</span>
            </div>
          </div>
        </div>
      </div>

      {/* ── How it works ───────────────────────────────────────────── */}
      <div style={{ borderTop: '1px solid var(--border-dim)', padding: '80px 40px' }}>
        <div style={{ maxWidth: 1100, margin: '0 auto', display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 60, alignItems: 'start' }}>
          <div>
            <div className="section-eyebrow">How It Works</div>
            <h2 className="section-title">From write to convergence</h2>
            <p className="section-desc">
              A write to any replica propagates to the cluster through two independent mechanisms.
              Even under packet loss and network partitions, every op eventually reaches every replica.
            </p>
          </div>
          <div className="how-steps">
            <div className="how-step">
              <div className="how-step-num">1</div>
              <div className="how-step-content">
                <div className="how-step-title">Browser makes a change</div>
                <div className="how-step-desc">Any browser accepts any edit immediately. No lock, no round-trip to a coordinator required.</div>
                <code className="how-step-code">POST /node {"{"}"id":"n1","title":"Start"{"}"}</code>
              </div>
            </div>
            <div className="how-step">
              <div className="how-step-num">2</div>
              <div className="how-step-content">
                <div className="how-step-title">Sync engine emits an operation</div>
                <div className="how-step-desc">The change becomes an immutable Operation with an HLC timestamp, vector clock snapshot, and unique ID.</div>
              </div>
            </div>
            <div className="how-step">
              <div className="how-step-num">3</div>
              <div className="how-step-content">
                <div className="how-step-title">Broadcast to all other browsers (fast path)</div>
                <div className="how-step-desc">The operation is sent to all connected browsers over WebSocket immediately — ~1ms propagation.</div>
              </div>
            </div>
            <div className="how-step">
              <div className="how-step-num">4</div>
              <div className="how-step-content">
                <div className="how-step-title">Anti-entropy heals any gaps</div>
                <div className="how-step-desc">Every 3 s, replicas compare vector clocks and replay any missing ops. Works through any amount of packet loss.</div>
              </div>
            </div>
            <div className="how-step">
              <div className="how-step-num">5</div>
              <div className="how-step-content">
                <div className="how-step-title">State hashes match — convergence proven</div>
                <div className="how-step-desc">Once all browsers hold the same ops, SHA-256 of their state is identical. Equal hash = mathematically proven sync.</div>
                <code className="how-step-code">stateHash: a3f8c1... on all 3 browsers</code>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* ── Demo Scenarios ─────────────────────────────────────────── */}
      <div className="landing-section-full">
        <div style={{ maxWidth: 1100, margin: '0 auto' }}>
          <div className="section-eyebrow">Demo Scenarios</div>
          <h2 className="section-title">Run from the dashboard</h2>
          <p className="section-desc">Every scenario is one-click in the dashboard — no curl commands required.</p>
          <div className="scenarios-grid">
            <div className="scenario-card" onClick={onEnter}>
              <div className="scenario-card-icon">⚡</div>
              <div className="scenario-card-title">Network Partition</div>
              <div className="scenario-card-desc">
                Disconnect Browser B mid-edit. Watch the sync chart diverge with a red band.
                Reconnect — the sync engine heals the gap automatically in ≤3 s.
              </div>
              <div className="scenario-card-cta">Run this scenario →</div>
            </div>
            <div className="scenario-card" onClick={onEnter}>
              <div className="scenario-card-icon">📦</div>
              <div className="scenario-card-title">Packet Loss Recovery</div>
              <div className="scenario-card-desc">
                Drop 60% of Browser A's sync messages mid-session.
                Restore connectivity — every missed change is replayed and merged automatically.
              </div>
              <div className="scenario-card-cta">Run this scenario →</div>
            </div>
            <div className="scenario-card" onClick={onEnter}>
              <div className="scenario-card-icon">🔥</div>
              <div className="scenario-card-title">High-Load Stress Test</div>
              <div className="scenario-card-desc">
                60 virtual users editing concurrently with 200 ms delay.
                Stop the load — all 3 browser hashes collapse to one value in seconds.
              </div>
              <div className="scenario-card-cta">Run this scenario →</div>
            </div>
          </div>
        </div>
      </div>

      {/* ── Footer ─────────────────────────────────────────────────── */}
      <footer className="landing-footer">
        <div className="footer-logo">
          <span style={{ color: 'var(--accent)' }}>⬡</span>
          SentinelSync
        </div>
        <div className="footer-links">
          <a className="footer-link" href="https://github.com/AryanMishra09/sentinel-sync" target="_blank" rel="noreferrer">GitHub</a>
          <button className="footer-link" style={{ background: 'none', border: 'none', cursor: 'pointer' }} onClick={onEnter}>Dashboard</button>
        </div>
      </footer>
    </div>
  )
}

/* ── Animated network diagram ──────────────────────────────────────────── */
function NetworkDiagram() {
  return (
    <svg viewBox="0 0 760 300" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
      <defs>
        {/* Connection paths */}
        <path id="ab" d="M 195 130 L 565 130" />
        <path id="ba" d="M 565 130 L 195 130" />
        <path id="ac" d="M 175 155 L 355 255" />
        <path id="ca" d="M 355 255 L 175 155" />
        <path id="bc" d="M 585 155 L 405 255" />
        <path id="cb" d="M 405 255 L 585 155" />
      </defs>

      {/* Connection lines */}
      {(['M 195 130 L 565 130', 'M 175 155 L 355 255', 'M 585 155 L 405 255'] as const).map((d, i) => (
        <path key={i} d={d} stroke="rgba(99,102,241,0.2)" strokeWidth="1.5" strokeDasharray="5 4" />
      ))}

      {/* Animated operation packets — A→B */}
      {[0, 1.2].map((delay, i) => (
        <circle key={`ab${i}`} r="4" fill="#6366f1">
          <animateMotion dur="2.4s" repeatCount="indefinite" begin={`${delay}s`}>
            <mpath href="#ab" />
          </animateMotion>
          <animate attributeName="opacity" values="0;1;1;0" dur="2.4s" repeatCount="indefinite" begin={`${delay}s`} />
        </circle>
      ))}

      {/* B→A (emerald) */}
      <circle r="3.5" fill="#059669">
        <animateMotion dur="2.4s" repeatCount="indefinite" begin="0.8s">
          <mpath href="#ba" />
        </animateMotion>
        <animate attributeName="opacity" values="0;1;1;0" dur="2.4s" repeatCount="indefinite" begin="0.8s" />
      </circle>

      {/* A→C */}
      <circle r="3.5" fill="#6366f1">
        <animateMotion dur="2.1s" repeatCount="indefinite" begin="0.4s">
          <mpath href="#ac" />
        </animateMotion>
        <animate attributeName="opacity" values="0;1;1;0" dur="2.1s" repeatCount="indefinite" begin="0.4s" />
      </circle>

      {/* B→C (amber) */}
      {[0, 1.0].map((delay, i) => (
        <circle key={`bc${i}`} r="3.5" fill="#d97706">
          <animateMotion dur="2.1s" repeatCount="indefinite" begin={`${delay}s`}>
            <mpath href="#bc" />
          </animateMotion>
          <animate attributeName="opacity" values="0;1;1;0" dur="2.1s" repeatCount="indefinite" begin={`${delay}s`} />
        </circle>
      ))}

      {/* C→A */}
      <circle r="3.5" fill="#059669">
        <animateMotion dur="2.1s" repeatCount="indefinite" begin="1.5s">
          <mpath href="#ca" />
        </animateMotion>
        <animate attributeName="opacity" values="0;1;1;0" dur="2.1s" repeatCount="indefinite" begin="1.5s" />
      </circle>

      {/* ── Browser A ─────────────────────────────── */}
      <circle cx="180" cy="130" r="42" fill="rgba(99,102,241,0.07)" stroke="rgba(99,102,241,0.35)" strokeWidth="1.5" />
      <circle cx="180" cy="130" r="34" fill="rgba(99,102,241,0.04)" />
      <text x="180" y="125" textAnchor="middle" fill="#6366f1" fontSize="15" fontWeight="700" fontFamily="system-ui">A</text>
      <text x="180" y="143" textAnchor="middle" fill="rgba(99,102,241,0.55)" fontSize="9" fontFamily="system-ui">Browser A</text>
      <text x="180" y="190" textAnchor="middle" fill="rgba(99,102,241,0.35)" fontSize="10" fontFamily="system-ui">:8080</text>

      {/* ── Browser B ─────────────────────────────── */}
      <circle cx="580" cy="130" r="42" fill="rgba(5,150,105,0.07)" stroke="rgba(5,150,105,0.35)" strokeWidth="1.5" />
      <circle cx="580" cy="130" r="34" fill="rgba(5,150,105,0.04)" />
      <text x="580" y="125" textAnchor="middle" fill="#059669" fontSize="15" fontWeight="700" fontFamily="system-ui">B</text>
      <text x="580" y="143" textAnchor="middle" fill="rgba(5,150,105,0.55)" fontSize="9" fontFamily="system-ui">Browser B</text>
      <text x="580" y="190" textAnchor="middle" fill="rgba(5,150,105,0.35)" fontSize="10" fontFamily="system-ui">:8081</text>

      {/* ── Browser C ─────────────────────────────── */}
      <circle cx="380" cy="260" r="42" fill="rgba(217,119,6,0.07)" stroke="rgba(217,119,6,0.35)" strokeWidth="1.5" />
      <circle cx="380" cy="260" r="34" fill="rgba(217,119,6,0.04)" />
      <text x="380" y="255" textAnchor="middle" fill="#d97706" fontSize="15" fontWeight="700" fontFamily="system-ui">C</text>
      <text x="380" y="273" textAnchor="middle" fill="rgba(217,119,6,0.55)" fontSize="9" fontFamily="system-ui">Browser C</text>
      <text x="380" y="225" textAnchor="middle" fill="rgba(217,119,6,0.35)" fontSize="10" fontFamily="system-ui">:8082</text>

      {/* Anti-entropy label */}
      <text x="380" y="112" textAnchor="middle" fill="rgba(99,102,241,0.4)" fontSize="10" fontFamily="system-ui" fontStyle="italic">sync engine · anti-entropy every 3 s</text>
    </svg>
  )
}
