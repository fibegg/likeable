import { useEffect, useMemo, useRef, useState } from 'react';
import { Loader2, Plus, Send, Trash2 } from 'lucide-react';
import { cleanPoolRows, makePoolRow, parsePoolRows } from './admin_pool';
import { api } from './api';
import { AppDialog, Metric } from './builder_components';
import { ADMIN_CONFIG_SECTIONS, adminConfigLabel } from './config';
import type { AdminConfigEntry, AdminConfigResponse, AdminUserDetail, AdminUserSummary, AdminUsersResponse, AppDialogConfig, PoolRow } from './domain';
import { formatMessageTime, formatShortDate } from './format';

function AdminCustomersPanel() {
  const [filters, setFilters] = useState({ q: '', status: '', github: '', billing: '', sort: 'created_desc', perPage: '25' });
  const [page, setPage] = useState(1);
  const [users, setUsers] = useState<AdminUserSummary[]>([]);
  const [total, setTotal] = useState(0);
  const [selectedUserID, setSelectedUserID] = useState('');
  const [detail, setDetail] = useState<AdminUserDetail | null>(null);
  const [loading, setLoading] = useState(false);
  const [actionError, setActionError] = useState('');
  const [accessNote, setAccessNote] = useState('');
  const [noticeBody, setNoticeBody] = useState('');
  const [noticeSeverity, setNoticeSeverity] = useState('warning');
  const [dialog, setDialog] = useState<AppDialogConfig | null>(null);
  const noticeListRef = useRef<HTMLDivElement | null>(null);

  const query = () => {
    const params = new URLSearchParams();
    params.set('page', String(page));
    params.set('per_page', filters.perPage);
    for (const key of ['q', 'status', 'github', 'billing', 'sort'] as const) {
      if (filters[key]) params.set(key, filters[key]);
    }
    return params.toString();
  };
  const loadUsers = async () => {
    setLoading(true);
    setActionError('');
    try {
      const response = await api<AdminUsersResponse>(`/api/admin/users?${query()}`);
      setUsers(response.users);
      setTotal(response.pagination.total);
      if (selectedUserID && !response.users.some((summary) => summary.user.id === selectedUserID)) {
        setSelectedUserID('');
        setDetail(null);
      }
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Could not load users');
    } finally {
      setLoading(false);
    }
  };
  const loadDetail = async (userID: string) => {
    setActionError('');
    const response = await api<AdminUserDetail>(`/api/admin/users/${userID}`);
    setSelectedUserID(userID);
    setDetail(response);
    setAccessNote(response.summary.user.accessNote ?? '');
  };
  useEffect(() => {
    const timer = setTimeout(() => { void loadUsers(); }, 220);
    return () => clearTimeout(timer);
  }, [filters, page]);
  const selectedSummary = detail?.summary;
  const orderedNotices = useMemo(() => [...(detail?.notices ?? [])].sort((left, right) => {
    const leftTime = Date.parse(left.createdAt);
    const rightTime = Date.parse(right.createdAt);
    if (Number.isNaN(leftTime) || Number.isNaN(rightTime)) return 0;
    return leftTime - rightTime;
  }), [detail?.notices]);
  const totalPages = Math.max(1, Math.ceil(total / Number(filters.perPage || 25)));

  useEffect(() => {
    const container = noticeListRef.current;
    if (!container) return;
    container.scrollTop = container.scrollHeight;
  }, [selectedUserID, orderedNotices.length]);

  const applyAccess = async (status: 'active' | 'restricted') => {
    if (!selectedUserID) return;
    setActionError('');
    try {
      await api(`/api/admin/users/${selectedUserID}`, {
        method: 'PATCH',
        body: JSON.stringify({ accessStatus: status, accessNote })
      });
      await loadDetail(selectedUserID);
      await loadUsers();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Access update failed');
    }
  };
  const patchAccess = async (status: 'active' | 'restricted') => {
    if (!selectedUserID) return;
    if (status === 'restricted') {
      setDialog({
        title: 'Restrict user?',
        body: 'The user will keep seeing system notices, but app actions will be blocked until access is restored.',
        tone: 'danger',
        confirmLabel: 'Restrict',
        onConfirm: () => applyAccess('restricted')
      });
      return;
    }
    await applyAccess(status);
  };
  const sendNotice = async () => {
    if (!selectedUserID || !noticeBody.trim()) return;
    setActionError('');
    try {
      await api(`/api/admin/users/${selectedUserID}/notices`, {
        method: 'POST',
        body: JSON.stringify({ severity: noticeSeverity, body: noticeBody.trim() })
      });
      setNoticeBody('');
      await loadDetail(selectedUserID);
      await loadUsers();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Notice failed');
    }
  };
  const handleNoticeKeyDown = (event: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key !== 'Enter' || event.shiftKey || event.nativeEvent.isComposing) return;
    event.preventDefault();
    if (noticeBody.trim()) void sendNotice();
  };
  const applyUnsendNotice = async (noticeID: string) => {
    if (!selectedUserID) return;
    setActionError('');
    try {
      await api(`/api/admin/users/${selectedUserID}/notices/${noticeID}`, { method: 'DELETE' });
      await loadDetail(selectedUserID);
      await loadUsers();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Unsend failed');
    }
  };
  const unsendNotice = async (noticeID: string) => {
    if (!selectedUserID) return;
    setDialog({
      title: 'Unsend system message?',
      body: 'It will disappear from the user mailbox and active banners.',
      tone: 'warning',
      confirmLabel: 'Unsend',
      onConfirm: () => applyUnsendNotice(noticeID)
    });
  };
  const applyDeleteProject = async (projectID: string) => {
    if (!selectedUserID) return;
    setActionError('');
    try {
      await api(`/api/admin/users/${selectedUserID}/projects/${projectID}`, { method: 'DELETE' });
      await loadDetail(selectedUserID);
      await loadUsers();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Project delete failed');
    }
  };
  const deleteProject = async (projectID: string) => {
    if (!selectedUserID) return;
    setDialog({
      title: 'Delete user project?',
      body: 'This removes the project from Likeable and runs the workspace cleanup path.',
      tone: 'danger',
      confirmLabel: 'Delete',
      onConfirm: () => applyDeleteProject(projectID)
    });
  };

  return (
    <section className="adminCard customersCard">
      {dialog && <AppDialog dialog={dialog} onClose={() => setDialog(null)} />}
      <div className="adminCardHeader">
        <h3>Customers</h3>
        <p>Search, filter, inspect usage, moderate access, message users, and clean up projects.</p>
      </div>
      <div className="customerFilters">
        <label className="configLabel">
          <span>Search</span>
          <input value={filters.q} placeholder="email, name, or user id" onChange={(event) => { setPage(1); setFilters({ ...filters, q: event.target.value }); }} />
        </label>
        <label className="configLabel">
          <span>Access</span>
          <select className="adminSelect" value={filters.status} onChange={(event) => { setPage(1); setFilters({ ...filters, status: event.target.value }); }}>
            <option value="">all</option>
            <option value="active">active</option>
            <option value="restricted">restricted</option>
          </select>
        </label>
        <label className="configLabel">
          <span>GitHub</span>
          <select className="adminSelect" value={filters.github} onChange={(event) => { setPage(1); setFilters({ ...filters, github: event.target.value }); }}>
            <option value="">all</option>
            <option value="connected">connected</option>
            <option value="missing">missing</option>
          </select>
        </label>
        <label className="configLabel">
          <span>Billing</span>
          <select className="adminSelect" value={filters.billing} onChange={(event) => { setPage(1); setFilters({ ...filters, billing: event.target.value }); }}>
            <option value="">all</option>
            <option value="paid">paid</option>
            <option value="unpaid">unpaid</option>
            <option value="subscribed">legacy subscription</option>
          </select>
        </label>
        <label className="configLabel">
          <span>Sort</span>
          <select className="adminSelect" value={filters.sort} onChange={(event) => setFilters({ ...filters, sort: event.target.value })}>
            <option value="created_desc">newest</option>
            <option value="messages_desc">messages</option>
            <option value="paid_desc">paid</option>
            <option value="projects_desc">projects</option>
            <option value="email_asc">email</option>
          </select>
        </label>
      </div>
      {actionError && <div className="adminError inlineAdminError">{actionError}</div>}
      <div className="customersLayout">
        <div className="customerList" aria-busy={loading}>
          {users.map((summary) => (
            <button className={`customerRow ${selectedUserID === summary.user.id ? 'selected' : ''} ${summary.user.accessStatus === 'restricted' ? 'restricted' : ''}`} key={summary.user.id} onClick={() => void loadDetail(summary.user.id)}>
              <span>
                <strong>{summary.user.email}</strong>
                <em>{summary.user.name || summary.user.id}</em>
              </span>
              <span className="customerStats">
                <b>{summary.dailyMessageCount}/{summary.freeMessageLimit}</b>
                <small>{summary.messageCount} lifetime · {summary.projectCount}/{summary.projectLimit ?? summary.projectCount} projects</small>
              </span>
              <span className="customerBadges">
                <i>{summary.githubConnected ? 'GitHub' : 'No GitHub'}</i>
                <i>{summary.paidCreditBalance > 0 ? `${summary.paidCreditBalance} credits` : (summary.paidTotalCents > 0 ? 'paid' : 'unpaid')}</i>
                {summary.user.accessStatus === 'restricted' && <i className="dangerBadge">restricted</i>}
              </span>
            </button>
          ))}
          {users.length === 0 && <div className="emptyPool">{loading ? 'Loading users...' : 'No users match these filters.'}</div>}
          <div className="paginationRow">
            <button className="ghostButton" disabled={page <= 1} onClick={() => setPage((value) => Math.max(1, value - 1))}>Previous</button>
            <span>{page}/{totalPages} · {total} users</span>
            <button className="ghostButton" disabled={page >= totalPages} onClick={() => setPage((value) => Math.min(totalPages, value + 1))}>Next</button>
          </div>
        </div>
        <div className="customerDetail">
          {!selectedSummary ? (
            <div className="emptyPool">Select a user to inspect projects, payments, notices, and access controls.</div>
          ) : (
            <>
              <div className="customerDetailHeader">
                <div>
                  <span className="eyebrow">Customer</span>
                  <strong>{selectedSummary.user.email}</strong>
                  <em>{selectedSummary.user.id}</em>
                </div>
                <span className={`accessBadge ${selectedSummary.user.accessStatus === 'restricted' ? 'restricted' : ''}`}>{selectedSummary.user.accessStatus || 'active'}</span>
              </div>
              <div className="customerMetricGrid">
                <Metric label="Free Today" value={`${selectedSummary.dailyMessageCount}/${selectedSummary.freeMessageLimit}`} />
                <Metric label="Lifetime Sent" value={String(selectedSummary.messageCount)} />
                <Metric label="Paid Credits" value={String(selectedSummary.paidCreditBalance)} />
                <Metric label="Projects" value={`${selectedSummary.projectCount}/${selectedSummary.projectLimit ?? selectedSummary.projectCount}`} />
                <Metric label="Paid Slots" value={selectedSummary.paidProjectSlots ? `${selectedSummary.paidProjectSlots}${selectedSummary.projectSlotsExpire ? ` until ${formatShortDate(selectedSummary.projectSlotsExpire)}` : ''}` : '0'} />
                <Metric label="GitHub" value={selectedSummary.githubConnected ? 'connected' : 'missing'} />
                <Metric label="Paid" value={formatMoney(selectedSummary.paidTotalCents, selectedSummary.paidCurrency)} />
              </div>
              <div className="moderationBox">
                <label className="configLabel">
                  <span>Access note</span>
                  <textarea className="adminTextarea compactTextarea" rows={3} value={accessNote} onChange={(event) => setAccessNote(event.target.value)} placeholder="Internal reason for restriction or follow-up" />
                </label>
                <div className="dialogActions compactActions">
                  <button className="ghostButton" onClick={() => void patchAccess('active')}>Restore access</button>
                  <button className="dangerButton" onClick={() => void patchAccess('restricted')}>Restrict access</button>
                </div>
              </div>
              <div className="adminMessageConsole">
                <div className="adminCardHeader compactHeader">
                  <h3>Messages</h3>
                  <p>User support messages and system notices sent from admin stay in one timeline.</p>
                </div>
                <div className="adminNoticeList" ref={noticeListRef}>
                  {orderedNotices.map((notice) => (
                    <div className={`adminNoticeBubble ${notice.sender === 'user' ? 'incoming' : 'outgoing'} ${notice.severity} ${notice.unsentAt ? 'unsent' : ''}`} key={notice.id}>
                      <div className="adminNoticeBubbleMeta">
                        <span>{notice.sender === 'user' ? 'User' : notice.sender === 'system' ? 'System' : 'Admin'}</span>
                        <time dateTime={notice.createdAt}>{formatMessageTime(notice.createdAt)}</time>
                        {notice.sender !== 'user' && !notice.unsentAt && (
                          <button className="unsendNoticeButton" onClick={() => void unsendNotice(notice.id)}>Unsend</button>
                        )}
                      </div>
                      <p>{notice.body}</p>
                      {notice.dismissedAt && <em>Dismissed by user</em>}
                      {notice.unsentAt && <em>Unsent</em>}
                    </div>
                  ))}
                  {orderedNotices.length === 0 && <div className="adminNoticeEmpty">No messages yet.</div>}
                </div>
                <div className="adminNoticeComposer">
                  <label className="noticeSeverityControl">
                    <span>Severity</span>
                    <select className="adminSelect" value={noticeSeverity} onChange={(event) => setNoticeSeverity(event.target.value)}>
                      <option value="info">info</option>
                      <option value="warning">warning</option>
                      <option value="danger">danger</option>
                    </select>
                  </label>
                  <textarea
                    className="adminTextarea compactTextarea"
                    rows={1}
                    value={noticeBody}
                    onChange={(event) => setNoticeBody(event.target.value)}
                    onKeyDown={handleNoticeKeyDown}
                    placeholder="Write a system message to this user..."
                  />
                  <button className="primaryButton noticeSendButton" disabled={!noticeBody.trim()} onClick={() => void sendNotice()}><Send size={16} /> Send</button>
                </div>
              </div>
              <div className="adminProjects">
                <div className="adminCardHeader compactHeader">
                  <h3>Projects</h3>
                  <p>Deletion uses the same workspace cleanup path as user-initiated deletion.</p>
                </div>
                {detail.projects.map((item) => (
                  <div className="adminProjectRow" key={item.project.id}>
                    <span>
                      <strong>{item.project.title}</strong>
                      <em>{item.project.status} · {item.messageCount} messages</em>
                    </span>
                    {item.project.previewUrl && <a className="ghostButton" href={item.project.previewUrl} target="_blank">Open</a>}
                    <button className="projectDelete" onClick={() => void deleteProject(item.project.id)} aria-label={`Delete ${item.project.title}`}><Trash2 size={15} /></button>
                  </div>
                ))}
                {detail.projects.length === 0 && <div className="emptyPool">No active projects.</div>}
              </div>
            </>
          )}
        </div>
      </div>
    </section>
  );
}

function formatMoney(cents: number, currency: string): string {
  if (!cents) return '0';
  const normalized = (currency || 'usd').toUpperCase();
  return `${normalized} ${(cents / 100).toFixed(2)}`;
}

export function Admin() {
  const [config, setConfig] = useState<Record<string, AdminConfigEntry>>({});
  const [draft, setDraft] = useState<Record<string, string>>({});
  const [signupMode, setSignupMode] = useState('forbidden');
  const [allowedEmails, setAllowedEmails] = useState('');
  const [poolRows, setPoolRows] = useState<PoolRow[]>([]);
  const [saving, setSaving] = useState(false);
  const [status, setStatus] = useState('');
  const [error, setError] = useState('');

  const loadConfig = async () => {
    const response = await api<AdminConfigResponse>('/api/admin/config');
    setConfig(response.config);
    setDraft({});
    setSignupMode(response.config.signup_mode?.value ?? 'forbidden');
    setAllowedEmails(response.config.signup_allowed_emails?.value ?? '');
    setPoolRows(parsePoolRows(response.config.fibe_agent_server_pool?.value ?? '[]'));
  };

  useEffect(() => { void loadConfig(); }, []);

  const setPoolRow = (id: string, patch: Partial<PoolRow>) => {
    setPoolRows((rows) => rows.map((row) => row.id === id ? { ...row, ...patch } : row));
  };

  const save = async () => {
    setSaving(true);
    setStatus('');
    setError('');
    try {
      const pool = cleanPoolRows(poolRows);
      const incomplete = pool.find((row) => !row.agent_id || !row.server_id);
      if (incomplete) {
        throw new Error('Every pool row needs both an agent ID and a server ID.');
      }
      await api('/api/admin/config', {
        method: 'PUT',
        body: JSON.stringify({
          ...draft,
          signup_mode: signupMode,
          signup_allowed_emails: allowedEmails,
          fibe_agent_server_pool: JSON.stringify(pool)
        })
      });
      await loadConfig();
      setStatus('Saved');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed');
    } finally {
      setSaving(false);
    }
  };

  const renderConfigFields = (keys: string[]) => {
    const entries = keys.map((key) => [key, config[key] ?? { value: '', secret: false, set: false }] as const);
    if (entries.length === 0) return <div className="emptyPool">No settings exposed for this section.</div>;
    return (
      <div className="configGrid">
        {entries.map(([key, meta]) => (
          <label key={key} className="configLabel">
            <span className="configLabelText">
              <strong>{adminConfigLabel(key)}</strong>
              {adminConfigLabel(key) !== key && <em>{key}</em>}
            </span>
            <input
              type={meta.secret ? 'password' : 'text'}
              placeholder={meta.secret && meta.set ? 'set' : meta.value}
              value={draft[key] ?? (meta.secret ? '' : meta.value)}
              onChange={(event) => setDraft({ ...draft, [key]: event.target.value })}
            />
          </label>
        ))}
      </div>
    );
  };

  return (
    <section className="panel adminPanel">
      <div className="panelTitleRow">
        <h2>Admin</h2>
        {status && <span className="adminStatus">{status}</span>}
        {error && <span className="adminError">{error}</span>}
      </div>

      <div className="adminStack">
        <AdminCustomersPanel />

        <section className="adminCard">
          <div className="adminCardHeader">
            <h3>Access</h3>
            <p>Signup starts closed. Use allowlist mode with one email or domain per line, or set signup_mode to all.</p>
          </div>
          <label className="configLabel compactConfigLabel">
            <span>signup_mode</span>
            <select className="adminSelect" value={signupMode} onChange={(event) => setSignupMode(event.target.value)}>
              <option value="forbidden">forbidden</option>
              <option value="allowlist">allowlist</option>
              <option value="all">all</option>
            </select>
          </label>
          {signupMode === 'allowlist' && (
            <label className="configLabel">
              <span>signup_allowed_emails</span>
              <textarea
                className="adminTextarea"
                rows={7}
                spellCheck={false}
                placeholder={'pilot@gmail.com\nfounder@gmail.com\n@trusted.test'}
                value={allowedEmails}
                onChange={(event) => setAllowedEmails(event.target.value)}
              />
            </label>
          )}
        </section>

        <section className="adminCard integrationCard">
          <div className="adminCardHeader">
            <h3>Fibe Integration</h3>
            <p>Connection used for project creation, workspace provisioning, and agent messaging.</p>
          </div>
          {renderConfigFields(['fibe_base_url', 'fibe_api_key'])}
          <div className="adminCardHeader withAction">
            <div>
              <h3>Agent and Server Pool</h3>
              <p>New projects store a deterministic pair selected from normalized email. Existing projects keep their stored pair.</p>
            </div>
            <button className="ghostButton" type="button" onClick={() => setPoolRows((rows) => [...rows, makePoolRow()])}>
              <Plus size={17} /> Add pair
            </button>
          </div>
          <div className="poolRows">
            {poolRows.length === 0 && <div className="emptyPool">No pool pairs configured. Add one agent and server pair before onboarding users.</div>}
            {poolRows.map((row, index) => (
              <div className="poolRow" key={row.id}>
                <label>
                  <span>Label</span>
                  <input value={row.label} placeholder={`Pair ${index + 1}`} onChange={(event) => setPoolRow(row.id, { label: event.target.value })} />
                </label>
                <label>
                  <span>Agent ID</span>
                  <input value={row.agentId} placeholder="agent_..." onChange={(event) => setPoolRow(row.id, { agentId: event.target.value })} />
                </label>
                <label>
                  <span>Server ID</span>
                  <input value={row.serverId} placeholder="server_..." onChange={(event) => setPoolRow(row.id, { serverId: event.target.value })} />
                </label>
                <button className="smallIconButton" type="button" aria-label="Remove pair" onClick={() => setPoolRows((rows) => rows.filter((candidate) => candidate.id !== row.id))}>
                  <Trash2 size={17} />
                </button>
              </div>
            ))}
          </div>
        </section>

        {ADMIN_CONFIG_SECTIONS.slice(1).map((section) => (
          <section className="adminCard integrationCard" key={section.title}>
            <div className="adminCardHeader">
              <h3>{section.title}</h3>
              <p>{section.body} Secrets are write-only; leave them blank to keep the current value.</p>
            </div>
            {renderConfigFields(section.keys)}
          </section>
        ))}
      </div>

      <button className="primaryButton adminSave" disabled={saving} onClick={save}>
        {saving ? <><Loader2 size={17} className="spin" /> Saving</> : 'Save'}
      </button>
    </section>
  );
}
