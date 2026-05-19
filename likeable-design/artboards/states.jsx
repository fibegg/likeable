// Likeable v2 — secondary states + flows
// Sign-in, project list, onboarding wizard, service selector, export, starting

const { StudioCanvas, StatusChip, TimelineStep, SystemEvent, ServiceBadge,
        HourPill, QuotaBar, PreviewFrame, FibeTab, BloodAnalysisLIS, Icon,
        ChatHeader, NowStrip, Composer } = window;

// ─── Sign-in ──────────────────────────────────────────────────────
window.SignedOut = function SignedOut() {
  return (
    <StudioCanvas>
      {/* Brand */}
      <div style={{ position: 'absolute', top: 36, left: 36, display: 'flex', alignItems: 'center', gap: 10 }}>
        <span className="lk-mark is-lg">L</span>
        <div>
          <div style={{ fontSize: 16, fontWeight: 600, letterSpacing: '-0.01em' }}>Likeable</div>
          <div className="lk-mono" style={{ fontSize: 9.5, color: 'var(--text-tertiary)', letterSpacing: '.14em' }}>POWERED BY FIBE.GG</div>
        </div>
      </div>

      <div style={{ position: 'absolute', top: 36, right: 36, display: 'flex', gap: 8, alignItems: 'center' }}>
        <button className="lk-btn is-sm is-ghost"><Icon name="language" size={12} /> EN / UK</button>
        <button className="lk-btn is-sm is-ghost"><Icon name="github" size={12} /> Status</button>
      </div>

      {/* Two-up: marketing hook + sign-in */}
      <div style={{ position: 'absolute', inset: 0, display: 'grid', gridTemplateColumns: '1.1fr 1fr', padding: 36, paddingTop: 110, gap: 48 }}>
        {/* Left */}
        <div style={{ display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
          <span className="lk-eyebrow is-cyan" style={{ marginBottom: 14 }}>AI PRODUCT STUDIO · v2</span>
          <h1 className="lk-display" style={{ fontSize: 64, margin: 0, fontWeight: 200, maxWidth: 580 }}>
            Describe an app.<br/>Build it. Ship it.
          </h1>
          <p style={{ fontSize: 14.5, color: 'var(--text-secondary)', marginTop: 18, maxWidth: 480, lineHeight: 1.55 }}>
            Likeable boots a Fibe playground, iterates with you through chat, and exports the source whenever you're ready. Works on phone or desktop.
          </p>

          {/* What happens next */}
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 10, marginTop: 36, maxWidth: 560 }}>
            {[
              { i: 'sparkle',  t: 'Sign in', s: 'Google or developer key' },
              { i: 'rocket',   t: 'Get a playground', s: 'Up to 5 hours free / 24h' },
              { i: 'github',   t: 'Export anywhere', s: 'GitHub or ZIP, anytime' },
            ].map((s, i) => (
              <div key={s.t} style={{ padding: 14, background: 'var(--bg-surface)', border: '1px solid var(--border-subtle)', borderRadius: 'var(--r-md)' }}>
                <span className="lk-mono lk-num" style={{ fontSize: 9.5, color: 'var(--amber)', letterSpacing: '.12em' }}>{String(i+1).padStart(2,'0')}</span>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 6, marginBottom: 4 }}>
                  <Icon name={s.i} size={14} style={{ color: 'var(--amber-bright)' }} />
                  <span style={{ fontSize: 12.5, fontWeight: 600 }}>{s.t}</span>
                </div>
                <div style={{ fontSize: 11.5, color: 'var(--text-tertiary)' }}>{s.s}</div>
              </div>
            ))}
          </div>
        </div>

        {/* Right — sign-in card with FIBE tab on top */}
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center' }}>
          <FibeTab />
          <div className="lk-studio-chat" style={{ width: 380, padding: 24, display: 'flex', flexDirection: 'column', gap: 14, marginTop: -1 }}>
            <div>
              <h2 style={{ fontSize: 19, fontWeight: 600, margin: 0, letterSpacing: '-0.01em' }}>Sign in</h2>
              <p style={{ margin: '4px 0 0', fontSize: 12.5, color: 'var(--text-secondary)' }}>You'll land in a fresh playground with starter prompts.</p>
            </div>
            <button className="lk-btn is-lg" style={{ background: '#fff', color: '#1a1d22', border: 'none', justifyContent: 'flex-start', paddingLeft: 14, height: 44 }}>
              <Icon name="google" size={15} /> <span>Continue with Google</span>
            </button>
            <button className="lk-btn is-lg" style={{ justifyContent: 'flex-start', paddingLeft: 14, height: 44 }}>
              <Icon name="github" size={15} /> <span>Continue with developer key</span>
            </button>
            <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
              <div style={{ flex: 1, height: 1, background: 'var(--border-hairline)' }} />
              <span className="lk-mono" style={{ fontSize: 9.5, color: 'var(--text-tertiary)', textTransform: 'uppercase', letterSpacing: '.14em' }}>OR EMAIL</span>
              <div style={{ flex: 1, height: 1, background: 'var(--border-hairline)' }} />
            </div>
            <input className="lk-input" placeholder="you@studio.com" />
            <button className="lk-btn is-lg is-primary">Continue</button>
            <div style={{ display: 'flex', alignItems: 'flex-start', gap: 8, padding: 10, background: 'var(--info-bg)', borderRadius: 8, border: '1px solid rgba(126,200,255,.22)' }}>
              <Icon name="info" size={12} style={{ color: 'var(--info)', marginTop: 2 }} />
              <span style={{ fontSize: 11, color: 'var(--text-secondary)' }}>
                Signups are <strong style={{ color: 'var(--text-primary)' }}>closed</strong> on this instance. Email rayfesoul@gmail.com for access.
              </span>
            </div>
          </div>
        </div>
      </div>
    </StudioCanvas>
  );
};

// ─── Playgrounds list (the dropdown inside the chat) ─────────────
window.ProjectList = function ProjectList() {
  return (
    <StudioCanvas>
      <div style={{ position: 'absolute', left: '50%', transform: 'translateX(-50%)', top: 80 }}>
        <FibeTab />
      </div>
      <div className="lk-studio-chat" style={{
        position: 'absolute', left: '50%', transform: 'translateX(-50%)',
        top: 102, width: 760, paddingBottom: 12,
      }}>
        <ChatHeader project="Playgrounds" count="2/3" />
        <div style={{ padding: '14px 16px 0', display: 'flex', alignItems: 'baseline', gap: 8 }}>
          <span className="lk-eyebrow">PLAYGROUNDS</span>
          <span style={{ fontSize: 13.5, color: 'var(--text-primary)', fontWeight: 600 }}>2 of 3 slots used</span>
          <span style={{ flex: 1 }} />
          <button className="lk-iconbtn is-sm"><Icon name="close" size={13} /></button>
        </div>

        <div style={{ padding: 16, display: 'flex', flexDirection: 'column', gap: 8 }}>
          {[
            { n: 'Blood Analysis webapp', s: 'ready',    sel: true,  upd: '12s ago', svc: 3, kind: 'Workflow app · LIS' },
            { n: 'Pricing experiments',   s: 'paused',   sel: false, upd: '2h ago',  svc: 1, kind: 'Internal tool' },
            { n: 'New playground',        s: 'archived', sel: false, upd: 'May 8',   svc: 0, kind: 'Product UI' },
          ].map(p => (
            <div key={p.n} style={{
              background: p.sel ? 'var(--amber-bg-08)' : 'var(--bg-elevated)',
              border: `1px solid ${p.sel ? 'var(--border-default)' : 'var(--border-subtle)'}`,
              borderRadius: 'var(--r-md)',
              padding: '12px 14px',
              display: 'flex', alignItems: 'center', gap: 12,
            }}>
              <span className={`lk-mark is-sm`} style={{ background: p.s === 'archived' ? 'var(--bg-overlay)' : undefined, color: p.s === 'archived' ? 'var(--text-tertiary)' : undefined }}>
                {p.n[0]}
              </span>
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ display: 'flex', alignItems: 'baseline', gap: 8 }}>
                  <span style={{ fontSize: 13.5, fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', color: p.s === 'archived' ? 'var(--text-secondary)' : 'var(--text-primary)' }}>{p.n}</span>
                  {p.sel && <span className="lk-mono" style={{ fontSize: 9.5, color: 'var(--amber)', letterSpacing: '.1em' }}>CURRENT</span>}
                </div>
                <div className="lk-mono" style={{ fontSize: 10.5, color: 'var(--text-tertiary)', marginTop: 2, letterSpacing: '.04em' }}>
                  {p.kind.toUpperCase()} · {p.svc} SVC · UPDATED {p.upd.toUpperCase()}
                </div>
              </div>
              <StatusChip state={p.s} />
              <button className="lk-iconbtn is-sm" title="Export"><Icon name="upload" size={13} /></button>
              <button className="lk-iconbtn is-sm" title="Rename"><Icon name="edit" size={13} /></button>
              <button className="lk-iconbtn is-sm" title="Delete"><Icon name="trash" size={13} /></button>
              <button className="lk-iconbtn is-sm" title="More"><Icon name="dots" size={13} /></button>
            </div>
          ))}
        </div>

        <div style={{ padding: '0 16px' }}>
          <button className="lk-btn is-lg" style={{
            width: '100%', justifyContent: 'center',
            border: '1px dashed var(--border-default)',
            background: 'transparent', color: 'var(--amber)',
          }}>
            <Icon name="plus" size={14} /> New playground
            <span className="lk-mono" style={{ fontSize: 10, color: 'var(--text-tertiary)', marginLeft: 10 }}>~3 BUILD HOURS EST</span>
          </button>
        </div>
      </div>
    </StudioCanvas>
  );
};

// ─── Onboarding wizard — matches "Створення через опис" but improved ───────
window.OnboardingWizard = function OnboardingWizard() {
  return (
    <StudioCanvas>
      <div style={{ position: 'absolute', inset: 0, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', padding: 36 }}>
        <span className="lk-chip is-ok" style={{ marginBottom: 18 }}><Icon name="check" size={11} stroke={2.4} /> PLAYGROUND READY</span>
        <span className="lk-eyebrow" style={{ marginBottom: 10 }}>FIRST PLAYGROUND · TUTORIAL</span>
        <h1 className="lk-display" style={{ fontSize: 56, margin: 0, fontWeight: 200 }}>Creating from a description</h1>
        <p style={{ fontSize: 14, color: 'var(--text-secondary)', marginTop: 14, maxWidth: 520, textAlign: 'center', lineHeight: 1.55 }}>
          A short guide while your first playground warms up. Start will appear when the preview is ready.
        </p>

        <div style={{ width: 720, marginTop: 36, background: 'var(--bg-surface)', border: '1px solid var(--border-subtle)', borderRadius: 'var(--r-xl)', padding: 24 }}>
          <div style={{ display: 'flex', alignItems: 'flex-start', gap: 12 }}>
            <span style={{ width: 32, height: 32, borderRadius: 8, background: 'var(--amber-bg-12)', color: 'var(--amber-bright)', display: 'inline-flex', alignItems: 'center', justifyContent: 'center', flex: '0 0 auto' }}>
              <Icon name="sparkle" size={16} />
            </span>
            <div style={{ flex: 1 }}>
              <div style={{ fontSize: 16, fontWeight: 600, letterSpacing: '-0.005em' }}>Write the first version prompt</div>
              <div style={{ fontSize: 12.5, color: 'var(--text-secondary)', marginTop: 4 }}>Describe product, screens, data and style. Then refine in chat.</div>
              <ul style={{ margin: '12px 0 0', padding: 0, listStyle: 'none', display: 'flex', flexDirection: 'column', gap: 6 }}>
                <li style={{ display: 'flex', gap: 8, fontSize: 12.5, color: 'var(--text-secondary)' }}>
                  <span style={{ color: 'var(--amber)' }}>●</span> One clear change per message works best.
                </li>
                <li style={{ display: 'flex', gap: 8, fontSize: 12.5, color: 'var(--text-secondary)' }}>
                  <span style={{ color: 'var(--amber)' }}>●</span> Drop a reference image when layout matters.
                </li>
                <li style={{ display: 'flex', gap: 8, fontSize: 12.5, color: 'var(--text-secondary)' }}>
                  <span style={{ color: 'var(--amber)' }}>●</span> Use slash commands for common edits.
                </li>
              </ul>
            </div>
            <span className="lk-mono" style={{ fontSize: 11, color: 'var(--text-tertiary)' }}>1 / 6</span>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 8, marginTop: 18 }}>
            {[
              { i: 'sparkle', t: 'Product UI',    s: 'Dashboards, landings, forms, internal tools.' },
              { i: 'pulse',   t: 'Workflow app',  s: 'Records, approvals, queues, hand-offs.' },
              { i: 'bolt',    t: 'Interactive idea', s: 'Games, calculators, maps, visual.' },
            ].map(s => (
              <div key={s.t} style={{
                padding: 12, background: 'var(--bg-elevated)',
                border: '1px solid var(--border-hairline)', borderRadius: 'var(--r-md)',
              }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 6, color: 'var(--amber)', marginBottom: 6 }}>
                  <Icon name={s.i} size={12} />
                  <span style={{ fontSize: 11.5, fontWeight: 600, color: 'var(--text-primary)' }}>{s.t}</span>
                </div>
                <div style={{ fontSize: 11, color: 'var(--text-tertiary)', lineHeight: 1.4 }}>{s.s}</div>
              </div>
            ))}
          </div>

          <div style={{ display: 'flex', alignItems: 'center', marginTop: 22 }}>
            {/* progress dots */}
            <div style={{ display: 'flex', gap: 6 }}>
              {[0,1,2,3,4,5].map(i => (
                <span key={i} style={{
                  width: i === 0 ? 24 : 8, height: 8, borderRadius: 999,
                  background: i === 0 ? 'var(--amber)' : 'var(--border-subtle)',
                }} />
              ))}
            </div>
            <span style={{ flex: 1 }} />
            <button className="lk-btn is-sm is-ghost">Skip tutorial</button>
            <button className="lk-btn is-primary" style={{ marginLeft: 8 }}>
              <Icon name="play" size={11} /> Start <Icon name="chevRight" size={12} />
            </button>
          </div>
        </div>
      </div>
    </StudioCanvas>
  );
};

// ─── Playground starting ─────────────────────────────────────────
window.PlaygroundStarting = function PlaygroundStarting() {
  return (
    <StudioCanvas>
      <div style={{ position: 'absolute', inset: 0, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', paddingBottom: 200 }}>
        <span className="lk-chip is-warn is-loading" style={{ marginBottom: 18 }}><span className="lk-dot" /> ALLOCATING FIBE PLAYGROUND</span>
        <h1 className="lk-display" style={{ fontSize: 60, margin: 0, fontWeight: 200 }}>Warming up…</h1>
        <p style={{ fontSize: 14, color: 'var(--text-secondary)', marginTop: 14, maxWidth: 460, textAlign: 'center', lineHeight: 1.5 }}>
          Likeable is reserving a Fibe cell and warming the dev server. Build hours are <strong style={{ color: 'var(--text-primary)' }}>not counted</strong> while booting.
        </p>

        <div style={{ width: 520, marginTop: 32, background: 'var(--bg-surface)', border: '1px solid var(--border-subtle)', borderRadius: 'var(--r-lg)', padding: 18 }}>
          <div className="lk-eyebrow" style={{ marginBottom: 12 }}>BOOT TIMELINE</div>
          <TimelineStep state="done"    label="Reserved compute"  detail="fibe://us-east/cell-04 · 2 vCPU · 4 GB" time="0.4s" />
          <TimelineStep state="done"    label="Pulled base image" detail="node:20-alpine · cached" time="1.1s" />
          <TimelineStep state="running" label="Installing dependencies" detail="pnpm install · 312 / 487 packages" time="8.2s" />
          <TimelineStep state="queued"  label="Booting dev server" />
          <TimelineStep state="queued"  label="Warming preview" last />
        </div>

        <div style={{ display: 'flex', gap: 8, marginTop: 18 }}>
          <button className="lk-btn is-sm is-ghost"><Icon name="code" size={12} /> View boot log</button>
          <button className="lk-btn is-sm is-ghost"><Icon name="stop" size={12} stroke={2} /> Cancel</button>
        </div>
      </div>
    </StudioCanvas>
  );
};

// ─── Multi-service selector ───────────────────────────────────────
window.ServiceSelector = function ServiceSelector() {
  return (
    <StudioCanvas>
      <div style={{ position: 'absolute', left: '50%', transform: 'translateX(-50%)', top: 60 }}>
        <FibeTab />
      </div>
      <div className="lk-studio-chat" style={{ position: 'absolute', left: '50%', transform: 'translateX(-50%)', top: 82, width: 620 }}>
        <ChatHeader project="Switch service" count="3 services" />
        <div style={{ padding: '14px 16px 4px', display: 'flex', alignItems: 'baseline' }}>
          <span className="lk-eyebrow">SERVICE MAP</span>
          <span style={{ flex: 1 }} />
          <span style={{ fontSize: 11.5, color: 'var(--text-tertiary)' }}>One previews at a time</span>
        </div>
        <div style={{ padding: '12px 16px 16px', display: 'flex', flexDirection: 'column', gap: 8 }}>
          {[
            { k: 'web', n: 'app',     p: '5173', s: 'live',     a: true,  u: 'app.likeable.studio/blood-analysis', d: 'React + Vite · /board, /patient/:id' },
            { k: 'api', n: 'api',     p: '3001', s: 'live',     a: false, u: 'api.blood-analysis.likeable.app', d: 'Hono · 14 routes · OpenAPI' },
            { k: 'worker', n: 'reports', p: '—',   s: 'starting', a: false, u: 'fibe://worker/reports', d: 'PDF generator · 2 jobs queued' },
          ].map(svc => (
            <div key={svc.n} style={{
              background: svc.a ? 'var(--amber-bg-08)' : 'var(--bg-elevated)',
              border: `1px solid ${svc.a ? 'var(--border-default)' : 'var(--border-subtle)'}`,
              borderRadius: 'var(--r-md)',
              padding: 14, display: 'flex', alignItems: 'center', gap: 12,
              position: 'relative',
            }}>
              {svc.a && (
                <span className="lk-mono" style={{
                  position: 'absolute', top: -8, left: 14,
                  padding: '2px 8px', borderRadius: 4,
                  background: 'var(--amber)', color: 'var(--amber-fg)',
                  fontSize: 9, fontWeight: 700, letterSpacing: '.12em',
                }}>PREVIEWING</span>
              )}
              <div style={{ width: 36, height: 36, borderRadius: 8, background: 'rgba(0,0,0,0.3)', color: svc.a ? 'var(--amber-bright)' : 'var(--text-secondary)', display: 'flex', alignItems: 'center', justifyContent: 'center', flex: '0 0 auto' }}>
                <Icon name={svc.k} size={16} />
              </div>
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <span style={{ fontSize: 14, fontWeight: 600 }}>{svc.n}</span>
                  <span className="lk-mono" style={{ fontSize: 10, color: 'var(--text-tertiary)', textTransform: 'uppercase', letterSpacing: '.08em' }}>{svc.k}</span>
                </div>
                <div style={{ fontSize: 11.5, color: 'var(--text-secondary)' }}>{svc.d}</div>
                <div className="lk-mono" style={{ fontSize: 10.5, color: 'var(--text-tertiary)', marginTop: 2 }}>{svc.u}</div>
              </div>
              <StatusChip state={svc.s === 'live' ? 'live' : 'starting'} />
              <button className="lk-btn is-sm">{svc.a ? 'Active' : 'Preview'}</button>
            </div>
          ))}
        </div>
        <div className="lk-divider" />
        <div style={{ padding: 12, display: 'flex', alignItems: 'center', gap: 8 }}>
          <Icon name="info" size={13} style={{ color: 'var(--info)' }} />
          <span style={{ fontSize: 11.5, color: 'var(--text-secondary)' }}>
            The agent can edit any service. Only one renders in the preview at a time.
          </span>
          <span style={{ flex: 1 }} />
          <button className="lk-btn is-sm">Open in new tab</button>
        </div>
      </div>
    </StudioCanvas>
  );
};

// ─── Export dialog ────────────────────────────────────────────────
window.ExportDialog = function ExportDialog() {
  return (
    <StudioCanvas>
      <div style={{ position: 'absolute', inset: 0, background: 'rgba(2,4,6,.6)', backdropFilter: 'blur(8px)' }} />
      <div style={{ position: 'absolute', left: '50%', top: '50%', transform: 'translate(-50%, -50%)', display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
        <FibeTab />
        <div className="lk-studio-chat" style={{ width: 580, marginTop: -1 }}>
          {/* Header */}
          <div style={{ padding: '14px 16px', borderBottom: '1px solid var(--border-hairline)', display: 'flex', alignItems: 'center', gap: 10 }}>
            <span style={{ width: 30, height: 30, borderRadius: 8, background: 'var(--amber-bg-12)', color: 'var(--amber-bright)', display: 'inline-flex', alignItems: 'center', justifyContent: 'center' }}>
              <Icon name="upload" size={14} />
            </span>
            <div>
              <div style={{ fontSize: 14, fontWeight: 600 }}>Export Blood Analysis webapp</div>
              <div className="lk-mono" style={{ fontSize: 10, color: 'var(--text-tertiary)', letterSpacing: '.08em' }}>BUILD 0247 · UPDATED 12s AGO</div>
            </div>
            <span style={{ flex: 1 }} />
            <button className="lk-iconbtn"><Icon name="close" size={14} /></button>
          </div>

          <div style={{ padding: 16, display: 'flex', flexDirection: 'column', gap: 16 }}>
            {/* Readiness */}
            <div>
              <div className="lk-eyebrow" style={{ marginBottom: 8 }}>EXPORT READINESS</div>
              <div className="lk-card-elev" style={{ overflow: 'hidden' }}>
                {[
                  { ok: true,  t: 'Source available', s: '247 files · 12.4 MB' },
                  { ok: true,  t: 'Last build successful', s: 'Build 0247 · 2 min ago' },
                  { ok: false, t: 'GitHub disconnected', s: 'Reconnect to push directly', cta: 'Reconnect' },
                  { ok: true,  t: 'Archive ready', s: 'Generated 12 seconds ago' },
                ].map((r, i) => (
                  <div key={r.t} style={{
                    display: 'flex', alignItems: 'center', gap: 12,
                    padding: '10px 12px',
                    borderTop: i === 0 ? 'none' : '1px solid var(--border-hairline)',
                  }}>
                    <span style={{
                      width: 20, height: 20, borderRadius: 5,
                      background: r.ok ? 'var(--ok-bg)' : 'var(--warn-bg)',
                      color: r.ok ? 'var(--ok)' : 'var(--warn)',
                      display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
                    }}>
                      <Icon name={r.ok ? 'check' : 'alert'} size={11} stroke={2.4} />
                    </span>
                    <div style={{ flex: 1 }}>
                      <div style={{ fontSize: 12.5, fontWeight: 500 }}>{r.t}</div>
                      <div className="lk-mono" style={{ fontSize: 10.5, color: 'var(--text-tertiary)' }}>{r.s}</div>
                    </div>
                    {r.cta && <button className="lk-btn is-sm"><Icon name="github" size={12} /> {r.cta}</button>}
                  </div>
                ))}
              </div>
            </div>

            {/* Destinations */}
            <div>
              <div className="lk-eyebrow" style={{ marginBottom: 8 }}>SEND TO</div>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 }}>
                <div style={{
                  border: '1.5px solid var(--border-default)',
                  background: 'var(--amber-bg-08)',
                  borderRadius: 'var(--r-md)', padding: 14,
                  display: 'flex', flexDirection: 'column', gap: 8,
                  cursor: 'pointer',
                }}>
                  <Icon name="github" size={20} style={{ color: 'var(--amber)' }} />
                  <div>
                    <div style={{ fontSize: 13, fontWeight: 600 }}>GitHub repository</div>
                    <div className="lk-mono" style={{ fontSize: 10, color: 'var(--text-tertiary)', marginTop: 2 }}>kateryna/blood-analysis · main</div>
                  </div>
                  <span className="lk-chip is-warn" style={{ height: 18, fontSize: 9.5 }}>DISCONNECTED</span>
                </div>
                <div style={{
                  border: '1px solid var(--border-subtle)',
                  background: 'var(--bg-elevated)',
                  borderRadius: 'var(--r-md)', padding: 14,
                  display: 'flex', flexDirection: 'column', gap: 8,
                  cursor: 'pointer',
                }}>
                  <Icon name="archive" size={20} style={{ color: 'var(--text-secondary)' }} />
                  <div>
                    <div style={{ fontSize: 13, fontWeight: 600 }}>ZIP archive</div>
                    <div className="lk-mono" style={{ fontSize: 10, color: 'var(--text-tertiary)', marginTop: 2 }}>blood-analysis-0247.zip · 12.4 MB</div>
                  </div>
                  <span className="lk-chip is-ok" style={{ height: 18, fontSize: 9.5 }}><span className="lk-dot" /> READY</span>
                </div>
              </div>
            </div>

            {/* Options */}
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
              {[
                { l: 'Include build artifacts (.next, dist)', d: 'Adds ~38 MB. Skip for clean export.', on: false },
                { l: 'Include environment variables', d: 'Stripped to placeholders by default.', on: false },
                { l: 'Generate README with the prompt history', d: 'New', on: true },
              ].map(o => (
                <label key={o.l} style={{ display: 'flex', alignItems: 'flex-start', gap: 10, padding: '4px 0' }}>
                  <span style={{
                    width: 16, height: 16, borderRadius: 4,
                    border: `1.5px solid ${o.on ? 'var(--amber)' : 'var(--border-strong)'}`,
                    background: o.on ? 'var(--amber)' : 'transparent',
                    display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
                    flex: '0 0 auto', marginTop: 1,
                  }}>
                    {o.on && <Icon name="check" size={10} stroke={3} style={{ color: 'var(--amber-fg)' }} />}
                  </span>
                  <span>
                    <div style={{ fontSize: 12.5 }}>{o.l}</div>
                    <div style={{ fontSize: 10.5, color: 'var(--text-tertiary)' }}>{o.d}</div>
                  </span>
                </label>
              ))}
            </div>
          </div>

          <div style={{ padding: '12px 16px', borderTop: '1px solid var(--border-hairline)', display: 'flex', alignItems: 'center', gap: 8 }}>
            <button className="lk-btn is-sm is-ghost"><Icon name="history" size={12} /> Export history</button>
            <span style={{ flex: 1 }} />
            <button className="lk-btn">Cancel</button>
            <button className="lk-btn is-primary"><Icon name="download" size={13} /> Download ZIP</button>
          </div>
        </div>
      </div>
    </StudioCanvas>
  );
};
