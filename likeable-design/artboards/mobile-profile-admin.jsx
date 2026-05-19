// Likeable v2 — mobile, profile, admin

const { StudioCanvas, StatusChip, TimelineStep, SystemEvent, ServiceBadge,
        HourPill, QuotaBar, PreviewFrame, FibeTab, BloodAnalysisLIS, Icon } = window;

// ─── Mobile builder ──────────────────────────────────────────────
const MobileStatusBar = () => (
  <div style={{
    height: 44, padding: '0 22px',
    display: 'flex', alignItems: 'center', justifyContent: 'space-between',
    color: 'var(--text-primary)', fontSize: 14, fontWeight: 600,
    flex: '0 0 auto',
  }}>
    <span className="lk-mono">9:41</span>
    <span style={{ width: 24, height: 8, borderRadius: 999, background: 'var(--text-primary)' }} />
    <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
      <svg width="16" height="11" viewBox="0 0 16 11" fill="currentColor"><path d="M1 4l1.5-1.5L4 4 5.5 2.5 7 4 8.5 2.5 10 4 11.5 2.5 13 4 14.5 2.5L16 4v3l-1.5 1.5L13 7l-1.5 1.5L10 7 8.5 8.5 7 7 5.5 8.5 4 7 2.5 8.5 1 7z" opacity=".8"/></svg>
      <span style={{ width: 18, height: 10, border: '1px solid currentColor', borderRadius: 2, position: 'relative' }}>
        <span style={{ position: 'absolute', inset: 1, background: 'currentColor', width: '70%' }} />
      </span>
    </span>
  </div>
);

window.MobileBuilder = function MobileBuilder() {
  return (
    <div className="lk-app lk-canvas lk-brackets" style={{ position: 'relative', display: 'flex', flexDirection: 'column' }}>
      <span className="br-tl" />
      <span className="br-tr" />
      <MobileStatusBar />

      {/* Compact project bar */}
      <div style={{
        padding: '6px 12px', display: 'flex', alignItems: 'center', gap: 8,
        borderBottom: '1px solid var(--border-hairline)',
      }}>
        <button className="lk-iconbtn is-sm"><Icon name="menu" size={15} /></button>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ fontSize: 12.5, fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            Blood Analysis webapp
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginTop: 1 }}>
            <span className="lk-dot-st is-ok" style={{ width: 6, height: 6 }} />
            <span className="lk-mono" style={{ fontSize: 9.5, color: 'var(--text-tertiary)', letterSpacing: '.08em' }}>LIVE · APP · 4H 26M / 5H</span>
          </div>
        </div>
        <button className="lk-iconbtn is-sm"><Icon name="folder" size={14} /></button>
        <button className="lk-iconbtn is-sm"><Icon name="dots" size={14} /></button>
      </div>

      {/* Preview — full bleed under sheet */}
      <div style={{ flex: 1, position: 'relative', overflow: 'hidden', padding: 8 }}>
        <div style={{ position: 'absolute', inset: 8, borderRadius: 'var(--r-lg)', overflow: 'hidden', background: '#fff' }}>
          <BloodAnalysisLIS />
        </div>

        {/* Floating service pill */}
        <div style={{
          position: 'absolute', top: 18, left: 18,
          display: 'flex', alignItems: 'center', gap: 6,
          padding: '0 10px', height: 26,
          background: 'rgba(10,20,26,0.88)', backdropFilter: 'blur(10px)',
          borderRadius: 999, border: '1px solid var(--border-default)',
          fontSize: 11, color: 'var(--amber-bright)', fontWeight: 500,
        }}>
          <Icon name="web" size={11} /> app
          <Icon name="chevDown" size={11} />
        </div>

        {/* Inspect FAB */}
        <button style={{
          position: 'absolute', top: 18, right: 18,
          width: 32, height: 32, borderRadius: 999,
          background: 'rgba(10,20,26,0.88)', backdropFilter: 'blur(10px)',
          border: '1px solid var(--border-default)',
          color: 'var(--amber)', cursor: 'pointer',
          display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
        }}><Icon name="target" size={14} /></button>
      </div>

      {/* Bottom chat sheet — snapped to mid */}
      <div style={{
        background: 'rgba(8,16,22,0.92)',
        backdropFilter: 'blur(22px)',
        borderTop: '1px solid var(--border-default)',
        borderRadius: '20px 20px 0 0',
        boxShadow: '0 -10px 30px rgba(0,0,0,.5)',
        paddingBottom: 22, flex: '0 0 auto',
      }}>
        <div style={{ display: 'flex', justifyContent: 'center', padding: '8px 0 4px' }}>
          <span style={{ width: 36, height: 4, borderRadius: 2, background: 'var(--border-strong)' }} />
        </div>

        {/* Now strip */}
        <div style={{ display: 'flex', alignItems: 'center', padding: '4px 14px 10px', gap: 8 }}>
          <span className="lk-chip is-ok"><span className="lk-dot" /> LIVE · APP</span>
          <span style={{ flex: 1 }} />
          <button className="lk-iconbtn is-sm"><Icon name="history" size={14} /></button>
          <button className="lk-iconbtn is-sm"><Icon name="folder" size={14} /></button>
        </div>

        {/* Last system event */}
        <div style={{ padding: '0 12px 10px' }}>
          <SystemEvent kind="ok" icon="check" label="Preview updated · dark-mode toggle added" sub="3 files · 12s" time="01:48" />
        </div>

        {/* Composer */}
        <div style={{ padding: '0 12px' }}>
          <div style={{
            background: 'var(--bg-inset)',
            border: '1px solid var(--border-subtle)',
            borderRadius: 'var(--r-lg)',
            padding: 10, minHeight: 72,
            display: 'flex', flexDirection: 'column', gap: 10,
          }}>
            <div style={{ fontSize: 13.5, color: 'var(--text-tertiary)', paddingTop: 2, minHeight: 22 }}>
              Ask the agent to change anything…
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
              <button className="lk-iconbtn is-bordered" style={{ width: 32, height: 32 }}><Icon name="paperclip" size={14} /></button>
              <button className="lk-iconbtn is-bordered" style={{ width: 32, height: 32 }}><Icon name="sparkle" size={14} /></button>
              <HourPill used="4h 26m" total="5h" />
              <span style={{ flex: 1 }} />
              <button className="lk-btn is-primary" style={{ height: 32, width: 44 }}><Icon name="send" size={14} /></button>
            </div>
          </div>
        </div>
      </div>

      <span className="br-bl" />
      <span className="br-br" />
    </div>
  );
};

// ─── Mobile chat fully expanded ──────────────────────────────────
window.MobileChatExpanded = function MobileChatExpanded() {
  return (
    <div className="lk-app lk-canvas lk-brackets" style={{ display: 'flex', flexDirection: 'column' }}>
      <span className="br-tl" /><span className="br-tr" />
      <MobileStatusBar />
      <div style={{
        padding: '8px 12px', display: 'flex', alignItems: 'center', gap: 8,
        borderBottom: '1px solid var(--border-hairline)',
      }}>
        <button className="lk-iconbtn is-sm"><Icon name="chevDown" size={15} /></button>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ fontSize: 12.5, fontWeight: 600 }}>Build chat · Blood Analysis</div>
          <div className="lk-mono" style={{ fontSize: 9.5, color: 'var(--text-tertiary)', letterSpacing: '.08em' }}>BUILD 0247 · AGENT WORKING 00:42</div>
        </div>
        <span className="lk-chip is-amber is-loading"><span className="lk-dot" /> 00:42</span>
      </div>

      <div style={{ flex: 1, overflow: 'auto', padding: 12, display: 'flex', flexDirection: 'column', gap: 10 }}>
        <SystemEvent kind="ok" icon="check" label="GitHub connected" sub="kateryna/blood-analysis" time="01:46" />
        <div style={{ alignSelf: 'flex-end', maxWidth: '88%' }}>
          <div style={{
            background: 'var(--amber-bg-12)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-primary)',
            borderRadius: '12px 12px 4px 12px',
            padding: '8px 12px', fontSize: 13,
          }}>
            Make the kanban board sortable by team
          </div>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span className="lk-mark is-sm">L</span>
          <span style={{ fontSize: 12, fontWeight: 600 }}>Agent</span>
          <span className="lk-chip is-amber is-loading"><span className="lk-dot" /> Working</span>
        </div>
        <div className="lk-card-elev" style={{ padding: 12 }}>
          <div className="lk-eyebrow" style={{ marginBottom: 8 }}>BUILD RUN</div>
          <TimelineStep state="done"    label="Planning"        time="00:08" />
          <TimelineStep state="done"    label="Editing files"   detail="3 of 4 written" time="00:24" />
          <TimelineStep state="running" label="Preview warming" detail="HMR · /board" time="00:42" last />
        </div>
      </div>

      <div style={{ padding: 12, borderTop: '1px solid var(--border-hairline)', background: 'var(--bg-surface)' }}>
        <div style={{ background: 'var(--bg-inset)', border: '1px solid var(--border-subtle)', borderRadius: 'var(--r-lg)', padding: 10 }}>
          <div style={{ fontSize: 13.5, color: 'var(--text-tertiary)', padding: '2px 4px 8px' }}>Steer the agent…</div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
            <button className="lk-iconbtn is-sm"><Icon name="paperclip" size={13} /></button>
            <button className="lk-iconbtn is-sm"><Icon name="sparkle" size={13} /></button>
            <span className="lk-mono" style={{ fontSize: 9.5, color: 'var(--text-tertiary)', flex: 1 }}>QUEUED · 1 MSG</span>
            <button className="lk-btn is-sm is-danger"><Icon name="stop" size={11} stroke={2} /> Stop</button>
            <button className="lk-btn is-sm is-primary"><Icon name="send" size={12} /></button>
          </div>
        </div>
      </div>
      <span className="br-bl" /><span className="br-br" />
    </div>
  );
};


// ─── Tab bar shared by Profile + Admin (matches the real product) ─
const TopTabs = ({ tab = 'builder', user = 'rayfesoul@gmail.com' }) => (
  <div style={{
    display: 'flex', alignItems: 'center', gap: 8,
    padding: '12px 22px',
    borderBottom: '1px solid var(--border-hairline)',
  }}>
    <span className="lk-mark is-sm">L</span>
    <span style={{ flex: 1 }} />
    <div style={{ display: 'flex', gap: 4, padding: 3, background: 'var(--bg-overlay)', borderRadius: 10, border: '1px solid var(--border-hairline)' }}>
      {[
        { id: 'builder', t: 'Builder',  i: 'cmd' },
        { id: 'profile', t: 'Profile',  i: 'user' },
        { id: 'admin',   t: 'Admin',    i: 'settings' },
      ].map(t => (
        <button key={t.id} style={{
          height: 30, padding: '0 14px',
          display: 'inline-flex', alignItems: 'center', gap: 6,
          background: t.id === tab ? 'var(--amber-bg-12)' : 'transparent',
          border: 'none', cursor: 'pointer',
          color: t.id === tab ? 'var(--amber-bright)' : 'var(--text-secondary)',
          fontSize: 12.5, fontWeight: t.id === tab ? 600 : 500,
          borderRadius: 7,
          fontFamily: 'inherit',
        }}>
          <Icon name={t.i} size={13} /> {t.t}
        </button>
      ))}
    </div>
    <span style={{ flex: 1 }} />
    <button className="lk-btn is-sm is-ghost"><Icon name="language" size={12} /> EN</button>
    <span style={{ fontSize: 12.5, color: 'var(--text-secondary)' }}>{user}</span>
    <button className="lk-btn is-sm"><Icon name="signout" size={12} /></button>
  </div>
);

// ─── Profile ─────────────────────────────────────────────────────
window.ProfileScreen = function ProfileScreen() {
  return (
    <div className="lk-app lk-canvas" style={{ display: 'flex', flexDirection: 'column' }}>
      <TopTabs tab="profile" />
      <div style={{ flex: 1, overflow: 'auto', padding: 28 }}>
        <div style={{ maxWidth: 1100, margin: '0 auto' }}>
          <div style={{ display: 'flex', alignItems: 'baseline', gap: 14, marginBottom: 22 }}>
            <span className="lk-eyebrow is-cyan">PROFILE</span>
            <h1 className="lk-display" style={{ fontSize: 36, margin: 0, fontWeight: 200, color: 'var(--text-primary)', textShadow: 'none' }}>rayfesoul@gmail.com</h1>
            <span style={{ flex: 1 }} />
            <button className="lk-btn is-sm is-ghost"><Icon name="trash" size={12} /> Delete account</button>
            <button className="lk-btn is-sm"><Icon name="signout" size={12} /> Sign out</button>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1.4fr 1fr', gap: 16 }}>
            {/* Left column */}
            <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
              {/* Identity */}
              <div className="lk-card" style={{ padding: 18 }}>
                <div className="lk-eyebrow" style={{ marginBottom: 14 }}>SIGNED IN AS</div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
                  <div style={{
                    width: 52, height: 52, borderRadius: '50%',
                    background: 'linear-gradient(135deg, var(--amber-bright), var(--amber-dim))',
                    color: 'var(--amber-fg)', fontSize: 19, fontWeight: 600,
                    display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
                    border: '1.5px solid var(--border-strong)',
                  }}>R</div>
                  <div style={{ flex: 1 }}>
                    <div style={{ fontSize: 16, fontWeight: 600 }}>rayfesoul</div>
                    <div className="lk-mono" style={{ fontSize: 11.5, color: 'var(--text-tertiary)', marginTop: 2 }}>rayfesoul@gmail.com</div>
                  </div>
                  <span className="lk-chip is-amber" style={{ height: 22 }}>ADMIN</span>
                </div>
              </div>

              {/* GitHub */}
              <div className="lk-card" style={{ padding: 18 }}>
                <div style={{ display: 'flex', alignItems: 'center' }}>
                  <div className="lk-eyebrow">GITHUB</div>
                  <span style={{ flex: 1 }} />
                  <span className="lk-chip is-warn" style={{ height: 20 }}><span className="lk-dot" /> NOT CONNECTED</span>
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 14, marginTop: 12 }}>
                  <Icon name="github" size={28} style={{ color: 'var(--text-secondary)' }} />
                  <div style={{ flex: 1 }}>
                    <div style={{ fontSize: 14, fontWeight: 600 }}>Export your repositories</div>
                    <div style={{ fontSize: 12, color: 'var(--text-secondary)', marginTop: 2, maxWidth: 360 }}>
                      Connect GitHub once. Likeable will push the build to a repo of your choice on every export.
                    </div>
                  </div>
                  <button className="lk-btn is-primary"><Icon name="github" size={13} /> Connect</button>
                </div>
              </div>

              {/* Tutorial */}
              <div className="lk-card" style={{ padding: 18 }}>
                <div className="lk-eyebrow" style={{ marginBottom: 12 }}>TUTORIAL</div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
                  <span style={{ width: 32, height: 32, borderRadius: 8, background: 'var(--cyan-bg-08)', color: 'var(--cyan)', display: 'inline-flex', alignItems: 'center', justifyContent: 'center' }}>
                    <Icon name="book" size={14} />
                  </span>
                  <div style={{ flex: 1 }}>
                    <div style={{ fontSize: 14, fontWeight: 600 }}>Open onboarding</div>
                    <div style={{ fontSize: 12, color: 'var(--text-secondary)', marginTop: 2 }}>Short guide on prompts, build hours, export and lifecycle.</div>
                  </div>
                  <button className="lk-btn"><Icon name="book" size={12} /> Open</button>
                </div>
              </div>

              {/* Playgrounds quick */}
              <div className="lk-card" style={{ padding: 18 }}>
                <div style={{ display: 'flex', alignItems: 'baseline', marginBottom: 12 }}>
                  <span className="lk-eyebrow">PLAYGROUNDS</span>
                  <span style={{ marginLeft: 8, fontSize: 11.5, color: 'var(--text-tertiary)' }}>2 of 3 slots</span>
                  <span style={{ flex: 1 }} />
                  <button className="lk-btn is-sm is-ghost">View all</button>
                </div>
                {[
                  { n: 'Blood Analysis webapp', s: 'ready',    sub: 'Workflow · LIS' },
                  { n: 'Pricing experiments',   s: 'paused',   sub: 'Internal tool' },
                  { n: 'New playground',        s: 'archived', sub: 'Product UI · export-only' },
                ].map(p => (
                  <div key={p.n} style={{
                    display: 'flex', alignItems: 'center', gap: 12,
                    padding: '10px 0',
                    borderTop: '1px solid var(--border-hairline)',
                  }}>
                    <span className="lk-mark is-sm">{p.n[0]}</span>
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <div style={{ fontSize: 13, fontWeight: 500 }}>{p.n}</div>
                      <div className="lk-mono" style={{ fontSize: 10.5, color: 'var(--text-tertiary)', letterSpacing: '.05em' }}>{p.sub.toUpperCase()}</div>
                    </div>
                    <StatusChip state={p.s} />
                    <button className="lk-iconbtn is-sm"><Icon name="dots" size={13} /></button>
                  </div>
                ))}
              </div>
            </div>

            {/* Right column — Build hours + support */}
            <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
              <div className="lk-card" style={{ padding: 18 }}>
                <div className="lk-eyebrow" style={{ marginBottom: 12 }}>BUILD HOURS</div>

                {/* Big readout */}
                <div style={{ display: 'flex', alignItems: 'baseline', gap: 10 }}>
                  <span className="lk-num" style={{ fontSize: 38, fontWeight: 200, letterSpacing: '-0.02em', color: 'var(--amber-bright)' }}>4h 26m</span>
                  <span className="lk-mono" style={{ fontSize: 12, color: 'var(--text-tertiary)' }}>/ 5h FREE PER 24h</span>
                </div>
                <div style={{ marginTop: 10 }}>
                  <QuotaBar used={4.43} total={5} label="" sub="Resets in 12h 28m" />
                </div>

                {/* Paid hours card — only shown if Stripe configured */}
                <div style={{
                  marginTop: 14, padding: 12,
                  background: 'var(--bg-elevated)',
                  border: '1px dashed var(--border-subtle)',
                  borderRadius: 'var(--r-md)',
                  display: 'flex', alignItems: 'flex-start', gap: 10,
                }}>
                  <Icon name="info" size={13} style={{ color: 'var(--text-tertiary)', marginTop: 2 }} />
                  <div style={{ flex: 1 }}>
                    <div style={{ fontSize: 12, fontWeight: 500, color: 'var(--text-secondary)' }}>Paid hour packs are not enabled</div>
                    <div style={{ fontSize: 11, color: 'var(--text-tertiary)', marginTop: 2 }}>This instance hasn't configured Stripe. Ask the admin to enable in Integrations.</div>
                  </div>
                </div>

                {/* Lifetime */}
                <div style={{ display: 'flex', alignItems: 'center', gap: 16, marginTop: 14, paddingTop: 12, borderTop: '1px solid var(--border-hairline)' }}>
                  <div>
                    <div className="lk-mono" style={{ fontSize: 10, color: 'var(--text-tertiary)', letterSpacing: '.08em' }}>LIFETIME</div>
                    <div className="lk-num" style={{ fontSize: 16, fontWeight: 600, marginTop: 2 }}>34 min</div>
                  </div>
                  <div>
                    <div className="lk-mono" style={{ fontSize: 10, color: 'var(--text-tertiary)', letterSpacing: '.08em' }}>BUILDS</div>
                    <div className="lk-num" style={{ fontSize: 16, fontWeight: 600, marginTop: 2 }}>0247</div>
                  </div>
                  <div>
                    <div className="lk-mono" style={{ fontSize: 10, color: 'var(--text-tertiary)', letterSpacing: '.08em' }}>EXPORTS</div>
                    <div className="lk-num" style={{ fontSize: 16, fontWeight: 600, marginTop: 2 }}>6</div>
                  </div>
                </div>
              </div>

              {/* Support */}
              <div className="lk-card" style={{ padding: 18 }}>
                <div className="lk-eyebrow" style={{ marginBottom: 12 }}>SUPPORT</div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 12 }}>
                  <span style={{ width: 32, height: 32, borderRadius: 8, background: 'var(--amber-bg-12)', color: 'var(--amber)', display: 'inline-flex', alignItems: 'center', justifyContent: 'center' }}>
                    <Icon name="flag" size={14} />
                  </span>
                  <div style={{ flex: 1 }}>
                    <div style={{ fontSize: 13, fontWeight: 600 }}>Open a support thread</div>
                    <div style={{ fontSize: 11.5, color: 'var(--text-tertiary)' }}>Reach the admin · usually replies same day</div>
                  </div>
                  <button className="lk-btn is-sm"><Icon name="plus" size={11} /> New</button>
                </div>
                {/* Threads */}
                {[
                  { t: 'Архів не качається на iPhone', s: 'admin replied · 2h ago', open: true },
                  { t: 'Як перепідключити GitHub?',    s: 'resolved · yesterday', open: false },
                ].map(m => (
                  <div key={m.t} style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '8px 0', borderTop: '1px solid var(--border-hairline)' }}>
                    <span className={`lk-dot-st ${m.open ? 'is-amber' : 'is-idle'}`} />
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <div style={{ fontSize: 12.5, color: 'var(--text-primary)' }}>{m.t}</div>
                      <div className="lk-mono" style={{ fontSize: 10, color: 'var(--text-tertiary)', letterSpacing: '.04em' }}>{m.s.toUpperCase()}</div>
                    </div>
                    <Icon name="chevRight" size={12} style={{ color: 'var(--text-tertiary)' }} />
                  </div>
                ))}
              </div>

              {/* Danger zone */}
              <div className="lk-card" style={{ padding: 18, borderColor: 'rgba(255,122,122,0.18)' }}>
                <div className="lk-eyebrow" style={{ color: 'var(--danger)', marginBottom: 12 }}>DANGER</div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                  <Icon name="trash" size={16} style={{ color: 'var(--danger)' }} />
                  <div style={{ flex: 1 }}>
                    <div style={{ fontSize: 13, fontWeight: 600 }}>Delete everything</div>
                    <div style={{ fontSize: 11.5, color: 'var(--text-tertiary)' }}>Removes all playgrounds, archives, and your account. Cannot be undone.</div>
                  </div>
                  <button className="lk-btn is-sm is-danger">Delete all</button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

// ─── Admin ───────────────────────────────────────────────────────
window.AdminDashboard = function AdminDashboard() {
  return (
    <div className="lk-app lk-canvas" style={{ display: 'flex', flexDirection: 'column' }}>
      <TopTabs tab="admin" />
      <div style={{ flex: 1, overflow: 'auto', padding: 28 }}>
        <div style={{ maxWidth: 1240, margin: '0 auto' }}>
          {/* Header */}
          <div style={{ display: 'flex', alignItems: 'baseline', gap: 14, marginBottom: 22 }}>
            <span className="lk-eyebrow is-cyan">ADMIN</span>
            <h1 className="lk-display" style={{ fontSize: 36, margin: 0, fontWeight: 200, color: 'var(--text-primary)', textShadow: 'none' }}>Operations</h1>
            <span style={{ flex: 1 }} />
            <span className="lk-chip"><span className="lk-dot-st is-ok" /> SINGLE-USER DEPLOYMENT · SIGNUPS CLOSED</span>
            <button className="lk-btn is-sm"><Icon name="refresh" size={12} /> Refresh</button>
          </div>

          {/* KPI tiles */}
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 12, marginBottom: 20 }}>
            {[
              { l: 'Customers',        v: '1',    d: 'rayfesoul', acc: 'var(--amber)', i: 'user' },
              { l: 'Active playgrounds', v: '1',  d: '2 archived', acc: 'var(--ok)',    i: 'server' },
              { l: 'Agent pool',       v: '94%',  d: 'capacity',   acc: 'var(--cyan-dim)', i: 'cpu' },
              { l: 'Pending recoveries', v: '0',  d: 'all clear',  acc: 'var(--text-secondary)', i: 'pulse' },
            ].map(k => (
              <div key={k.l} className="lk-card" style={{ padding: 14 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
                  <span style={{ width: 22, height: 22, borderRadius: 5, background: 'rgba(0,0,0,0.25)', color: k.acc, display: 'inline-flex', alignItems: 'center', justifyContent: 'center' }}>
                    <Icon name={k.i} size={12} />
                  </span>
                  <span style={{ fontSize: 11.5, color: 'var(--text-secondary)' }}>{k.l}</span>
                </div>
                <div style={{ display: 'flex', alignItems: 'baseline', gap: 8 }}>
                  <span className="lk-num" style={{ fontSize: 28, fontWeight: 200, letterSpacing: '-0.02em' }}>{k.v}</span>
                  <span className="lk-mono" style={{ fontSize: 10.5, color: 'var(--text-tertiary)' }}>{k.d}</span>
                </div>
              </div>
            ))}
          </div>

          {/* Two-column: customers + system */}
          <div style={{ display: 'grid', gridTemplateColumns: '1.5fr 1fr', gap: 14 }}>
            {/* Customers */}
            <div className="lk-card" style={{ padding: 16 }}>
              <div style={{ display: 'flex', alignItems: 'baseline', marginBottom: 12 }}>
                <div className="lk-eyebrow">CUSTOMERS</div>
                <span style={{ marginLeft: 8, fontSize: 11.5, color: 'var(--text-tertiary)' }}>Search, filter, access control, cleanup</span>
                <span style={{ flex: 1 }} />
                <button className="lk-btn is-sm"><Icon name="filter" size={11} /> Filter</button>
              </div>

              {/* Filter bar */}
              <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr 1fr 1fr 1fr', gap: 8, marginBottom: 12 }}>
                <div style={{ position: 'relative' }}>
                  <Icon name="search" size={12} style={{ position: 'absolute', top: 11, left: 11, color: 'var(--text-tertiary)' }} />
                  <input className="lk-input" placeholder="email, name, id" style={{ paddingLeft: 30 }} />
                </div>
                {['Access · all', 'GitHub · all', 'Payment · all', 'Sort · newest'].map(t => (
                  <button key={t} className="lk-btn" style={{ height: 34, justifyContent: 'space-between' }}>
                    <span style={{ color: 'var(--text-secondary)', fontWeight: 400 }}>{t}</span>
                    <Icon name="chevDown" size={12} />
                  </button>
                ))}
              </div>

              {/* Customer row */}
              <div style={{
                padding: 14,
                background: 'var(--bg-elevated)', border: '1px solid var(--border-default)',
                borderRadius: 'var(--r-md)',
                display: 'flex', alignItems: 'center', gap: 14,
              }}>
                <span style={{ width: 36, height: 36, borderRadius: '50%', background: 'linear-gradient(135deg, var(--amber-bright), var(--amber-dim))', color: 'var(--amber-fg)', fontWeight: 600, display: 'inline-flex', alignItems: 'center', justifyContent: 'center' }}>R</span>
                <div style={{ flex: 1 }}>
                  <div style={{ display: 'flex', alignItems: 'baseline', gap: 8 }}>
                    <span style={{ fontSize: 13.5, fontWeight: 600 }}>rayfesoul@gmail.com</span>
                    <span className="lk-chip is-amber" style={{ height: 18, fontSize: 9 }}>ADMIN</span>
                  </div>
                  <div className="lk-mono" style={{ fontSize: 10.5, color: 'var(--text-tertiary)', marginTop: 2, letterSpacing: '.04em' }}>
                    JOINED MAY 1 · 1/3 SLOTS · 34M LIFETIME · BUILD 0247
                  </div>
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                  <span className="lk-mono lk-num" style={{ fontSize: 11, color: 'var(--amber-bright)', padding: '4px 8px', borderRadius: 4, background: 'var(--amber-bg-12)' }}>4h 26m / 5h</span>
                  <span className="lk-chip is-warn" style={{ height: 18, fontSize: 9 }}>NO GITHUB</span>
                  <span className="lk-chip" style={{ height: 18, fontSize: 9 }}>UNPAID 89/31</span>
                </div>
                <button className="lk-iconbtn is-sm"><Icon name="dots" size={13} /></button>
              </div>

              <div style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '10px 4px 0' }}>
                <button className="lk-btn is-sm is-ghost"><Icon name="chevLeft" size={11} /> Previous</button>
                <span style={{ flex: 1, fontSize: 11, color: 'var(--text-tertiary)', textAlign: 'center' }}>1 / 1 · 1 customer</span>
                <button className="lk-btn is-sm is-ghost">Next <Icon name="chevRight" size={11} /></button>
              </div>

              <div className="lk-divider" style={{ margin: '14px 0' }} />

              {/* Detail placeholder */}
              <div style={{
                padding: 28, textAlign: 'center',
                background: 'var(--bg-elevated)',
                border: '1px dashed var(--border-subtle)',
                borderRadius: 'var(--r-md)',
              }}>
                <Icon name="user" size={20} style={{ color: 'var(--text-tertiary)', marginBottom: 8 }} />
                <div style={{ fontSize: 12.5, color: 'var(--text-secondary)' }}>Select a customer to view playgrounds, payments, notices and access.</div>
              </div>
            </div>

            {/* System column */}
            <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
              {/* Access */}
              <div className="lk-card" style={{ padding: 16 }}>
                <div className="lk-eyebrow" style={{ marginBottom: 12 }}>ACCESS</div>
                <p style={{ fontSize: 11.5, color: 'var(--text-secondary)', margin: '0 0 12px' }}>
                  Signups are closed by default. Use an email or domain allowlist, or open registration for everyone.
                </p>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                  {[
                    { l: 'Closed (current)', d: 'No one can sign up', sel: true },
                    { l: 'Allowlist', d: 'email or @domain · one per line', sel: false },
                    { l: 'Open', d: 'Anyone with the link', sel: false },
                  ].map(o => (
                    <label key={o.l} style={{
                      display: 'flex', alignItems: 'flex-start', gap: 10,
                      padding: 10,
                      background: o.sel ? 'var(--amber-bg-08)' : 'transparent',
                      border: `1px solid ${o.sel ? 'var(--border-default)' : 'var(--border-hairline)'}`,
                      borderRadius: 'var(--r-sm)',
                      cursor: 'pointer',
                    }}>
                      <span style={{
                        width: 14, height: 14, borderRadius: '50%',
                        border: `1.5px solid ${o.sel ? 'var(--amber)' : 'var(--border-strong)'}`,
                        background: o.sel ? 'radial-gradient(circle, var(--amber) 35%, transparent 38%)' : 'transparent',
                        marginTop: 2, flex: '0 0 auto',
                      }} />
                      <div style={{ flex: 1 }}>
                        <div style={{ fontSize: 12.5, fontWeight: 500 }}>{o.l}</div>
                        <div style={{ fontSize: 11, color: 'var(--text-tertiary)' }}>{o.d}</div>
                      </div>
                    </label>
                  ))}
                </div>
              </div>

              {/* Recovery */}
              <div className="lk-card" style={{ padding: 16 }}>
                <div style={{ display: 'flex', alignItems: 'baseline' }}>
                  <div className="lk-eyebrow">RECOVERY</div>
                  <span style={{ flex: 1 }} />
                  <button className="lk-btn is-sm"><Icon name="refresh" size={11} /> Sweep now</button>
                </div>
                <p style={{ fontSize: 11.5, color: 'var(--text-secondary)', margin: '8px 0 12px' }}>
                  Deletion + cleanup status. Auto-runs every 300s.
                </p>
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: 6 }}>
                  {[
                    { l: 'Accounts pending', v: '0' },
                    { l: 'Playgrounds deleting', v: '0' },
                    { l: 'Last sweep', v: 'every 300s' },
                    { l: 'Last checked', v: '14:32' },
                  ].map(s => (
                    <div key={s.l} style={{ padding: 10, background: 'var(--bg-elevated)', border: '1px solid var(--border-hairline)', borderRadius: 'var(--r-sm)' }}>
                      <div className="lk-mono" style={{ fontSize: 9.5, color: 'var(--text-tertiary)', letterSpacing: '.08em', textTransform: 'uppercase' }}>{s.l}</div>
                      <div className="lk-num" style={{ fontSize: 14, fontWeight: 500, marginTop: 3 }}>{s.v}</div>
                    </div>
                  ))}
                </div>
              </div>

              {/* Fibe integration & pool */}
              <div className="lk-card" style={{ padding: 16 }}>
                <div className="lk-eyebrow" style={{ marginBottom: 12 }}>FIBE INTEGRATION · AGENT POOL</div>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '8px 10px', background: 'var(--bg-elevated)', borderRadius: 6, border: '1px solid var(--border-hairline)' }}>
                    <Icon name="server" size={14} style={{ color: 'var(--ok)' }} />
                    <span style={{ fontSize: 12.5, flex: 1 }}>Fibe API</span>
                    <span className="lk-mono" style={{ fontSize: 10.5, color: 'var(--text-tertiary)' }}>us-east · v1.42</span>
                    <span className="lk-dot-st is-ok" />
                  </div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '8px 10px', background: 'var(--bg-elevated)', borderRadius: 6, border: '1px solid var(--border-hairline)' }}>
                    <Icon name="cpu" size={14} style={{ color: 'var(--amber)' }} />
                    <span style={{ fontSize: 12.5, flex: 1 }}>Agent pool</span>
                    <span className="lk-mono" style={{ fontSize: 10.5, color: 'var(--text-tertiary)' }}>3 of 4 ready</span>
                    <span className="lk-dot-st is-ok" />
                  </div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '8px 10px', background: 'var(--bg-elevated)', borderRadius: 6, border: '1px solid var(--border-hairline)' }}>
                    <Icon name="github" size={14} style={{ color: 'var(--ok)' }} />
                    <span style={{ fontSize: 12.5, flex: 1 }}>GitHub App</span>
                    <span className="lk-mono" style={{ fontSize: 10.5, color: 'var(--text-tertiary)' }}>configured</span>
                    <span className="lk-dot-st is-ok" />
                  </div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '8px 10px', background: 'var(--bg-elevated)', borderRadius: 6, border: '1px solid var(--border-hairline)' }}>
                    <Icon name="google" size={14} style={{ color: 'var(--ok)' }} />
                    <span style={{ fontSize: 12.5, flex: 1 }}>Google OAuth</span>
                    <span className="lk-mono" style={{ fontSize: 10.5, color: 'var(--text-tertiary)' }}>configured</span>
                    <span className="lk-dot-st is-ok" />
                  </div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '8px 10px', background: 'var(--bg-elevated)', borderRadius: 6, border: '1px dashed var(--border-subtle)' }}>
                    <Icon name="bolt" size={14} style={{ color: 'var(--text-tertiary)' }} />
                    <span style={{ fontSize: 12.5, flex: 1, color: 'var(--text-secondary)' }}>Stripe (paid hour packs)</span>
                    <span className="lk-mono" style={{ fontSize: 10.5, color: 'var(--text-tertiary)' }}>not configured</span>
                    <button className="lk-btn is-sm" style={{ height: 22, fontSize: 10.5, padding: '0 8px' }}>Configure</button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};
