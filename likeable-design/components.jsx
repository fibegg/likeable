// Likeable v2 — shared components + icons
// Cinematic studio shell · amber + cyan-mint on deep teal

const ICON_PATHS = {
  // Brand / core
  send:      <path d="M4 12 L20 12 M14 6 L20 12 L14 18" />,
  paperclip: <path d="M21 11.5 12.5 20a5 5 0 0 1-7-7L14 4.5a3.5 3.5 0 0 1 5 5L10.5 18a2 2 0 0 1-3-3L15 7.5" />,
  sparkle:   <path d="M12 3 L13.4 9.4 L19.8 11 L13.4 12.6 L12 19 L10.6 12.6 L4.2 11 L10.6 9.4 Z M19 3 L19.6 5.4 L22 6 L19.6 6.6 L19 9 L18.4 6.6 L16 6 L18.4 5.4 Z" />,
  plus:      <path d="M12 5v14M5 12h14" />,
  close:     <path d="M6 6l12 12M18 6L6 18" />,
  chevDown:  <path d="M6 9l6 6 6-6" />,
  chevUp:    <path d="M18 15l-6-6-6 6" />,
  chevRight: <path d="M9 6l6 6-6 6" />,
  chevLeft:  <path d="M15 6l-6 6 6 6" />,
  folder:    <path d="M3 6.5a2 2 0 0 1 2-2h3.5l2 2H19a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />,
  folderOpen:<><path d="M3 6.5a2 2 0 0 1 2-2h3.5l2 2H19a2 2 0 0 1 2 2v1H3z"/><path d="M3 8h18l-2 8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/></>,
  grid:      <><rect x="4" y="4" width="6.5" height="6.5" rx="1"/><rect x="13.5" y="4" width="6.5" height="6.5" rx="1"/><rect x="4" y="13.5" width="6.5" height="6.5" rx="1"/><rect x="13.5" y="13.5" width="6.5" height="6.5" rx="1"/></>,
  user:      <><circle cx="12" cy="8" r="3.4"/><path d="M5 20c0-3.8 3.1-7 7-7s7 3.2 7 7"/></>,
  settings:  <><circle cx="12" cy="12" r="2.6"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3 1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8 1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z"/></>,
  download:  <path d="M12 4v12m0 0l-4-4m4 4l4-4M5 20h14"/>,
  upload:    <path d="M12 20V8m0 0l-4 4m4-4l4 4M5 4h14"/>,
  github:    <path d="M12 2a10 10 0 0 0-3.16 19.49c.5.09.68-.22.68-.48v-1.7c-2.78.6-3.37-1.34-3.37-1.34-.45-1.16-1.11-1.47-1.11-1.47-.91-.62.07-.6.07-.6 1 .07 1.53 1.03 1.53 1.03.89 1.53 2.34 1.09 2.91.83.09-.65.35-1.09.63-1.34-2.22-.25-4.55-1.11-4.55-4.94 0-1.09.39-1.98 1.03-2.68-.1-.25-.45-1.27.1-2.65 0 0 .84-.27 2.75 1.02a9.5 9.5 0 0 1 5 0c1.91-1.29 2.75-1.02 2.75-1.02.55 1.38.2 2.4.1 2.65.64.7 1.03 1.6 1.03 2.68 0 3.84-2.34 4.69-4.57 4.94.36.31.68.92.68 1.85v2.74c0 .27.18.58.69.48A10 10 0 0 0 12 2z"/>,
  google:    <path fill="currentColor" stroke="none" d="M21.35 11.1H12v3.2h5.35c-.23 1.23-1.65 3.6-5.35 3.6-3.22 0-5.85-2.66-5.85-5.93s2.63-5.93 5.85-5.93c1.83 0 3.06.78 3.76 1.45l2.56-2.47C16.78 3.7 14.62 2.8 12 2.8 6.94 2.8 2.85 6.9 2.85 12s4.09 9.2 9.15 9.2c5.28 0 8.78-3.7 8.78-8.92 0-.6-.06-1.06-.13-1.18z"/>,
  branch:    <><circle cx="6" cy="5" r="2"/><circle cx="6" cy="19" r="2"/><circle cx="18" cy="12" r="2"/><path d="M6 7v10M16 12H10a4 4 0 0 1-4-4"/></>,
  code:      <path d="M8 8l-4 4 4 4M16 8l4 4-4 4M14 4l-4 16"/>,
  eye:       <><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12z"/><circle cx="12" cy="12" r="3"/></>,
  monitor:   <><rect x="3" y="4" width="18" height="12" rx="1.5"/><path d="M8 20h8M12 16v4"/></>,
  phone:     <><rect x="7" y="2" width="10" height="20" rx="2"/><path d="M11 18h2"/></>,
  refresh:   <path d="M21 12a9 9 0 1 1-3-6.7M21 4v5h-5"/>,
  search:    <><circle cx="11" cy="11" r="6"/><path d="m20 20-3.5-3.5"/></>,
  filter:    <path d="M3 5h18l-7 9v5l-4 2v-7z"/>,
  bell:      <path d="M6 8a6 6 0 1 1 12 0c0 5 2 6 2 6H4s2-1 2-6zM10 19a2 2 0 0 0 4 0"/>,
  paperclip2:<path d="M21 11.5 12.5 20a5 5 0 0 1-7-7L14 4.5a3.5 3.5 0 0 1 5 5L10.5 18a2 2 0 0 1-3-3L15 7.5"/>,
  image:     <><rect x="3" y="4" width="18" height="16" rx="2"/><circle cx="9" cy="10" r="1.5"/><path d="m3 17 5-5 6 6 3-3 4 4"/></>,
  zap:       <path d="M13 2 4 14h7l-1 8 9-12h-7z"/>,
  check:     <path d="M5 12l5 5L20 7"/>,
  alert:     <path d="M12 3 2 21h20zM12 10v5M12 18v.5"/>,
  info:      <><circle cx="12" cy="12" r="9"/><path d="M12 8v.5M11 12h1v5h1"/></>,
  dots:      <><circle cx="6" cy="12" r="1.6"/><circle cx="12" cy="12" r="1.6"/><circle cx="18" cy="12" r="1.6"/></>,
  queue:     <path d="M3 6h18M3 12h18M3 18h12"/>,
  archive:   <><rect x="3" y="4" width="18" height="4" rx="1"/><path d="M5 8v11a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1V8M10 13h4"/></>,
  trash:     <path d="M4 7h16M9 7V5a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2M6 7l1 13a1 1 0 0 0 1 1h8a1 1 0 0 0 1-1l1-13"/>,
  edit:      <path d="M14 4l6 6L8 22H2v-6z"/>,
  globe:     <><circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3a13 13 0 0 1 0 18M12 3a13 13 0 0 0 0 18"/></>,
  language:  <><path d="M3 5h12M9 3v2M5 5c2 7 6 9 10 9M6 14c4 0 8-2 10-9"/><path d="M14 21l4-10 4 10M16 17h4"/></>,
  cpu:       <><rect x="5" y="5" width="14" height="14" rx="2"/><rect x="9" y="9" width="6" height="6"/><path d="M3 9h2M3 15h2M19 9h2M19 15h2M9 3v2M15 3v2M9 19v2M15 19v2"/></>,
  server:    <><rect x="3" y="4" width="18" height="7" rx="1.5"/><rect x="3" y="13" width="18" height="7" rx="1.5"/><circle cx="7" cy="7.5" r=".8"/><circle cx="7" cy="16.5" r=".8"/></>,
  pulse:     <path d="M3 12h4l2-6 4 12 2-6h6"/>,
  flag:      <path d="M5 21V4M5 4h12l-2 4 2 4H5"/>,
  book:      <path d="M4 5a2 2 0 0 1 2-2h12v16H6a2 2 0 0 0-2 2zM4 19a2 2 0 0 1 2-2h12"/>,
  clock:     <><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/></>,
  history:   <path d="M3 12a9 9 0 1 0 3-6.7M3 4v5h5M12 8v5l3 2"/>,
  menu:      <path d="M4 6h16M4 12h16M4 18h16"/>,
  split:     <><rect x="3" y="4" width="18" height="16" rx="1.5"/><path d="M12 4v16"/></>,
  overlay:   <><rect x="3" y="4" width="18" height="16" rx="1.5"/><rect x="7" y="12" width="13" height="6" rx="1.5"/></>,
  expand:    <path d="M4 14v6h6M20 10V4h-6M4 20l7-7M20 4l-7 7"/>,
  collapse:  <path d="M9 4v5H4M15 20v-5h5M9 9 4 4M15 15l5 5"/>,
  rocket:    <path d="M14 13.5 10.5 10M9 21l-3-3 2-4 4-4 2 2-4 4-4 2zM13 8c1-3 4-5 8-5-1 4-3 7-6 8M14 14c-3 1-5 4-5 8 4-1 7-3 8-6"/>,
  target:    <><circle cx="12" cy="12" r="9"/><circle cx="12" cy="12" r="5"/><circle cx="12" cy="12" r="1.4" fill="currentColor"/></>,
  steer:     <><circle cx="12" cy="12" r="8"/><circle cx="12" cy="12" r="2" fill="currentColor"/><path d="M12 4v4M12 16v4M4 12h4M16 12h4"/></>,
  stop:      <rect x="6" y="6" width="12" height="12" rx="2"/>,
  pause:     <><rect x="6" y="5" width="4" height="14" rx="0.5"/><rect x="14" y="5" width="4" height="14" rx="0.5"/></>,
  play:      <polygon points="6 4 19 12 6 20" />,
  layers:    <path d="M12 3l9 5-9 5-9-5 9-5zM3 13l9 5 9-5M3 17l9 5 9-5"/>,
  bolt:      <path d="M13 2 4 14h7l-1 8 9-12h-7z"/>,
  star:      <polygon points="12 3 14.5 9 21 9.5 16 14 17.5 21 12 17.5 6.5 21 8 14 3 9.5 9.5 9"/>,
  web:       <><rect x="3" y="4" width="18" height="16" rx="2"/><path d="M3 9h18M7 6.5h.1M10 6.5h.1"/></>,
  api:       <path d="M4 7h6v10H4zM14 7h6v10h-6zM10 9h4M10 15h4M10 12h4"/>,
  docs:      <><path d="M14 3H6a1 1 0 0 0-1 1v16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1V8zM14 3v5h5"/><path d="M9 13h6M9 17h6"/></>,
  worker:    <><circle cx="12" cy="12" r="3"/><circle cx="12" cy="12" r="9"/><path d="M12 3v4M12 17v4M3 12h4M17 12h4"/></>,
  cmd:       <path d="M7 7a3 3 0 1 0 3 3V7zm10 0a3 3 0 1 1-3 3V7zM7 17a3 3 0 1 0 3-3v3zm10 0a3 3 0 1 1-3-3v3zM10 7h4M10 17h4M7 10v4M17 10v4"/>,
  help:      <><circle cx="12" cy="12" r="9"/><path d="M9.5 9a2.5 2.5 0 1 1 3.4 2.3c-.5.2-.9.7-.9 1.4V13M12 16v.5"/></>,
  signout:   <path d="M16 17l5-5-5-5M21 12H9M12 19H5a2 2 0 0 1-2-2V7a2 2 0 0 1 2-2h7"/>,
  inspector: <><path d="M4 4l16 7-6 2-2 6z"/><path d="M14 13l7 7"/></>,
};

const Icon = ({ name, size = 16, stroke = 1.5, ...rest }) => {
  const p = ICON_PATHS[name];
  if (!p) return <span style={{ width: size, height: size, display: 'inline-block', background: 'rgba(255,255,255,.08)', borderRadius: 2 }} />;
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor"
         strokeWidth={stroke} strokeLinecap="round" strokeLinejoin="round" {...rest}>
      {p}
    </svg>
  );
};

// ─── Studio canvas background (dot grid + corner brackets) ────────────────
const StudioCanvas = ({ children, brackets = true, vignette = true, style }) => (
  <div className="lk-canvas lk-brackets" style={{ position: 'relative', width: '100%', height: '100%', overflow: 'hidden', ...style }}>
    {brackets && (
      <>
        <span className="br-tl" />
        <span className="br-tr" />
        <span className="br-bl" />
        <span className="br-br" />
      </>
    )}
    {children}
  </div>
);

// ─── Status chip with named state ─────────────────────────────────────────
const STATUS_MAP = {
  ready:    { cls: 'is-ok',     label: 'Ready',   dot: true },
  live:     { cls: 'is-ok',     label: 'Live',    dot: true },
  starting: { cls: 'is-warn is-loading', label: 'Starting',  dot: true },
  building: { cls: 'is-amber is-loading', label: 'Building', dot: true },
  queued:   { cls: 'is-info',   label: 'Queued',  dot: true },
  paused:   { cls: 'is-warn',   label: 'Paused',  dot: true },
  stopped:  { cls: '',          label: 'Stopped', dot: false },
  archived: { cls: 'is-cyan',   label: 'Archived · export-only', dot: false },
  error:    { cls: 'is-danger', label: 'Needs attention', dot: true },
};

const StatusChip = ({ state = 'ready', children, label }) => {
  const c = STATUS_MAP[state] || STATUS_MAP.ready;
  return (
    <span className={`lk-chip ${c.cls}`}>
      {c.dot && <span className="lk-dot" />}
      {children || label || c.label}
    </span>
  );
};

// ─── Build run timeline ───────────────────────────────────────────────────
const TimelineStep = ({ state, label, detail, time, last }) => {
  const color = {
    done:    { bg: 'var(--ok)',    bord: 'var(--ok)',    line: 'var(--ok)' },
    running: { bg: 'var(--amber)', bord: 'var(--amber)', line: 'var(--border-default)' },
    queued:  { bg: 'transparent',  bord: 'var(--text-tertiary)', line: 'var(--border-hairline)' },
    fail:    { bg: 'var(--danger)',bord: 'var(--danger)',line: 'var(--border-default)' },
  }[state] || { bg: 'transparent', bord: 'var(--text-tertiary)', line: 'var(--border-hairline)' };
  return (
    <div style={{ display: 'flex', gap: 12, position: 'relative', paddingBottom: last ? 0 : 12 }}>
      <div style={{ position: 'relative', flex: '0 0 auto', width: 14 }}>
        <div style={{
          width: 12, height: 12, borderRadius: '50%',
          background: color.bg, border: `1.5px solid ${color.bord}`,
          marginTop: 3, position: 'relative', zIndex: 1,
        }}>
          {state === 'running' && (
            <span style={{
              position: 'absolute', inset: -4,
              border: '1px solid var(--amber)', borderRadius: '50%',
              opacity: 0.45, animation: 'lk-pulse 1.4s ease-in-out infinite',
            }} />
          )}
          {state === 'done' && (
            <svg width="10" height="10" viewBox="0 0 24 24" style={{ position: 'absolute', top: 0, left: 0 }}
                 fill="none" stroke="var(--amber-fg)" strokeWidth="3.4" strokeLinecap="round" strokeLinejoin="round">
              <path d="M5 12l5 5L20 7" />
            </svg>
          )}
        </div>
        {!last && (
          <div style={{ position: 'absolute', top: 18, left: 6, bottom: -12, width: 1, background: color.line }} />
        )}
      </div>
      <div style={{ flex: 1, minWidth: 0, paddingTop: 1 }}>
        <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, justifyContent: 'space-between' }}>
          <span style={{
            fontSize: 12.5, fontWeight: 500,
            color: state === 'queued' ? 'var(--text-tertiary)' : 'var(--text-primary)',
            fontFamily: state === 'running' ? 'var(--font-mono)' : 'inherit',
          }}>{label}</span>
          {time && <span className="lk-mono lk-num" style={{ fontSize: 10.5, color: 'var(--text-tertiary)' }}>{time}</span>}
        </div>
        {detail && (
          <div style={{ fontSize: 11.5, color: 'var(--text-secondary)', marginTop: 2,
                        fontFamily: state === 'running' ? 'var(--font-mono)' : 'inherit' }}>
            {detail}
          </div>
        )}
      </div>
    </div>
  );
};

// ─── System event row (replaces leaked [[LIKEABLE_NOTIFICATION_START]] etc) ─
const SystemEvent = ({ kind = 'ok', icon = 'check', label, sub, time, action }) => {
  const palette = {
    ok:     { bord: 'rgba(94,212,168,0.22)',  bg: 'var(--ok-bg)',     fg: 'var(--ok)' },
    info:   { bord: 'rgba(126,200,255,0.22)', bg: 'var(--info-bg)',   fg: 'var(--info)' },
    amber:  { bord: 'var(--border-subtle)',   bg: 'var(--amber-bg-08)', fg: 'var(--amber)' },
    danger: { bord: 'rgba(255,122,122,0.22)', bg: 'var(--danger-bg)', fg: 'var(--danger)' },
  }[kind];
  return (
    <div style={{
      display: 'flex', alignItems: 'center', gap: 10,
      padding: '8px 10px',
      background: palette.bg,
      border: `1px solid ${palette.bord}`,
      borderRadius: 'var(--r-md)',
    }}>
      <div style={{
        width: 20, height: 20, borderRadius: 5,
        background: 'rgba(0,0,0,0.25)',
        color: palette.fg,
        display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
        flex: '0 0 auto',
      }}>
        <Icon name={icon} size={11} stroke={2.2} />
      </div>
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ fontSize: 12, fontWeight: 500, color: 'var(--text-primary)' }}>{label}</div>
        {sub && <div className="lk-mono" style={{ fontSize: 10.5, color: 'var(--text-tertiary)', marginTop: 1 }}>{sub}</div>}
      </div>
      {action}
      {time && <span className="lk-mono lk-num" style={{ fontSize: 10.5, color: 'var(--text-tertiary)' }}>{time}</span>}
    </div>
  );
};

// ─── Service badge ────────────────────────────────────────────────────────
const ServiceBadge = ({ kind = 'web', name, port, status = 'live', active = false, url, compact = false }) => (
  <div style={{
    display: 'flex', alignItems: 'center', gap: 10,
    padding: compact ? '6px 10px' : '8px 10px',
    borderRadius: 'var(--r-md)',
    background: active ? 'var(--amber-bg-08)' : 'var(--bg-overlay)',
    border: `1px solid ${active ? 'var(--border-default)' : 'var(--border-subtle)'}`,
    cursor: 'pointer',
  }}>
    <div style={{
      width: 26, height: 26, borderRadius: 6,
      background: 'rgba(0,0,0,0.25)',
      color: active ? 'var(--amber-bright)' : 'var(--text-secondary)',
      display: 'flex', alignItems: 'center', justifyContent: 'center',
      flex: '0 0 auto',
    }}>
      <Icon name={kind} size={13} />
    </div>
    <div style={{ flex: 1, minWidth: 0 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
        <span style={{ fontSize: 12.5, fontWeight: 500, color: active ? 'var(--text-primary)' : 'var(--text-secondary)' }}>{name}</span>
        <span className="lk-mono" style={{ fontSize: 9.5, color: 'var(--text-tertiary)', textTransform: 'uppercase', letterSpacing: '.08em' }}>{kind}</span>
      </div>
      {!compact && (
        <div className="lk-mono" style={{ fontSize: 10.5, color: 'var(--text-tertiary)', marginTop: 1 }}>
          {url || (port ? `localhost:${port}` : '—')}
        </div>
      )}
    </div>
    <span className={`lk-dot-st ${status === 'live' ? 'is-ok' : status === 'starting' ? 'is-warn' : 'is-idle'}`} />
  </div>
);

// ─── Hour pill (build-hour quota badge) ───────────────────────────────────
const HourPill = ({ used = '4h 26m', total = '5h', tone = 'default' }) => {
  const c = tone === 'warn' ? 'var(--warn)' : tone === 'danger' ? 'var(--danger)' : 'var(--amber)';
  return (
    <span className="lk-mono lk-num" style={{
      display: 'inline-flex', alignItems: 'center', gap: 6,
      height: 24, padding: '0 10px',
      borderRadius: 999,
      background: 'rgba(0,0,0,0.32)',
      border: '1px solid var(--border-subtle)',
      color: c, fontSize: 11,
    }}>
      <Icon name="clock" size={10} stroke={1.6} />
      {used} <span style={{ color: 'var(--text-tertiary)' }}>/ {total}</span>
    </span>
  );
};

// ─── Quota bar ────────────────────────────────────────────────────────────
const QuotaBar = ({ used, total, label, sub, accent = 'var(--amber)' }) => {
  const pct = Math.min(100, (used / total) * 100);
  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', marginBottom: 6 }}>
        <span style={{ fontSize: 11.5, color: 'var(--text-secondary)' }}>{label}</span>
        <span className="lk-mono lk-num" style={{ fontSize: 11.5, color: 'var(--text-primary)' }}>
          <strong style={{ fontWeight: 600 }}>{used}</strong>
          <span style={{ color: 'var(--text-tertiary)' }}> / {total}</span>
        </span>
      </div>
      <div style={{ height: 6, borderRadius: 999, background: 'var(--bg-inset)', overflow: 'hidden', border: '1px solid var(--border-hairline)' }}>
        <div style={{ height: '100%', width: `${pct}%`, background: accent, borderRadius: 999 }} />
      </div>
      {sub && <div className="lk-mono" style={{ fontSize: 10.5, color: 'var(--text-tertiary)', marginTop: 5 }}>{sub}</div>}
    </div>
  );
};

// ─── Browser-like preview frame ────────────────────────────────────────────
const PreviewFrame = ({ children, url = 'app.likeable.studio/app', actions, label = 'app' }) => (
  <div style={{
    flex: 1, display: 'flex', flexDirection: 'column',
    background: '#ffffff',
    borderRadius: 'var(--r-lg)',
    border: '1px solid var(--border-subtle)',
    overflow: 'hidden',
    minHeight: 0,
    position: 'relative',
  }}>
    <div className="lk-browser-bar">
      <div className="lk-traffic">
        <span style={{ background: '#ff5f57' }} />
        <span style={{ background: '#febc2e' }} />
        <span style={{ background: '#28c840' }} />
      </div>
      <span className="lk-chip is-amber" style={{ height: 20 }}>
        <Icon name="web" size={10} stroke={1.4} /> {label}
      </span>
      <div style={{
        flex: 1, height: 24,
        background: 'rgba(0,0,0,0.3)',
        border: '1px solid var(--border-hairline)',
        borderRadius: 'var(--r-sm)',
        display: 'flex', alignItems: 'center', gap: 8, padding: '0 10px',
      }}>
        <Icon name="globe" size={11} stroke={1.4} style={{ color: 'var(--text-tertiary)' }} />
        <span className="lk-mono" style={{ fontSize: 10.5, color: 'var(--text-secondary)' }}>{url}</span>
      </div>
      {actions}
      <button className="lk-iconbtn is-sm"><Icon name="refresh" size={13} /></button>
      <button className="lk-iconbtn is-sm"><Icon name="expand" size={13} /></button>
    </div>
    <div style={{ flex: 1, position: 'relative', minHeight: 0, overflow: 'hidden' }}>
      {children}
    </div>
  </div>
);

// ─── Faux Blood Analysis LIS preview (matches the screenshot) ──────────────
const BloodAnalysisLIS = () => (
  <div className="lk-app is-light" style={{ width: '100%', height: '100%', overflow: 'hidden', padding: '20px 28px', fontFamily: 'var(--font-sans)' }}>
    {/* Top bar */}
    <div style={{ display: 'flex', alignItems: 'center', gap: 16, paddingBottom: 18, borderBottom: '1px solid #ececec' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <span style={{
          width: 20, height: 20, borderRadius: 5,
          background: '#fff', border: '1px solid #ececec',
          display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
        }}>
          <svg width="11" height="11" viewBox="0 0 24 24" fill="#e0303c">
            <path d="M12 2 C 8 8, 5 12, 5 16 a 7 7 0 0 0 14 0 c 0 -4 -3 -8 -7 -14 Z" />
          </svg>
        </span>
        <span style={{ fontSize: 15, fontWeight: 600 }}>Blood Analysis LIS</span>
      </div>
      <div style={{ flex: 1, display: 'flex', justifyContent: 'center' }}>
        <div style={{
          width: 340, height: 32, borderRadius: 8,
          border: '1px solid #ececec', background: '#fff',
          display: 'flex', alignItems: 'center', padding: '0 12px', gap: 10,
        }}>
          <span style={{ fontSize: 12.5 }}>P-001 — Elena Marchetti</span>
          <span style={{ flex: 1 }} />
          <span style={{ fontSize: 11, color: '#888' }}>▼</span>
        </div>
      </div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, fontSize: 11.5, color: '#5a626a' }}>
        <span>◐</span>
        <span>Demo Admin</span>
        <span style={{
          padding: '4px 10px', borderRadius: 6, background: '#0e1216', color: '#fff', fontWeight: 500,
        }}>Sign out</span>
      </div>
    </div>

    {/* Patient row */}
    <div style={{ display: 'flex', alignItems: 'center', gap: 14, paddingTop: 16, paddingBottom: 16 }}>
      <div style={{ width: 44, height: 44, borderRadius: '50%', background: '#e8f0f4', color: '#2c4a5a', fontWeight: 600, display: 'inline-flex', alignItems: 'center', justifyContent: 'center', fontSize: 14 }}>EM</div>
      <div style={{ flex: 1 }}>
        <div style={{ fontSize: 17, fontWeight: 600 }}>Elena Marchetti</div>
        <div style={{ fontSize: 11.5, color: '#6a727a' }}>P-001 · DOB 1982-03-15 · Age 44 · Sex F · Dr. K. Nowak</div>
      </div>
      <div style={{ display: 'flex', gap: 28, fontSize: 10.5, color: '#8a929a', textTransform: 'uppercase', letterSpacing: '.08em' }}>
        <div>
          <div>Sample ID</div>
          <div style={{ color: '#1a1d22', fontFamily: 'var(--font-mono)', fontSize: 12, marginTop: 2, fontWeight: 500 }}>LAB-2026-7291</div>
        </div>
        <div>
          <div>Collected</div>
          <div style={{ color: '#1a1d22', fontFamily: 'var(--font-mono)', fontSize: 12, marginTop: 2, fontWeight: 500 }}>2026-05-19 08:14</div>
        </div>
        <div>
          <div>Status</div>
          <div style={{ display: 'inline-flex', padding: '3px 8px', borderRadius: 999, background: '#e6f7ee', color: '#0d8a4d', fontSize: 11, marginTop: 2, fontWeight: 500 }}>Final</div>
        </div>
      </div>
    </div>

    {/* KPI cards */}
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 10, marginBottom: 14 }}>
      {[
        { v: '17', l: 'Total Tests', i: '📈', bg: '#f0f4f9', c: '#1f5ea8' },
        { v: '10', l: 'Normal', i: '✓', bg: '#e7f4ec', c: '#0d8a4d' },
        { v: '7',  l: 'Abnormal', i: '!', bg: '#fff4dc', c: '#a86b00' },
        { v: '0',  l: 'Critical', i: '♥', bg: '#fce8e8', c: '#a82424' },
      ].map(k => (
        <div key={k.l} style={{ background: '#fff', border: '1px solid #ececec', borderRadius: 10, padding: '12px 14px', display: 'flex', alignItems: 'center', gap: 12 }}>
          <span style={{ width: 30, height: 30, borderRadius: 8, background: k.bg, color: k.c, display: 'inline-flex', alignItems: 'center', justifyContent: 'center', fontSize: 13, fontWeight: 600 }}>{k.i}</span>
          <div>
            <div style={{ fontSize: 20, fontWeight: 600, letterSpacing: '-0.01em' }}>{k.v}</div>
            <div style={{ fontSize: 11, color: '#6a727a' }}>{k.l}</div>
          </div>
        </div>
      ))}
    </div>

    {/* Three panels */}
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 10 }}>
      {[
        { name: 'Complete Blood Count', flagged: 4, rows: [
          { t: 'WBC', sub: 'White Blood Cells', v: '7.2', u: '10³/µL', rng: '4.5 – 11',  flag: 'Normal' },
          { t: 'RBC', sub: 'Red Blood Cells',   v: '3.91', u: '10⁶/µL', rng: '4.2 – 5.4', flag: 'Low' },
          { t: 'HGB', sub: 'Hemoglobin',         v: '11.2', u: 'g/dL',   rng: '12 – 16',   flag: 'Low' },
        ]},
        { name: 'Metabolic Panel', flagged: 1, rows: [
          { t: 'GLU', sub: 'Glucose', v: '112', u: 'mg/dL', rng: '70 – 100', flag: 'High' },
          { t: 'BUN', sub: 'Blood Urea Nitrogen', v: '18', u: 'mg/dL', rng: '7 – 20', flag: 'Normal' },
          { t: 'CREA', sub: 'Creatinine', v: '0.82', u: 'mg/dL', rng: '0.6 – 1.1', flag: 'Normal' },
        ]},
        { name: 'Lipid Panel', flagged: 2, rows: [
          { t: 'TC',  sub: 'Total Cholesterol', v: '218', u: 'mg/dL', rng: '< 200', flag: 'High' },
          { t: 'HDL', sub: 'HDL Cholesterol', v: '52', u: 'mg/dL', rng: '50 – 999', flag: 'Normal' },
          { t: 'LDL', sub: 'LDL Cholesterol', v: '138', u: 'mg/dL', rng: '< 100', flag: 'High' },
        ]},
      ].map(panel => (
        <div key={panel.name} style={{ background: '#fff', border: '1px solid #ececec', borderRadius: 10, overflow: 'hidden' }}>
          <div style={{ display: 'flex', alignItems: 'center', padding: '10px 14px', borderBottom: '1px solid #ececec' }}>
            <span style={{ fontSize: 12.5, fontWeight: 600 }}>{panel.name}</span>
            <span style={{ flex: 1 }} />
            <span style={{ fontSize: 10, padding: '2px 7px', borderRadius: 999, background: '#fff4dc', color: '#a86b00', fontWeight: 500 }}>{panel.flagged} flagged</span>
          </div>
          <table style={{ width: '100%', fontSize: 11, borderCollapse: 'collapse' }}>
            <thead><tr style={{ color: '#8a929a', fontSize: 9.5, textTransform: 'uppercase', letterSpacing: '.06em', textAlign: 'left' }}>
              <th style={{ padding: '6px 14px', fontWeight: 500 }}>Test</th>
              <th style={{ padding: '6px 4px', fontWeight: 500, textAlign: 'right' }}>Result</th>
              <th style={{ padding: '6px 4px', fontWeight: 500, textAlign: 'right' }}>Range</th>
              <th style={{ padding: '6px 14px', fontWeight: 500, textAlign: 'right' }}>Flag</th>
            </tr></thead>
            <tbody>
              {panel.rows.map(r => (
                <tr key={r.t} style={{ borderTop: '1px solid #f3f3f3' }}>
                  <td style={{ padding: '7px 14px' }}>
                    <div style={{ fontWeight: 500 }}>{r.t}</div>
                    <div style={{ fontSize: 10, color: '#8a929a' }}>{r.sub}</div>
                  </td>
                  <td style={{ padding: '7px 4px', textAlign: 'right' }}>
                    <span style={{ fontWeight: 600 }}>{r.v}</span> <span style={{ color: '#8a929a' }}>{r.u}</span>
                  </td>
                  <td style={{ padding: '7px 4px', textAlign: 'right', color: '#6a727a', fontFamily: 'var(--font-mono)', fontSize: 10.5 }}>{r.rng}</td>
                  <td style={{ padding: '7px 14px', textAlign: 'right' }}>
                    <span style={{
                      fontSize: 10, padding: '2px 8px', borderRadius: 999,
                      background: r.flag === 'High' || r.flag === 'Low' ? '#fff4dc' : '#e7f4ec',
                      color: r.flag === 'High' || r.flag === 'Low' ? '#a86b00' : '#0d8a4d',
                      fontWeight: 500,
                    }}>{r.flag}</span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ))}
    </div>
  </div>
);

// ─── Window chrome / FIBE tab ─────────────────────────────────────────────
const FibeTab = ({ children = 'Powered by Fibe.gg' }) => (
  <div style={{ display: 'flex', justifyContent: 'center', position: 'relative', zIndex: 2 }}>
    <span className="lk-fibe-tab">
      Powered by <span className="lk-amber-strong">FIBE.GG</span>
    </span>
  </div>
);

Object.assign(window, {
  Icon, StudioCanvas, StatusChip, TimelineStep, SystemEvent, ServiceBadge,
  HourPill, QuotaBar, PreviewFrame, FibeTab, BloodAnalysisLIS,
});
