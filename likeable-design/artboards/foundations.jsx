// Likeable v2 — foundations: rationale, tokens, components inventory,
// error states, interaction notes

const { StudioCanvas, StatusChip, TimelineStep, SystemEvent, ServiceBadge,
        HourPill, QuotaBar, PreviewFrame, FibeTab, BloodAnalysisLIS, Icon,
        ChatHeader } = window;

// ─── Rationale: what changed and why ─────────────────────────────
window.Rationale = function Rationale() {
  return (
    <div className="lk-app lk-canvas" style={{ padding: 40, overflow: 'auto' }}>
      <div style={{ maxWidth: 900, display: 'flex', flexDirection: 'column', gap: 22 }}>
        <div>
          <span className="lk-eyebrow is-cyan">REDESIGN · v2</span>
          <h1 className="lk-display" style={{ fontSize: 44, margin: '12px 0 0', fontWeight: 200 }}>
            A studio that gets out of the way of the preview.
          </h1>
          <p style={{ margin: '14px 0 0', fontSize: 14, color: 'var(--text-secondary)', lineHeight: 1.55, maxWidth: 720 }}>
            Likeable's identity is a cinematic dark playground with amber controls and a glowing cyan headline.
            v2 keeps every piece of that DNA. What it fixes is everything that was making the floating chat
            feel like a modal blocking the product — not the brand, the behaviour.
          </p>
        </div>

        <div className="lk-divider" />

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: 14 }}>
          {[
            {
              tag: 'PRESERVED',
              tone: 'is-ok',
              h: 'The visual identity stays',
              p: 'Deep teal canvas + dot grid + corner brackets + cyan-mint headlines + amber control colour + the "Powered by FIBE.GG" tab. None of that was the problem.',
            },
            {
              tag: 'FIXED',
              tone: 'is-amber',
              h: 'Preview is never obscured',
              p: 'The floating chat keeps its position but gets glass-blur so the preview shows through. Split mode promotes it from "trick" to first-class layout with a tweak toggle.',
            },
            {
              tag: 'FIXED',
              tone: 'is-amber',
              h: 'Labelled controls, not icon soup',
              p: 'The header row of 8 icon-only buttons becomes a service pill, playground count, language, layout toggle, and a labelled "Profile" — same density, half the guessing.',
            },
            {
              tag: 'FIXED',
              tone: 'is-amber',
              h: 'Internal markers never leak',
              p: '[[LIKEABLE_NOTIFICATION_START]] is now a System Event row with a check icon, a friendly label like "Preview updated — dark-mode added", and a "View diff" affordance.',
            },
            {
              tag: 'NEW',
              tone: 'is-cyan',
              h: '"Now" strip + Build run timeline',
              p: 'Above the chat history, a single chip answers "what is happening". When the agent runs, a 5-step timeline replaces opaque thinking with an event log + steerable controls.',
            },
            {
              tag: 'NEW',
              tone: 'is-cyan',
              h: 'Quota adapts to configuration',
              p: 'Build hours are big and amber-glowing. If Stripe is not configured, the paid pack card explicitly says so instead of dominating the layout with a non-functional CTA.',
            },
            {
              tag: 'NEW',
              tone: 'is-cyan',
              h: 'Admin gains hierarchy',
              p: 'KPI strip first, then the single-customer detail row with its real chips (NO GITHUB, UNPAID 89/31). Access, recovery, and the integration list each get their own card.',
            },
            {
              tag: 'NEW',
              tone: 'is-cyan',
              h: 'Mobile leads with the preview',
              p: '390-wide builder. Preview is full-bleed. Service pill and inspect FAB float at the top. Chat lives in a snap-pointed bottom sheet with a real composer.',
            },
          ].map(c => (
            <div key={c.h} className="lk-card" style={{ padding: 16 }}>
              <span className={`lk-chip ${c.tone}`} style={{ height: 20 }}>{c.tag}</span>
              <h3 style={{ fontSize: 14.5, fontWeight: 600, margin: '10px 0 8px', letterSpacing: '-0.005em' }}>{c.h}</h3>
              <p style={{ margin: 0, fontSize: 12, color: 'var(--text-secondary)', lineHeight: 1.5 }}>{c.p}</p>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
};

// ─── Tokens ──────────────────────────────────────────────────────
window.Tokens = function Tokens() {
  const Swatch = ({ name, v, use, fg }) => (
    <div style={{ background: 'var(--bg-surface)', border: '1px solid var(--border-subtle)', borderRadius: 'var(--r-md)', overflow: 'hidden' }}>
      <div style={{ height: 56, background: v, position: 'relative' }}>
        {fg && <span style={{ position: 'absolute', bottom: 6, left: 10, color: fg, fontSize: 11, fontFamily: 'var(--font-mono)' }}>Aa</span>}
      </div>
      <div style={{ padding: '8px 10px', borderTop: '1px solid var(--border-hairline)' }}>
        <div className="lk-mono" style={{ fontSize: 10.5 }}>{name}</div>
        <div className="lk-mono" style={{ fontSize: 9.5, color: 'var(--text-tertiary)', marginTop: 2 }}>{v}</div>
        {use && <div style={{ fontSize: 10.5, color: 'var(--text-secondary)', marginTop: 4 }}>{use}</div>}
      </div>
    </div>
  );

  return (
    <div className="lk-app lk-canvas" style={{ padding: 32, overflow: 'auto' }}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 26, maxWidth: 1080 }}>
        <div>
          <span className="lk-eyebrow is-cyan">FOUNDATIONS</span>
          <h1 className="lk-display" style={{ fontSize: 32, margin: '10px 0 0', fontWeight: 200 }}>Design tokens</h1>
        </div>

        {/* Surface */}
        <div>
          <div className="lk-eyebrow" style={{ marginBottom: 10 }}>SURFACE</div>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(5, 1fr)', gap: 8 }}>
            <Swatch name="bg-base" v="#06090c" use="App shell" />
            <Swatch name="bg-surface" v="#0a141a" use="Chat panel" />
            <Swatch name="bg-elevated" v="#0e1b22" use="Cards" />
            <Swatch name="bg-inset" v="#08121a" use="Inputs, code" />
            <Swatch name="bg-overlay" v="#122029" use="Pills" />
          </div>
        </div>

        {/* Accent — amber + cyan */}
        <div>
          <div className="lk-eyebrow" style={{ marginBottom: 10 }}>SIGNATURE COLOURS</div>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(5, 1fr)', gap: 8 }}>
            <Swatch name="amber" v="#d4953c" use="Primary controls, FIBE.GG" fg="#1a1108" />
            <Swatch name="amber-bright" v="#e8b56f" use="Hover, focus glow" fg="#1a1108" />
            <Swatch name="amber-dim" v="#a06c27" use="Muted accent" />
            <Swatch name="cyan" v="#aef5ff" use="Headline glow, archived" fg="#0a141a" />
            <Swatch name="cyan-dim" v="#6ed8e8" use="Cyan-secondary" />
          </div>
        </div>

        {/* Status */}
        <div>
          <div className="lk-eyebrow" style={{ marginBottom: 10 }}>STATUS</div>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 8 }}>
            <Swatch name="ok" v="#5ed4a8" use="Live, ready" />
            <Swatch name="warn" v="#e8b56f" use="Paused, starting" />
            <Swatch name="danger" v="#ff7a7a" use="Errors, destructive" />
            <Swatch name="info" v="#7ec8ff" use="Tips, notices" />
          </div>
        </div>

        {/* Text */}
        <div>
          <div className="lk-eyebrow" style={{ marginBottom: 10 }}>TEXT</div>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 8 }}>
            {[
              { n: 'text-primary',   v: '#e6edf2' },
              { n: 'text-secondary', v: '#8d99a3' },
              { n: 'text-tertiary',  v: '#5a6770' },
              { n: 'text-disabled',  v: '#3a4248' },
            ].map(c => (
              <div key={c.n} style={{ background: 'var(--bg-surface)', borderRadius: 'var(--r-md)', border: '1px solid var(--border-subtle)', padding: 14 }}>
                <div style={{ fontSize: 22, fontWeight: 500, color: c.v, letterSpacing: '-0.01em' }}>Aa</div>
                <div className="lk-mono" style={{ fontSize: 10.5, marginTop: 8 }}>{c.n}</div>
                <div className="lk-mono" style={{ fontSize: 9.5, color: 'var(--text-tertiary)', marginTop: 2 }}>{c.v}</div>
              </div>
            ))}
          </div>
        </div>

        {/* Type */}
        <div>
          <div className="lk-eyebrow" style={{ marginBottom: 10 }}>TYPE · GEIST + JETBRAINS MONO</div>
          <div style={{ display: 'grid', gridTemplateColumns: '1.4fr 1fr', gap: 10 }}>
            <div className="lk-card" style={{ padding: 18 }}>
              <span className="lk-mono" style={{ fontSize: 10, color: 'var(--text-tertiary)' }}>HEADLINE · CYAN-MINT GLOW</span>
              <div className="lk-display" style={{ fontSize: 40, marginTop: 6, fontWeight: 200 }}>Playground archived</div>
              <div style={{ fontSize: 22, fontWeight: 500, marginTop: 8, letterSpacing: '-0.01em' }}>Blood Analysis webapp</div>
              <div style={{ fontSize: 13, color: 'var(--text-secondary)', marginTop: 4 }}>Body — what the agent did, in conversational language.</div>
              <div style={{ display: 'flex', gap: 14, marginTop: 14, fontSize: 10.5, color: 'var(--text-tertiary)', fontFamily: 'var(--font-mono)' }}>
                <span>40/200 cyan</span><span>22/500</span><span>13/400</span><span>11/500 mono</span>
              </div>
            </div>
            <div className="lk-card" style={{ padding: 18, fontFamily: 'var(--font-mono)' }}>
              <span style={{ fontSize: 10, color: 'var(--text-tertiary)' }}>MONO · JETBRAINS</span>
              <div style={{ fontSize: 14, marginTop: 8 }}>src/views/BoardView.tsx</div>
              <div style={{ fontSize: 11, color: 'var(--text-secondary)', marginTop: 6 }}>BUILD 0247 · UPDATED 12s AGO</div>
              <div style={{ fontSize: 11, color: 'var(--text-tertiary)', marginTop: 4 }}>4h 26m / 5h · 89/31</div>
              <div style={{ fontSize: 11, color: 'var(--ok)', marginTop: 12 }}>+38 −12 · 4 files</div>
            </div>
          </div>
        </div>

        {/* Spacing + radius + shadow */}
        <div style={{ display: 'grid', gridTemplateColumns: '1.4fr 1fr 1fr', gap: 10 }}>
          <div className="lk-card" style={{ padding: 16 }}>
            <div className="lk-eyebrow">SPACING · 4PX BASE</div>
            <div style={{ display: 'flex', alignItems: 'flex-end', gap: 8, marginTop: 14, height: 60 }}>
              {[
                { n: '1', v: 4 }, { n: '2', v: 8 }, { n: '3', v: 12 }, { n: '4', v: 16 },
                { n: '5', v: 20 }, { n: '6', v: 24 }, { n: '8', v: 32 }, { n: '10', v: 40 }, { n: '12', v: 48 },
              ].map(s => (
                <div key={s.n} style={{ textAlign: 'center' }}>
                  <div style={{ width: s.v, height: s.v, background: 'var(--amber)', borderRadius: 2 }} />
                  <div className="lk-mono" style={{ fontSize: 9.5, color: 'var(--text-tertiary)', marginTop: 5 }}>{s.n}</div>
                </div>
              ))}
            </div>
          </div>
          <div className="lk-card" style={{ padding: 16 }}>
            <div className="lk-eyebrow">RADIUS</div>
            <div style={{ display: 'flex', alignItems: 'flex-end', gap: 6, marginTop: 14 }}>
              {[{ n: 'sm', v: 6 }, { n: 'md', v: 10 }, { n: 'lg', v: 14 }, { n: 'xl', v: 18 }].map(r => (
                <div key={r.n} style={{ textAlign: 'center' }}>
                  <div style={{ width: 36, height: 36, background: 'var(--bg-overlay)', borderRadius: r.v, border: '1px solid var(--border-strong)' }} />
                  <div className="lk-mono" style={{ fontSize: 9.5, color: 'var(--text-tertiary)', marginTop: 5 }}>{r.n}·{r.v}</div>
                </div>
              ))}
            </div>
          </div>
          <div className="lk-card" style={{ padding: 16 }}>
            <div className="lk-eyebrow">ELEVATION</div>
            <div style={{ display: 'flex', gap: 10, marginTop: 14 }}>
              {[
                { n: 'sm', b: 'var(--shadow-sm)' },
                { n: 'md', b: 'var(--shadow-md)' },
                { n: 'lg', b: 'var(--shadow-lg)' },
                { n: 'amber', b: 'var(--shadow-amber)' },
              ].map(s => (
                <div key={s.n} style={{ textAlign: 'center' }}>
                  <div style={{ width: 38, height: 38, background: 'var(--bg-elevated)', borderRadius: 8, boxShadow: s.b }} />
                  <div className="lk-mono" style={{ fontSize: 9.5, color: 'var(--text-tertiary)', marginTop: 5 }}>{s.n}</div>
                </div>
              ))}
            </div>
          </div>
        </div>

        {/* Brackets and Fibe tab */}
        <div>
          <div className="lk-eyebrow" style={{ marginBottom: 10 }}>STUDIO CHROME</div>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 }}>
            <div className="lk-card" style={{ padding: 24, position: 'relative', height: 140 }}>
              {/* corner brackets demo */}
              <span style={{ position: 'absolute', top: 8, left: 8, width: 18, height: 18, borderTop: '1.5px solid var(--amber)', borderLeft: '1.5px solid var(--amber)', borderTopLeftRadius: 4 }} />
              <span style={{ position: 'absolute', top: 8, right: 8, width: 18, height: 18, borderTop: '1.5px solid var(--amber)', borderRight: '1.5px solid var(--amber)', borderTopRightRadius: 4 }} />
              <span style={{ position: 'absolute', bottom: 8, left: 8, width: 18, height: 18, borderBottom: '1.5px solid var(--amber)', borderLeft: '1.5px solid var(--amber)', borderBottomLeftRadius: 4 }} />
              <span style={{ position: 'absolute', bottom: 8, right: 8, width: 18, height: 18, borderBottom: '1.5px solid var(--amber)', borderRight: '1.5px solid var(--amber)', borderBottomRightRadius: 4 }} />
              <div style={{ textAlign: 'center', paddingTop: 30 }}>
                <span className="lk-eyebrow">CORNER BRACKETS</span>
                <div style={{ fontSize: 12, color: 'var(--text-secondary)', marginTop: 8 }}>Cinematic frame around the studio canvas</div>
              </div>
            </div>
            <div className="lk-card" style={{ padding: 24, display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 12 }}>
              <FibeTab />
              <span className="lk-eyebrow">FIBE.GG ATTRIBUTION TAB</span>
              <div style={{ fontSize: 12, color: 'var(--text-secondary)', textAlign: 'center' }}>Sits above the chat panel like a film slate</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

// ─── Component inventory ──────────────────────────────────────────
window.Components = function Components() {
  const Section = ({ title, cols = 2, children }) => (
    <div className="lk-card" style={{ padding: 16 }}>
      <div className="lk-eyebrow" style={{ marginBottom: 12 }}>{title}</div>
      <div style={{ display: 'grid', gridTemplateColumns: `repeat(${cols}, 1fr)`, gap: 8 }}>{children}</div>
    </div>
  );
  const Demo = ({ children }) => (
    <div style={{ padding: 14, background: 'var(--bg-elevated)', borderRadius: 8, border: '1px solid var(--border-hairline)', display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 8 }}>
      {children}
    </div>
  );

  return (
    <div className="lk-app lk-canvas" style={{ padding: 32, overflow: 'auto' }}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 14, maxWidth: 1080 }}>
        <div>
          <span className="lk-eyebrow is-cyan">FOUNDATIONS</span>
          <h1 className="lk-display" style={{ fontSize: 32, margin: '10px 0 0', fontWeight: 200 }}>Component inventory</h1>
        </div>

        <Section title="BUTTONS">
          <Demo>
            <button className="lk-btn is-primary"><Icon name="rocket" size={13} /> Start</button>
            <button className="lk-btn"><Icon name="github" size={13} /> Export</button>
            <button className="lk-btn is-ghost"><Icon name="sparkle" size={12} /> Improve</button>
            <button className="lk-btn is-danger"><Icon name="stop" size={11} stroke={2} /> Stop</button>
            <button className="lk-btn is-cyan"><Icon name="archive" size={12} /> Restore</button>
            <button className="lk-iconbtn is-bordered"><Icon name="settings" size={14} /></button>
          </Demo>
          <Demo>
            <button className="lk-btn is-lg is-primary">Continue <span className="lk-kbd">⌘⏎</span></button>
            <button className="lk-btn is-lg">Cancel</button>
            <button className="lk-btn is-sm">Small</button>
          </Demo>
        </Section>

        <Section title="STATUS CHIPS" cols={3}>
          <Demo><StatusChip state="live" /><StatusChip state="starting" /><StatusChip state="building" /></Demo>
          <Demo><StatusChip state="paused" /><StatusChip state="queued" /><StatusChip state="stopped" /></Demo>
          <Demo><StatusChip state="error" /><StatusChip state="archived" /><StatusChip state="ready" /></Demo>
        </Section>

        <Section title="HOUR PILL · QUOTA BAR">
          <Demo>
            <HourPill used="4h 26m" total="5h" />
            <HourPill used="4h 50m" total="5h" tone="warn" />
            <HourPill used="5h 00m" total="5h" tone="danger" />
          </Demo>
          <Demo>
            <div style={{ width: '100%' }}>
              <QuotaBar used={4.43} total={5} label="Build hours" sub="Resets in 12h 28m" />
            </div>
          </Demo>
        </Section>

        <Section title="SYSTEM EVENT · REPLACES INTERNAL MARKERS" cols={1}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            <SystemEvent kind="ok"     icon="check" label="Preview updated — dark-mode toggle added" sub="3 files · 12s ago" time="01:48" />
            <SystemEvent kind="info"   icon="info"  label="GitHub disconnected" sub="Reconnect to push directly" time="01:46" />
            <SystemEvent kind="amber"  icon="bolt"  label="Agent paused for input" sub="2 acceptance criteria need confirmation" time="01:48" />
            <SystemEvent kind="danger" icon="alert" label="Build failed at step 3" sub="ELIFECYCLE postinstall · sharp" time="01:49" />
          </div>
        </Section>

        <Section title="COMPOSER" cols={1}>
          <div style={{ background: 'var(--bg-inset)', border: '1px solid var(--border-subtle)', borderRadius: 'var(--r-lg)', padding: 10 }}>
            <div style={{ display: 'flex', alignItems: 'flex-start', gap: 10 }}>
              <button className="lk-iconbtn"><Icon name="paperclip" size={15} /></button>
              <div style={{ flex: 1, paddingTop: 7, fontSize: 13.5, color: 'var(--text-tertiary)' }}>Describe what you want to build…</div>
              <HourPill used="4h 26m" total="5h" />
              <button className="lk-btn is-primary" style={{ height: 32 }}><Icon name="send" size={13} /></button>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginTop: 8, paddingLeft: 38 }}>
              <button className="lk-btn is-sm is-ghost"><Icon name="sparkle" size={12} /> Improve prompt</button>
              <button className="lk-btn is-sm is-ghost"><Icon name="image" size={12} /> Reference</button>
              <button className="lk-btn is-sm is-ghost"><Icon name="cmd" size={12} /> Slash</button>
              <span style={{ flex: 1 }} />
              <span className="lk-mono" style={{ fontSize: 10, color: 'var(--text-tertiary)' }}>⏎ SEND · ⇧⏎ QUEUE</span>
            </div>
          </div>
        </Section>

        <Section title="BUILD RUN TIMELINE" cols={1}>
          <div className="lk-card-elev" style={{ padding: 14 }}>
            <TimelineStep state="done"    label="Prompt parsed"   time="00:01" />
            <TimelineStep state="done"    label="Planning"        detail="3 files identified" time="00:08" />
            <TimelineStep state="running" label="Editing files"   detail="3 of 4 written" time="00:42" />
            <TimelineStep state="queued"  label="Verifying" last />
          </div>
        </Section>

        <Section title="SERVICE BADGES · MULTI-SERVICE">
          <Demo>
            <div style={{ width: '100%' }}>
              <ServiceBadge kind="web"    name="app"     port="5173" status="live" active url="app.likeable.studio/blood-analysis" />
            </div>
          </Demo>
          <Demo>
            <div style={{ width: '100%' }}>
              <ServiceBadge kind="worker" name="reports" status="starting" />
            </div>
          </Demo>
        </Section>

        <Section title="HEADER ROW · LABELLED CONTROLS REPLACE ICON SOUP">
          <Demo>
            <button className="lk-iconbtn is-bordered"><Icon name="folder" size={13} /></button>
            <button style={{ display: 'flex', alignItems: 'center', gap: 6, height: 28, padding: '0 10px', background: 'var(--amber-bg-08)', border: '1px solid var(--border-default)', borderRadius: 999, color: 'var(--amber-bright)', cursor: 'pointer', font: '500 11px/1 var(--font-sans)' }}>
              <Icon name="web" size={11} /> app <Icon name="chevDown" size={11} />
            </button>
            <button style={{ display: 'flex', alignItems: 'center', gap: 6, height: 28, padding: '0 10px', background: 'transparent', border: '1px solid var(--border-default)', borderRadius: 999, color: 'var(--amber)', font: '500 11px/1 var(--font-mono)' }}>
              <Icon name="folder" size={11} /> 1/3
            </button>
            <button style={{ display: 'flex', alignItems: 'center', gap: 6, height: 28, padding: '0 10px', background: 'transparent', border: '1px solid var(--border-subtle)', borderRadius: 999, color: 'var(--text-secondary)', font: '500 11px/1 var(--font-mono)' }}>
              <Icon name="language" size={11} /> EN
            </button>
            <div style={{ display: 'flex', gap: 2, padding: 2, background: 'var(--bg-overlay)', borderRadius: 8, border: '1px solid var(--border-hairline)' }}>
              <button className="lk-iconbtn is-sm is-active"><Icon name="overlay" size={13} /></button>
              <button className="lk-iconbtn is-sm"><Icon name="split" size={13} /></button>
            </div>
          </Demo>
        </Section>
      </div>
    </div>
  );
};


// ─── Error state ─────────────────────────────────────────────────
window.ErrorStates = function ErrorStates() {
  return (
    <StudioCanvas>
      <div style={{ position: 'absolute', inset: 0, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', paddingBottom: 420 }}>
        <span className="lk-chip is-danger" style={{ marginBottom: 18 }}><span className="lk-dot" /> NEEDS ATTENTION</span>
        <h1 className="lk-display" style={{ fontSize: 56, margin: 0, fontWeight: 200, color: '#ffb0b0', textShadow: '0 0 24px rgba(255,122,122,.22)' }}>
          Build failed at step 3
        </h1>
        <p style={{ fontSize: 14, color: 'var(--text-secondary)', marginTop: 14, maxWidth: 540, textAlign: 'center', lineHeight: 1.5 }}>
          The dependency install crashed on <span className="lk-mono" style={{ color: 'var(--text-primary)' }}>sharp@0.33.0</span>. The agent ranked three fixes by historical success.
          You're not charged for build hours while the project is in this state.
        </p>

        <div style={{ display: 'grid', gridTemplateColumns: '1.3fr 1fr', gap: 12, width: 920, marginTop: 28 }}>
          <div className="lk-card" style={{ padding: 0, overflow: 'hidden' }}>
            <div style={{ padding: '8px 14px', borderBottom: '1px solid var(--border-hairline)', display: 'flex', alignItems: 'center', gap: 8 }}>
              <span className="lk-eyebrow">PNPM INSTALL · STDERR</span>
              <span style={{ flex: 1 }} />
              <button className="lk-iconbtn is-sm"><Icon name="download" size={12} /></button>
            </div>
            <div className="lk-mono" style={{ padding: 14, fontSize: 11.5, color: 'var(--text-secondary)', lineHeight: 1.55, background: 'var(--bg-inset)', minHeight: 240 }}>
              <div style={{ color: 'var(--text-tertiary)' }}>14:22:01 ▸ pnpm install --frozen-lockfile</div>
              <div>14:22:02 ▸ Resolving 487 packages</div>
              <div>14:22:04 ▸ Fetching @stripe/stripe-js 4.2.0</div>
              <div style={{ color: 'var(--warn)' }}>14:22:06 ⚠ peer react@^19 wanted by some-lib · found 18.3</div>
              <div style={{ color: 'var(--danger)' }}>14:22:08 ✗ ELIFECYCLE postinstall hook of <strong>sharp@0.33.0</strong> failed</div>
              <div style={{ color: 'var(--danger)' }}>14:22:08 ✗ No prebuilt binary for linux-arm64 musl</div>
              <div style={{ color: 'var(--text-tertiary)' }}>14:22:08 ▸ Process exited with code 1</div>
              <div style={{ color: 'var(--text-tertiary)', marginTop: 8 }}>{'─'.repeat(40)}</div>
              <div style={{ marginTop: 6 }}>
                <span style={{ color: 'var(--amber-bright)' }}>likeable@agent</span>: native binary mismatch on
                <span style={{ color: 'var(--text-primary)' }}> sharp</span>. Suggested fix below.
              </div>
            </div>
          </div>

          <div className="lk-card" style={{ padding: 14, display: 'flex', flexDirection: 'column', gap: 10 }}>
            <div className="lk-eyebrow">SUGGESTED RECOVERIES</div>
            {[
              { p: '92%', t: 'Replace sharp with @squoosh/lib', d: 'Pure-WASM image lib · no native binary', top: true },
              { p: '64%', t: 'Pin Node to 20.11',                d: 'Older minor with cached prebuilds' },
              { p: '38%', t: 'Use Linux x86 playground',         d: 'Restart with different arch' },
            ].map(r => (
              <div key={r.t} style={{
                padding: 12,
                background: r.top ? 'var(--amber-bg-08)' : 'var(--bg-elevated)',
                border: `1px solid ${r.top ? 'var(--border-default)' : 'var(--border-subtle)'}`,
                borderRadius: 'var(--r-md)',
                display: 'flex', alignItems: 'center', gap: 10,
              }}>
                <span className="lk-num lk-mono" style={{ fontSize: 13, fontWeight: 600, color: r.top ? 'var(--amber-bright)' : 'var(--text-secondary)', minWidth: 40 }}>{r.p}</span>
                <div style={{ flex: 1 }}>
                  <div style={{ fontSize: 12.5, fontWeight: 500 }}>{r.t}</div>
                  <div style={{ fontSize: 11, color: 'var(--text-tertiary)' }}>{r.d}</div>
                </div>
                <button className={`lk-btn is-sm ${r.top ? 'is-primary' : ''}`}>Apply</button>
              </div>
            ))}
            <div style={{ display: 'flex', gap: 6, marginTop: 4 }}>
              <button className="lk-btn is-sm"><Icon name="refresh" size={11} /> Retry as-is</button>
              <button className="lk-btn is-sm"><Icon name="sparkle" size={11} /> Ask agent to fix</button>
            </div>
          </div>
        </div>
      </div>

      <div style={{ position: 'absolute', left: '50%', transform: 'translateX(-50%)', bottom: 36, width: 720 }}>
        <FibeTab />
        <div className="lk-studio-chat" style={{ marginTop: -1 }}>
          <ChatHeader project="Pricing experiments" count="2/3" />
          <div style={{ padding: 12 }}>
            <SystemEvent kind="danger" icon="alert" label="Build 0090 failed at install" sub="ELIFECYCLE · sharp@0.33.0 · 4s" action={<button className="lk-btn is-sm">View log</button>} time="01:49" />
          </div>
        </div>
      </div>
    </StudioCanvas>
  );
};

// ─── Interaction notes ───────────────────────────────────────────
window.InteractionNotes = function InteractionNotes() {
  const flows = [
    {
      t: 'First prompt → live preview',
      steps: [
        'Onboarding wizard shows the same "Створення через опис" tutorial while the first playground boots. Start enables when preview is reachable.',
        'On submit, "Now" strip flips to "Building" and Build Run timeline expands. Build hours start counting only when /preview returns 200, never during boot.',
        'When the build finishes, the timeline collapses into a single System Event ("Preview updated — 3 files · 12s") that you can expand.',
      ],
    },
    {
      t: 'Agent running · steer vs queue',
      steps: [
        'While the agent is mid-build, the composer footer flips to "AGENT RUNNING · MSG WILL QUEUE" and the Stop button turns red.',
        '⏎ queues; ⇧⏎ sends a steer that interrupts at the next safe checkpoint. Both states are labelled — never icon-only.',
        'Stopping the agent leaves files in their current state and keeps the playground live, so /preview keeps serving the last good version.',
      ],
    },
    {
      t: 'Export · ZIP or GitHub',
      steps: [
        'Export dialog opens with a readiness checklist. Any non-ready item shows an inline action (Reconnect, Generate, Build now).',
        'If GitHub is disconnected, the GitHub tile is selectable but shows DISCONNECTED + Reconnect; selecting it starts OAuth without closing the dialog.',
        'ZIP downloads stream directly from Fibe; a small "Export ready" pill stays in the chat header even after the dialog closes.',
      ],
    },
    {
      t: 'Service switching',
      steps: [
        'Service pill on the chat header opens a panel listing every service (web · api · worker). One is marked PREVIEWING; clicking another swaps preview without restarting the others.',
        'On mobile the panel becomes a full-screen sheet; the same pill is at the top-left of the preview area.',
        'Deep-links inside the preview survive a switch — the URL bar updates to the new service host.',
      ],
    },
    {
      t: 'Lifecycle · stop · archive · restore',
      steps: [
        'States: creating → launching → ready → (stopped · error · maintenance) ↔ archived → deleting. Status chip in the chat header and project list always agree.',
        'Stopping idles the Fibe cell after 15s. Resuming reuses the same cell when possible; preview stays bookmarked.',
        'Archive snapshots the source and flips the project into export-only mode — composer locks, cyan-mint "Playground archived" headline appears in the preview.',
      ],
    },
    {
      t: 'Quota pressure · adapts to config',
      steps: [
        'Free hours show as a glowing amber readout in profile and in the composer pill. Resets every 24h.',
        'At 90% the pill turns warn-amber; at 100% the agent pauses but live playgrounds keep serving. Export remains free at any quota.',
        'Paid hour packs are hidden unless the admin has configured Stripe. The profile card explicitly says "not enabled" instead of dangling a non-functional CTA.',
      ],
    },
    {
      t: 'Internal markers · normalised',
      steps: [
        '[[LIKEABLE_NOTIFICATION_START]] / _END_ tokens never render as user-facing chat text. Anything between them parses into a System Event row.',
        'A SystemEvent has icon + label + sub + time + optional inline action ("View diff", "Reconnect"). Type is one of ok · info · amber · danger.',
        'If parsing fails, the raw block is silently dropped from the chat and forwarded to telemetry — never shown.',
      ],
    },
    {
      t: 'Mobile-first interactions',
      steps: [
        'Preview is full-bleed under a 44pt status bar and a compact project bar. Service pill and inspect FAB float at the top.',
        'Chat lives in a bottom sheet with three snap points: peek (just the Now strip), mid (composer + last event), expanded (full conversation).',
        'All composer actions are 32pt minimum — no icon-only 18pt rows. Stop stays a red labelled chip.',
      ],
    },
  ];

  return (
    <div className="lk-app lk-canvas" style={{ padding: 32, overflow: 'auto' }}>
      <div style={{ maxWidth: 960, display: 'flex', flexDirection: 'column', gap: 18 }}>
        <div>
          <span className="lk-eyebrow is-cyan">FOUNDATIONS</span>
          <h1 className="lk-display" style={{ fontSize: 32, margin: '10px 0 0', fontWeight: 200 }}>Interaction notes</h1>
          <p style={{ margin: '10px 0 0', fontSize: 13.5, color: 'var(--text-secondary)', maxWidth: 640, lineHeight: 1.55 }}>
            The flows that hold the studio together. Each is reachable from the keyboard, all of them stop short of being a wizard.
          </p>
        </div>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: 10 }}>
          {flows.map((f, idx) => (
            <div key={f.t} className="lk-card" style={{ padding: 16 }}>
              <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, marginBottom: 10 }}>
                <span className="lk-mono lk-num" style={{ fontSize: 10, color: 'var(--amber)', letterSpacing: '.12em' }}>FLOW · {String(idx + 1).padStart(2, '0')}</span>
              </div>
              <h3 style={{ fontSize: 14, fontWeight: 600, margin: '0 0 12px', letterSpacing: '-0.005em' }}>{f.t}</h3>
              <ol style={{ margin: 0, padding: 0, listStyle: 'none', display: 'flex', flexDirection: 'column', gap: 8 }}>
                {f.steps.map((s, i) => (
                  <li key={i} style={{ display: 'flex', gap: 10, fontSize: 12, color: 'var(--text-secondary)', lineHeight: 1.5 }}>
                    <span className="lk-mono lk-num" style={{ flex: '0 0 18px', color: 'var(--text-tertiary)', paddingTop: 1 }}>{String(i + 1).padStart(2, '0')}</span>
                    <span>{s}</span>
                  </li>
                ))}
              </ol>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
};
