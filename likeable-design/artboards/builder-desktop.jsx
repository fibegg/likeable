// Likeable v2 — Desktop builder
// Preview canvas behind + floating glass chat panel (the brand identity)
// with the readability problems fixed.

const { StudioCanvas, StatusChip, TimelineStep, SystemEvent, ServiceBadge,
        HourPill, QuotaBar, PreviewFrame, FibeTab, BloodAnalysisLIS, Icon } = window;

// ─── Header row inside the floating chat ─────────────────────────
const ChatHeader = ({ project = 'Blood Analysis webapp', count = '1/3', service = 'app', lang = 'EN' }) => (
  <div style={{
    display: 'flex', alignItems: 'center', gap: 10,
    padding: '12px 14px 12px 12px',
    borderBottom: '1px solid var(--border-hairline)',
  }}>
    <span className="lk-mark">L</span>
    <div style={{ minWidth: 0, flex: 1 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <span style={{ fontSize: 13.5, fontWeight: 600, letterSpacing: '-0.01em', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 280 }}>
          {project}
        </span>
        <Icon name="chevDown" size={13} style={{ color: 'var(--text-tertiary)', cursor: 'pointer' }} />
      </div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 3 }}>
        <span className="lk-mono lk-num" style={{ fontSize: 10, color: 'var(--text-tertiary)', textTransform: 'uppercase', letterSpacing: '.08em' }}>
          BUILD 0247 · UPDATED 12s AGO
        </span>
      </div>
    </div>

    {/* Service selector — labeled, not icon-only */}
    <button style={{
      display: 'flex', alignItems: 'center', gap: 6,
      height: 28, padding: '0 10px',
      background: 'var(--amber-bg-08)',
      border: '1px solid var(--border-default)',
      borderRadius: 999, color: 'var(--amber-bright)',
      cursor: 'pointer', font: '500 11px/1 var(--font-sans)',
    }}>
      <Icon name="web" size={11} stroke={1.6} /> {service}
      <Icon name="chevDown" size={11} />
    </button>

    {/* Playgrounds chip — kept compact like original */}
    <button style={{
      display: 'flex', alignItems: 'center', gap: 6,
      height: 28, padding: '0 10px',
      background: 'transparent', border: '1px solid var(--border-default)',
      borderRadius: 999, color: 'var(--amber)', cursor: 'pointer',
      font: '500 11px/1 var(--font-mono)',
    }}>
      <Icon name="folder" size={11} stroke={1.6} /> {count}
    </button>

    {/* Lang */}
    <button style={{
      display: 'flex', alignItems: 'center', gap: 6,
      height: 28, padding: '0 10px',
      background: 'transparent', border: '1px solid var(--border-subtle)',
      borderRadius: 999, color: 'var(--text-secondary)', cursor: 'pointer',
      font: '500 11px/1 var(--font-mono)',
    }}>
      <Icon name="language" size={11} stroke={1.6} /> {lang}
    </button>

    {/* Layout toggle */}
    <div style={{ display: 'flex', gap: 2, padding: 2, background: 'var(--bg-overlay)', borderRadius: 8, border: '1px solid var(--border-hairline)' }}>
      <button className="lk-iconbtn is-sm is-active" title="Overlay mode"><Icon name="overlay" size={13} /></button>
      <button className="lk-iconbtn is-sm" title="Split mode"><Icon name="split" size={13} /></button>
    </div>

    {/* Profile + help — kept as icons but tighter */}
    <button className="lk-iconbtn is-sm" title="Profile"><Icon name="user" size={13} /></button>
    <button className="lk-iconbtn is-sm" title="Help"><Icon name="help" size={13} /></button>
    <button className="lk-iconbtn is-sm" title="Minimize"><Icon name="collapse" size={13} /></button>
  </div>
);

// ─── "What's happening now" status strip ─────────────────────────
const NowStrip = ({ state = 'ready', detail }) => {
  const map = {
    ready:    { chip: 'is-ok',     label: 'Live · app · web preview',   action: { i: 'sparkle', t: 'Ask the agent for the next change', hint: '⌘K' } },
    building: { chip: 'is-amber is-loading', label: 'Agent is editing 4 files',     action: { i: 'steer', t: 'Steer or queue', hint: 'Shift⏎' } },
    starting: { chip: 'is-warn is-loading',  label: 'Playground starting · 14s',    action: { i: 'eye', t: 'Watch the boot log' } },
    archived: { chip: 'is-cyan',   label: 'Archived · export-only',     action: { i: 'download', t: 'Download source' } },
  };
  const m = map[state] || map.ready;
  return (
    <div style={{
      display: 'flex', alignItems: 'center', gap: 10,
      padding: '8px 14px',
      borderBottom: '1px solid var(--border-hairline)',
    }}>
      <span className={`lk-chip ${m.chip}`}><span className="lk-dot" /> {m.label}</span>
      <span style={{ flex: 1 }} />
      <span className="lk-eyebrow is-dim">NEXT BEST ACTION</span>
      <button className="lk-btn is-sm">
        <Icon name={m.action.i} size={12} />
        {m.action.t}
        {m.action.hint && <span className="lk-kbd">{m.action.hint}</span>}
      </button>
    </div>
  );
};

// ─── Composer (inside chat panel) ─────────────────────────────────
const Composer = ({ mode = 'idle', placeholder = 'Describe what you want to build…', value = '' }) => (
  <div style={{
    margin: 12, padding: 10,
    background: 'var(--bg-inset)',
    border: '1px solid var(--border-subtle)',
    borderRadius: 'var(--r-lg)',
  }}>
    <div style={{ display: 'flex', alignItems: 'flex-start', gap: 10 }}>
      <button className="lk-iconbtn" title="Attach"><Icon name="paperclip" size={15} /></button>
      <div style={{ flex: 1, minHeight: 36, paddingTop: 7, fontSize: 13.5, color: value ? 'var(--text-primary)' : 'var(--text-tertiary)' }}>
        {value || placeholder}
      </div>
      <HourPill used="4h 26m" total="5h" />
      <button className="lk-btn is-primary" style={{ height: 32 }}>
        <Icon name="send" size={13} />
      </button>
    </div>
    <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginTop: 8, paddingLeft: 38 }}>
      <button className="lk-btn is-sm is-ghost">
        <Icon name="sparkle" size={12} /> Improve prompt
      </button>
      <button className="lk-btn is-sm is-ghost">
        <Icon name="image" size={12} /> Reference
      </button>
      <button className="lk-btn is-sm is-ghost">
        <Icon name="cmd" size={12} /> Slash
      </button>
      <span style={{ flex: 1 }} />
      {mode === 'building' ? (
        <>
          <span className="lk-mono" style={{ fontSize: 10, color: 'var(--text-tertiary)' }}>AGENT RUNNING · MSG WILL QUEUE</span>
          <button className="lk-btn is-sm is-danger"><Icon name="stop" size={11} stroke={2} /> Stop</button>
        </>
      ) : (
        <span className="lk-mono" style={{ fontSize: 10, color: 'var(--text-tertiary)' }}>⏎ SEND · ⇧⏎ QUEUE</span>
      )}
    </div>
  </div>
);

// ─── BUILDER · ready state (the main artboard) ───────────────────
window.BuilderReady = function BuilderReady() {
  return (
    <StudioCanvas>
      {/* Background preview, full-bleed within brackets */}
      <div style={{ position: 'absolute', inset: 36, borderRadius: 'var(--r-lg)', overflow: 'hidden', boxShadow: '0 24px 80px rgba(0,0,0,.4)', border: '1px solid var(--border-hairline)' }}>
        <PreviewFrame url="app.likeable.studio/blood-analysis/p-001" label="app">
          <BloodAnalysisLIS />
        </PreviewFrame>
      </div>

      {/* FIBE tab */}
      <div style={{ position: 'absolute', left: '50%', transform: 'translateX(-50%)', bottom: 380 }}>
        <FibeTab />
      </div>

      {/* Floating chat panel — bottom-center, glass */}
      <div className="lk-studio-chat" style={{
        position: 'absolute', left: '50%', transform: 'translateX(-50%)',
        bottom: 36, width: 880, maxHeight: 360,
        display: 'flex', flexDirection: 'column',
      }}>
        <ChatHeader />
        <NowStrip state="ready" />

        {/* Chat body */}
        <div style={{ flex: 1, overflow: 'auto', padding: '12px 14px', display: 'flex', flexDirection: 'column', gap: 8, minHeight: 0 }}>
          {/* System event — formerly leaked [[LIKEABLE_NOTIFICATION_START]] */}
          <SystemEvent
            kind="ok"
            icon="check"
            label="Preview updated — dark-mode toggle added"
            sub="3 files · 12s ago"
            action={<button className="lk-btn is-sm is-ghost" style={{ height: 22, fontSize: 10.5 }}>View diff</button>}
            time="01:48"
          />
          {/* User message */}
          <div style={{ alignSelf: 'flex-end', maxWidth: '78%' }}>
            <div className="lk-mono lk-num" style={{ fontSize: 9.5, color: 'var(--text-tertiary)', textAlign: 'right', marginBottom: 4, letterSpacing: '.08em' }}>
              YOU · 01:48 PM
            </div>
            <div style={{
              background: 'var(--amber-bg-12)',
              border: '1px solid var(--border-default)',
              color: 'var(--text-primary)',
              borderRadius: '12px 12px 4px 12px',
              padding: '8px 12px',
              fontSize: 13, lineHeight: 1.45,
            }}>
              давай еще добавим переключение на темную тему
            </div>
          </div>
        </div>

        <Composer placeholder="Ask the agent to change anything in app…" />
      </div>
    </StudioCanvas>
  );
};

// ─── BUILDER · agent working ─────────────────────────────────────
window.BuilderBuilding = function BuilderBuilding() {
  return (
    <StudioCanvas>
      <div style={{ position: 'absolute', inset: 36, borderRadius: 'var(--r-lg)', overflow: 'hidden', boxShadow: '0 24px 80px rgba(0,0,0,.4)', border: '1px solid var(--border-hairline)' }}>
        <PreviewFrame url="app.likeable.studio/blood-analysis/p-001" label="app"
          actions={<span className="lk-chip is-amber is-loading"><span className="lk-dot" /> HMR · /board</span>}>
          <BloodAnalysisLIS />
          {/* Inspect highlight — agent is editing a region */}
          <div style={{
            position: 'absolute', top: 320, left: 50, right: 50, height: 64,
            border: '1.5px dashed var(--amber-bright)',
            borderRadius: 10,
            background: 'rgba(212, 149, 60, 0.06)',
            pointerEvents: 'none',
          }}>
            <span className="lk-mono" style={{
              position: 'absolute', top: -22, left: 0,
              padding: '2px 8px', borderRadius: 4,
              background: 'var(--amber)', color: 'var(--amber-fg)',
              fontSize: 10, fontWeight: 600, letterSpacing: '.06em',
            }}>BoardToolbar · being edited</span>
          </div>
        </PreviewFrame>
      </div>

      <div style={{ position: 'absolute', left: '50%', transform: 'translateX(-50%)', bottom: 470 }}>
        <FibeTab />
      </div>

      <div className="lk-studio-chat" style={{
        position: 'absolute', left: '50%', transform: 'translateX(-50%)',
        bottom: 36, width: 920, maxHeight: 450,
        display: 'flex', flexDirection: 'column',
      }}>
        <ChatHeader project="Blood Analysis webapp" />
        <NowStrip state="building" />

        <div style={{ flex: 1, overflow: 'auto', padding: '12px 14px', display: 'flex', flexDirection: 'column', gap: 10, minHeight: 0 }}>
          {/* User message */}
          <div style={{ alignSelf: 'flex-end', maxWidth: '78%' }}>
            <div className="lk-mono lk-num" style={{ fontSize: 9.5, color: 'var(--text-tertiary)', textAlign: 'right', marginBottom: 4, letterSpacing: '.08em' }}>
              YOU · 02:41 PM
            </div>
            <div style={{
              background: 'var(--amber-bg-12)',
              border: '1px solid var(--border-default)',
              borderRadius: '12px 12px 4px 12px',
              padding: '8px 12px', fontSize: 13,
            }}>
              Make the kanban board sortable by team, and add a filter chip for blocked tasks
            </div>
          </div>

          {/* Agent narration + timeline */}
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <span className="lk-mark is-sm">L</span>
            <span style={{ fontSize: 12, fontWeight: 600 }}>Likeable Agent</span>
            <span className="lk-chip is-amber is-loading"><span className="lk-dot" /> Working · 00:42</span>
            <span style={{ flex: 1 }} />
            <button className="lk-btn is-sm is-ghost" style={{ height: 22 }}>
              <Icon name="steer" size={11} /> Steer
            </button>
          </div>

          <div className="lk-card-elev" style={{ padding: 12 }}>
            <div className="lk-eyebrow" style={{ marginBottom: 10 }}>BUILD RUN</div>
            <TimelineStep state="done"    label="Prompt parsed"   detail="3 acceptance criteria detected" time="00:01" />
            <TimelineStep state="done"    label="Planning"        detail="Edit BoardView, add useTeamFilter, update Chip" time="00:08" />
            <TimelineStep state="done"    label="Editing files"   detail="3 of 4 written" time="00:24" />
            <TimelineStep state="running" label="Preview warming" detail="vite HMR · /board" time="00:42" />
            <TimelineStep state="queued"  label="Verifying" last />
          </div>

          {/* Inline file pills */}
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 4 }}>
            {[
              { f: 'src/views/BoardView.tsx', d: '+24 −6', s: 'done' },
              { f: 'src/hooks/useTeamFilter.ts', d: '+38 new', s: 'done' },
              { f: 'src/ui/Chip.tsx', d: '+5 −2', s: 'done' },
              { f: 'src/state/board.store.ts', d: 'writing…', s: 'run' },
            ].map(p => (
              <div key={p.f} style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '5px 9px', background: 'var(--bg-inset)', borderRadius: 6, border: '1px solid var(--border-hairline)' }}>
                <Icon name="code" size={11} stroke={1.6} style={{ color: 'var(--amber)' }} />
                <span className="lk-mono" style={{ fontSize: 10.5, flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{p.f}</span>
                <span className="lk-mono" style={{ fontSize: 10, color: p.s === 'run' ? 'var(--amber)' : 'var(--ok)' }}>{p.d}</span>
              </div>
            ))}
          </div>
        </div>

        <Composer mode="building" placeholder="Send a steer or queue a follow-up…" />
      </div>
    </StudioCanvas>
  );
};

// ─── BUILDER · empty / new playground ────────────────────────────
window.BuilderEmpty = function BuilderEmpty() {
  return (
    <StudioCanvas>
      {/* Empty centerpiece */}
      <div style={{ position: 'absolute', inset: 0, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', paddingBottom: 420 }}>
        <span className="lk-eyebrow is-cyan" style={{ marginBottom: 18 }}>NEW PLAYGROUND · 2 OF 3</span>
        <h1 className="lk-display" style={{ fontSize: 56, margin: 0 }}>What are we building?</h1>
        <p style={{ fontSize: 14.5, color: 'var(--text-secondary)', marginTop: 14, maxWidth: 540, textAlign: 'center', lineHeight: 1.5 }}>
          Describe an app in a sentence, drop a screenshot, or pick a starter. Likeable boots a Fibe playground and the agent gets to work.
        </p>

        {/* Starter cards — matches the wizard from screen 6 */}
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 12, marginTop: 28, width: 760 }}>
          {[
            { i: 'sparkle', t: 'Product UI', s: 'Dashboards, landings, forms, internal tools.' },
            { i: 'pulse',   t: 'Workflow app', s: 'Records, approvals, queues, hand-offs between teams.' },
            { i: 'bolt',    t: 'Interactive idea', s: 'Games, calculators, maps, visual experiments.' },
          ].map(s => (
            <button key={s.t} style={{
              padding: 16, textAlign: 'left',
              background: 'var(--bg-surface)',
              border: '1px solid var(--border-subtle)',
              borderRadius: 'var(--r-lg)',
              cursor: 'pointer', color: 'var(--text-primary)',
              transition: 'border-color .15s',
            }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, color: 'var(--amber-bright)', marginBottom: 8 }}>
                <Icon name={s.i} size={14} /> <span className="lk-eyebrow" style={{ color: 'var(--amber)' }}>{s.t}</span>
              </div>
              <div style={{ fontSize: 13, fontWeight: 500, marginBottom: 4 }}>{s.t}</div>
              <div style={{ fontSize: 11.5, color: 'var(--text-secondary)', lineHeight: 1.45 }}>{s.s}</div>
            </button>
          ))}
        </div>
      </div>

      <div style={{ position: 'absolute', left: '50%', transform: 'translateX(-50%)', bottom: 240 }}>
        <FibeTab />
      </div>

      <div className="lk-studio-chat" style={{
        position: 'absolute', left: '50%', transform: 'translateX(-50%)',
        bottom: 36, width: 760, padding: 4,
      }}>
        <ChatHeader project="New playground" count="2/3" />
        <div style={{ padding: '0 12px 12px' }}>
          <div style={{
            background: 'var(--bg-inset)',
            border: '1px solid var(--border-subtle)',
            borderRadius: 'var(--r-lg)',
            padding: 14,
          }}>
            <div style={{ display: 'flex', gap: 10 }}>
              <Icon name="sparkle" size={16} style={{ color: 'var(--amber-bright)', marginTop: 4, flex: '0 0 auto' }} />
              <div style={{ flex: 1, minHeight: 64, fontSize: 14, color: 'var(--text-tertiary)', lineHeight: 1.5, paddingTop: 2 }}>
                A Linear-style issue tracker for a 4-person team, with keyboard shortcuts and a dark mode…
              </div>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginTop: 6, paddingLeft: 26 }}>
              <button className="lk-btn is-sm is-ghost"><Icon name="paperclip" size={12} /> Attach</button>
              <button className="lk-btn is-sm is-ghost"><Icon name="image" size={12} /> Screenshot</button>
              <button className="lk-btn is-sm is-ghost"><Icon name="github" size={12} /> Clone repo</button>
              <span style={{ flex: 1 }} />
              <span className="lk-mono" style={{ fontSize: 10, color: 'var(--text-tertiary)' }}>~3 BUILD HOURS EST.</span>
              <button className="lk-btn is-primary"><Icon name="rocket" size={13} /> Start <span className="lk-kbd">⌘⏎</span></button>
            </div>
          </div>
        </div>
      </div>
    </StudioCanvas>
  );
};

// ─── BUILDER · split mode (alternate layout) ─────────────────────
window.BuilderSplit = function BuilderSplit() {
  return (
    <StudioCanvas>
      <div style={{ position: 'absolute', inset: 36, display: 'flex', gap: 12 }}>
        {/* Preview */}
        <div style={{ flex: 1.45, minWidth: 0, borderRadius: 'var(--r-lg)', overflow: 'hidden', boxShadow: '0 24px 80px rgba(0,0,0,.35)', border: '1px solid var(--border-hairline)' }}>
          <PreviewFrame url="app.likeable.studio/blood-analysis/p-001" label="app">
            <BloodAnalysisLIS />
          </PreviewFrame>
        </div>

        {/* Chat panel docked right */}
        <div style={{ flex: '0 0 420px', display: 'flex', flexDirection: 'column', position: 'relative' }}>
          {/* fibe tab attached */}
          <div style={{ display: 'flex', justifyContent: 'center', marginBottom: -1 }}><FibeTab /></div>
          <div className="lk-studio-chat" style={{ flex: 1, display: 'flex', flexDirection: 'column', minHeight: 0 }}>
            <ChatHeader />
            <NowStrip state="ready" />
            <div style={{ flex: 1, overflow: 'auto', padding: '12px 14px', display: 'flex', flexDirection: 'column', gap: 10, minHeight: 0 }}>
              <SystemEvent kind="ok" icon="check" label="GitHub connected · kateryna/blood-analysis" sub="just now" time="01:46" />
              <SystemEvent kind="ok" icon="check" label="Preview updated — dark-mode toggle added" sub="3 files" time="01:48" />
              <div style={{ alignSelf: 'flex-end', maxWidth: '88%' }}>
                <div className="lk-mono lk-num" style={{ fontSize: 9.5, color: 'var(--text-tertiary)', textAlign: 'right', marginBottom: 4, letterSpacing: '.08em' }}>YOU · 01:48 PM</div>
                <div style={{ background: 'var(--amber-bg-12)', border: '1px solid var(--border-default)', borderRadius: '12px 12px 4px 12px', padding: '8px 12px', fontSize: 13 }}>
                  давай еще добавим переключение на темную тему
                </div>
              </div>
              <div style={{ display: 'flex', alignItems: 'flex-start', gap: 8 }}>
                <span className="lk-mark is-sm">L</span>
                <div style={{ flex: 1, fontSize: 12.5, color: 'var(--text-secondary)', lineHeight: 1.5 }}>
                  Added a <span style={{ color: 'var(--text-primary)' }}>theme toggle</span> in the top bar. The preference persists in localStorage and follows OS preference on first load.
                </div>
              </div>
            </div>
            <Composer placeholder="Ask the agent to change anything…" />
          </div>
        </div>
      </div>
    </StudioCanvas>
  );
};

// ─── BUILDER · archived ──────────────────────────────────────────
window.BuilderArchived = function BuilderArchived() {
  return (
    <StudioCanvas>
      <div style={{ position: 'absolute', inset: 0, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', paddingBottom: 360 }}>
        <span className="lk-chip is-cyan" style={{ marginBottom: 22 }}>
          <Icon name="archive" size={10} /> ARCHIVED · EXPORT ONLY
        </span>
        <h1 className="lk-display" style={{ fontSize: 88, margin: 0, fontWeight: 200 }}>Playground archived</h1>
        <p style={{ fontSize: 15, color: 'var(--text-secondary)', marginTop: 18, maxWidth: 560, textAlign: 'center', lineHeight: 1.5 }}>
          This playground is export-only. The source remains downloadable for 90 days. Restore it to get a fresh Fibe playground and keep building.
        </p>
        <div style={{ display: 'flex', gap: 10, marginTop: 24 }}>
          <button className="lk-btn is-primary is-lg"><Icon name="refresh" size={14} /> Restore playground</button>
          <button className="lk-btn is-lg"><Icon name="download" size={14} /> Download source</button>
          <button className="lk-btn is-lg is-ghost"><Icon name="github" size={14} /> Push to GitHub</button>
        </div>
        <div className="lk-mono" style={{ fontSize: 10.5, color: 'var(--text-tertiary)', marginTop: 20, letterSpacing: '.08em' }}>
          ARCHIVED MAY 8 · SOURCE EXPIRES AUG 6 · 247 FILES · 12.4 MB
        </div>
      </div>

      <div style={{ position: 'absolute', left: '50%', transform: 'translateX(-50%)', bottom: 220 }}>
        <FibeTab />
      </div>

      <div className="lk-studio-chat" style={{
        position: 'absolute', left: '50%', transform: 'translateX(-50%)',
        bottom: 36, width: 760, opacity: 0.9,
      }}>
        <ChatHeader project="Blood Analysis webapp" />
        <div style={{ padding: 12 }}>
          <div style={{
            background: 'var(--bg-inset)', border: '1px solid var(--border-subtle)',
            borderRadius: 'var(--r-lg)', padding: 12,
            display: 'flex', alignItems: 'center', gap: 10,
          }}>
            <Icon name="archive" size={15} style={{ color: 'var(--cyan-dim)' }} />
            <span style={{ fontSize: 13, color: 'var(--text-secondary)' }}>
              This playground is archived. Export it or start a new one.
            </span>
            <span style={{ flex: 1 }} />
            <HourPill used="4h 26m" total="5h" />
            <button className="lk-btn is-cyan is-sm"><Icon name="plus" size={11} /> New playground</button>
          </div>
        </div>
      </div>
    </StudioCanvas>
  );
};

Object.assign(window, { ChatHeader, NowStrip, Composer });
