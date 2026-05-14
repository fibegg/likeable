import React, { useEffect, useMemo, useRef, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { CircleStop, ExternalLink, FolderOpen, Languages, LayoutPanelLeft, Loader2, LogOut, Minimize2, MessageSquare, Paperclip, Send, Settings, UserRound, X } from 'lucide-react';
import './styles.css';
import { Admin } from './admin';
import { api } from './api';
import { AgentNotificationRow, AppDialog, CanvasLoader, ConfirmDeleteProject, ConfirmExportProject, ConfirmNewProject, DeleteAllAccountDialog, EmptyCanvas, ProjectList, UserMessageRow } from './builder_components';
import { BASIC_CHAT_COLLAPSED_KEY, BASIC_CHAT_HEIGHT_KEY, BUILDER_MODE_KEY, COLLAPSED_CHAT_POSITION_KEY, MAX_ATTACHMENTS, SINGLE_VIEW_QUERY } from './config';
import type { AppDialogConfig, BuilderMode, BusyPolicy, Feed, FeedRow, Message, MessageQuota, Me, PendingAttachment, PreviewStatus, Project, ProjectArchiveResponse, ProjectExportResponse, ProjectListResponse, ProjectService, UserNotice } from './domain';
import { feedAwaitingAgent, feedHasAssistantAfterLatestUser, feedLiveIdle, feedRows } from './feed';
import { formatResetCountdown } from './format';
import { I18nProvider, resetCountdownLabels, useI18n } from './i18n';
import { installPwa } from './pwa';
import { ProfilePanel } from './profile_panel';
import { clampBasicChatHeight, defaultBasicChatHeight, singleViewScreen } from './viewport';

installPwa();

const LOCAL_AGENT_RUN_MAX_MS = 30 * 60_000;
const LOCAL_AGENT_IDLE_GRACE_MS = 10_000;
const COLLAPSED_CHAT_EDGE_MARGIN = 28;

function App() {
  const [me, setMe] = useState<Me | null>(null);
  const [route, setRoute] = useState(location.pathname);

  useEffect(() => {
    api<Me>('/api/me').then(setMe).catch(() => setMe({ user: null }));
    const onPop = () => setRoute(location.pathname);
    addEventListener('popstate', onPop);
    return () => removeEventListener('popstate', onPop);
  }, []);

  const nav = (to: string) => {
    history.pushState(null, '', to);
    setRoute(to);
  };

  const { t } = useI18n();

  if (!me) return <div className="loading">{t('app.loading')}</div>;

  return (
    <Shell me={me} nav={nav}>
      {route.startsWith('/admin') && me.isAdmin ? <Admin /> : <Builder nav={nav} me={me} profileRoute={route.startsWith('/profile')} />}
    </Shell>
  );
}

function Shell({ me, nav, children }: { me: Me; nav: (to: string) => void; children: React.ReactNode }) {
  const { t } = useI18n();
  const [notices, setNotices] = useState<UserNotice[]>(me.notices ?? []);
  const online = useOnlineStatus();
  const googleReady = me.auth?.googleConfigured !== false;
  useEffect(() => setNotices(me.notices ?? []), [me.notices]);
  const notice = notices[0];
  const dismissNotice = async (noticeID: string) => {
    setNotices((current) => current.filter((candidate) => candidate.id !== noticeID));
    try {
      await api(`/api/messages/${noticeID}/dismiss`, { method: 'POST' });
    } catch {
      // The banner is intentionally optimistic; the next /api/me refresh will reconcile.
    }
  };
  return (
    <div className="shell">
      <header className="topbar">
        <button className="brand" onClick={() => nav('/')} aria-label={t('builder.brand.tooltip')}>
          <span className="mark small statusMark"><span className="markGlyph">L</span><span className="brandStatusDot" /></span>
        </button>
        <nav>
          <button onClick={() => nav('/')}><MessageSquare size={18} /> {t('nav.builder')}</button>
          {me.user && <button onClick={() => nav('/profile')}><UserRound size={18} /> {t('nav.profile')}</button>}
          {me.isAdmin && <button onClick={() => nav('/admin')}><Settings size={18} /> {t('nav.admin')}</button>}
        </nav>
        <div className="account">
          <LanguageToggle />
          {me.user ? (
            <>
              <span>{me.user.email}</span>
              <button onClick={() => fetch('/api/auth/logout', { method: 'POST' }).then(() => location.reload())}><LogOut size={17} /></button>
            </>
          ) : (
            <>
              <a className={!googleReady ? 'disabled' : ''} href="/api/auth/google/start">{t('auth.signIn')}</a>
              {me.auth?.devAuth && <button onClick={() => fetch('/api/dev/login?email=admin@example.com', { method: 'POST' }).then(() => location.reload())}>{t('auth.dev')}</button>}
            </>
          )}
        </div>
      </header>
      {(!online || notice) && (
        <div className="noticeStack">
          {!online && (
            <div className="systemNotice warning">
              <strong>{t('notice.offlineTitle')}</strong>
              <span>{t('notice.offlineBody')}</span>
            </div>
          )}
          {notice && (
            <div className={`systemNotice ${notice.severity}`}>
              <strong>{t('notice.system')}</strong>
              <span>{notice.body}</span>
              <button onClick={() => void dismissNotice(notice.id)} aria-label={t('notice.dismiss')}><X size={14} /></button>
            </div>
          )}
        </div>
      )}
      <main className="workspace">{children}</main>
    </div>
  );
}

function LanguageToggle({ className = '' }: { className?: string }) {
  const { locale, setLocale, t } = useI18n();
  const nextLocale = locale === 'en' ? 'uk' : 'en';
  const currentLanguage = locale === 'en' ? t('language.english') : t('language.ukrainian');
  const tip = t('language.current', { language: currentLanguage });
  return (
    <button
      className={className || 'languageToggle'}
      onClick={() => setLocale(nextLocale)}
      aria-label={t('language.switch')}
      title={tip}
      data-tip={tip}
    >
      <Languages size={16} />
      <span>{locale === 'en' ? 'UA' : 'EN'}</span>
    </button>
  );
}

function useOnlineStatus() {
  const [online, setOnline] = useState(() => navigator.onLine);

  useEffect(() => {
    const handleOnline = () => setOnline(true);
    const handleOffline = () => setOnline(false);
    addEventListener('online', handleOnline);
    addEventListener('offline', handleOffline);
    return () => {
      removeEventListener('online', handleOnline);
      removeEventListener('offline', handleOffline);
    };
  }, []);

  return online;
}

function Builder({ nav, me, profileRoute = false }: { nav: (to: string) => void; me: Me; profileRoute?: boolean }) {
  const { t } = useI18n();
  const signedIn = Boolean(me.user);
  const googleReady = me.auth?.googleConfigured !== false;
  const [projects, setProjects] = useState<Project[]>([]);
  const [activeID, setActiveID] = useState<string>('');
  const [feed, setFeed] = useState<Feed | null>(null);
  const [prompt, setPrompt] = useState('');
  const [busy, setBusy] = useState(false);
  const [messageSubmitting, setMessageSubmitting] = useState(false);
  const [pendingAgentRuns, setPendingAgentRuns] = useState<Record<string, number>>({});
  const [projectCap, setProjectCap] = useState<number | null>(null);
  const [showProjects, setShowProjects] = useState(false);
  const [showProfile, setShowProfile] = useState(profileRoute && signedIn);
  const [confirmNewProject, setConfirmNewProject] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Project | null>(null);
  const [exportTarget, setExportTarget] = useState<Project | null>(null);
  const [exportingID, setExportingID] = useState('');
  const [exportingMode, setExportingMode] = useState<'github' | 'zip' | ''>('');
  const [controllingProjectID, setControllingProjectID] = useState('');
  const [dialog, setDialog] = useState<AppDialogConfig | null>(null);
  const [iframeLoaded, setIframeLoaded] = useState(false);
  const [previewStatus, setPreviewStatus] = useState<PreviewStatus | null>(null);
  const [attachments, setAttachments] = useState<PendingAttachment[]>([]);
  const [draggingFiles, setDraggingFiles] = useState(false);
  const [basicChatCollapsed, setBasicChatCollapsed] = useState(() => localStorage.getItem(BASIC_CHAT_COLLAPSED_KEY) === 'true');
  const [basicChatHeight, setBasicChatHeight] = useState(() => {
    const stored = Number(localStorage.getItem(BASIC_CHAT_HEIGHT_KEY));
    return Number.isFinite(stored) && stored > 0 ? clampBasicChatHeight(stored) : defaultBasicChatHeight();
  });
  const [collapsedChatPosition, setCollapsedChatPosition] = useState(() => initialCollapsedChatPosition());
  const [busyPolicy, setBusyPolicy] = useState<BusyPolicy>('queue');
  const [messageQuota, setMessageQuota] = useState<MessageQuota | null>(me.messageQuota ?? null);
  const [quotaNow, setQuotaNow] = useState(Date.now());
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);
  const messagesRef = useRef<HTMLDivElement | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const dragDepthRef = useRef(0);
  const [singleView, setSingleView] = useState(singleViewScreen);
  const [mode, setMode] = useState<BuilderMode>(() => {
    return localStorage.getItem(BUILDER_MODE_KEY) === 'split' ? 'split' : 'overlay';
  });
  const viewMode: BuilderMode = singleView ? 'overlay' : mode;
  const active = useMemo(() => projects.find((p) => p.id === activeID), [projects, activeID]);
  const activeProject = feed?.project?.id === activeID ? feed.project : active;
  const selectedService = useMemo(() => selectedProjectService(activeProject), [activeProject]);
  const activePreviewURL = selectedService?.url ?? activeProject?.previewUrl ?? '';
  const rawRows = useMemo(() => feedRows(feed), [feed]);
  const rows = useMemo(() => normalizeActiveNotificationRows(rawRows), [rawRows]);
  const pendingAgentStartedAt = activeProject?.id ? pendingAgentRuns[activeProject.id] : undefined;
  const pendingAgentAge = typeof pendingAgentStartedAt === 'number' ? Date.now() - pendingAgentStartedAt : null;
  const localAgentRunActive = Boolean(
    pendingAgentAge != null
    && pendingAgentAge < LOCAL_AGENT_RUN_MAX_MS
    && !feedHasAssistantAfterLatestUser(feed)
    && (pendingAgentAge < LOCAL_AGENT_IDLE_GRACE_MS || !feedLiveIdle(feed))
  );
  const agentWorking = Boolean(signedIn && activeProject?.status === 'ready' && activePreviewURL && (messageSubmitting || localAgentRunActive || feed?.live?.isProcessing || feedAwaitingAgent(feed)));
  const agentWorkingLabel = messageSubmitting ? t('builder.agent.transmitting') : t('builder.agent.synthesizing');
  const lastRow = rows.at(-1);
  const lastRowSignature = lastRow ? `${lastRow.id}:${lastRow.body}` : '';
  const modeLabel = viewMode === 'overlay' ? t('builder.mode.basic') : t('builder.mode.split');
  const quotaProjectCount = projects.filter((project) => project.status !== 'archived' && project.status !== 'deleting').length;
  const projectCapLabel = projectCap == null ? `${quotaProjectCount}` : `${quotaProjectCount}/${projectCap}`;
  const messageQuotaLabel = messageQuota ? `${messageQuota.remaining}/${messageQuota.limit}` : '';
  const messageQuotaTooltip = messageQuota ? t('builder.messageQuota.tooltip', { paid: messageQuota.paidRemaining ?? 0, reset: formatResetCountdown(messageQuota.resetsAt, quotaNow, resetCountdownLabels(t)) }) : '';
  const githubConnected = Boolean(me.githubConnected);
  const githubNeedsReconnect = Boolean(me.githubNeedsReconnect);
  const projectArchived = activeProject?.status === 'archived';
  const isProjectStarting = activeProject?.status === 'creating' || activeProject?.status === 'launching';
  const previewRuntimeActive = activeProject?.status === 'ready';
  const previewMaintenance = Boolean(activePreviewURL && previewRuntimeActive && previewStatus?.maintenance);
  const previewReady = Boolean(activePreviewURL && previewRuntimeActive && previewStatus?.ready);
  const previewDisplayable = Boolean(activePreviewURL && previewRuntimeActive && (previewReady || previewMaintenance));
  const canvasStatusLabel = agentWorking ? t('builder.status.agentWorking') : previewMaintenance ? t('builder.status.maintenance') : activeProject?.status === 'ready' ? (previewReady ? t('builder.status.canvasLive') : t('builder.status.canvasStarting')) : isProjectStarting ? t('builder.status.canvasStarting') : projectArchived ? t('builder.status.canvasArchived') : activeProject?.status === 'stopped' ? t('builder.status.canvasStopped') : activeProject?.status === 'error' ? t('builder.status.canvasError') : t('builder.status.canvasIdle');
  const hasDraft = Boolean(prompt.trim()) || attachments.length > 0;
  const canSend = signedIn && !projectArchived && hasDraft && !busy && !messageSubmitting && Boolean(activePreviewURL) && (activeProject?.status === 'ready' || previewReady);
  const hasActiveNotification = rows.some((row) => row.kind === 'notification' && row.active);
  const utilityScreenOpen = showProjects || showProfile;
  const inputPlaceholder = !signedIn
    ? t('builder.placeholder.signIn')
    : isProjectStarting
    ? t('builder.placeholder.starting')
    : projectArchived
      ? t('builder.placeholder.archived')
    : activeProject?.status === 'error'
      ? t('builder.placeholder.error')
      : singleView
        ? t('builder.placeholder.single')
        : t('builder.placeholder.default');

  const loadProjects = () => api<ProjectListResponse>('/api/projects').then((r) => {
    const nextProjects = Array.isArray(r.projects) ? r.projects : [];
    setProjects(nextProjects);
    setProjectCap(typeof r.projectCap === 'number' ? r.projectCap : null);
    setActiveID((current) => {
      if (current && nextProjects.some((project) => project.id === current)) return current;
      return nextProjects[0]?.id ?? '';
    });
  });
  const refreshQuota = () => api<Me>('/api/me')
    .then((next) => setMessageQuota(next.messageQuota ?? null))
    .catch(() => undefined);
  const rememberPendingAgentRun = (projectID: string) => {
    setPendingAgentRuns((current) => ({ ...current, [projectID]: Date.now() }));
  };
  const forgetPendingAgentRun = (projectID: string) => {
    setPendingAgentRuns((current) => {
      if (!current[projectID]) return current;
      const { [projectID]: _removed, ...rest } = current;
      return rest;
    });
  };

  useEffect(() => {
    if (!signedIn) {
      setProjects([]);
      setActiveID('');
      setFeed(null);
      setPendingAgentRuns({});
      return;
    }
    void loadProjects().catch(() => {
      setProjects([]);
      setActiveID('');
    });
    void refreshQuota();
  }, [signedIn]);
  useEffect(() => {
    localStorage.setItem(BUILDER_MODE_KEY, mode);
  }, [mode]);
  useEffect(() => {
    document.documentElement.dataset.builderMode = viewMode;
    return () => {
      delete document.documentElement.dataset.builderMode;
    };
  }, [viewMode]);
  useEffect(() => {
    const query = window.matchMedia(SINGLE_VIEW_QUERY);
    const update = () => setSingleView(query.matches);
    update();
    query.addEventListener('change', update);
    return () => query.removeEventListener('change', update);
  }, []);
  useEffect(() => {
    localStorage.setItem(BASIC_CHAT_COLLAPSED_KEY, String(basicChatCollapsed));
  }, [basicChatCollapsed]);
  useEffect(() => {
    localStorage.setItem(BASIC_CHAT_HEIGHT_KEY, String(Math.round(basicChatHeight)));
  }, [basicChatHeight]);
  useEffect(() => {
    localStorage.setItem(COLLAPSED_CHAT_POSITION_KEY, JSON.stringify(collapsedChatPosition));
  }, [collapsedChatPosition]);
  useEffect(() => {
    const resize = () => {
      setBasicChatHeight((height) => clampBasicChatHeight(height));
      setCollapsedChatPosition((position) => clampCollapsedChatPosition(position));
    };
    addEventListener('resize', resize);
    return () => removeEventListener('resize', resize);
  }, []);
  useEffect(() => {
    setShowProfile(profileRoute && signedIn);
    if (profileRoute && signedIn) {
      setShowProjects(false);
      setBasicChatCollapsed(false);
    }
  }, [profileRoute, signedIn]);
  useEffect(() => {
    setMessageQuota(me.messageQuota ?? null);
  }, [me.messageQuota]);
  useEffect(() => {
    const timer = setInterval(() => setQuotaNow(Date.now()), 30000);
    return () => clearInterval(timer);
  }, []);

  useEffect(() => {
    if (!activeID) return;
    const load = () => api<Feed>(`/api/projects/${activeID}/feed`).then((nextFeed) => {
      setFeed((current) => mergeFeedSnapshot(current, nextFeed));
    }).catch((err) => {
      if (err instanceof Error && err.message.includes('project not found')) {
        setFeed(null);
        void loadProjects();
        return;
      }
      console.error(err);
    });
    void load();
    const timer = setInterval(load, agentWorking ? 1200 : 4000);
    return () => clearInterval(timer);
  }, [activeID, agentWorking]);
  useEffect(() => {
    if (!feed?.project) return;
    setProjects((current) => current.map((project) => project.id === feed.project.id ? feed.project : project));
  }, [feed?.project]);
  useEffect(() => {
    if (!feed?.project) return;
    const pendingStartedAt = pendingAgentRuns[feed.project.id];
    const pendingAgeMs = typeof pendingStartedAt === 'number' ? Date.now() - pendingStartedAt : null;
    const idleSettled = feedLiveIdle(feed) && (pendingAgeMs == null || pendingAgeMs >= LOCAL_AGENT_IDLE_GRACE_MS);
    if (feed.project.status !== 'ready' || idleSettled || feedHasAssistantAfterLatestUser(feed)) {
      forgetPendingAgentRun(feed.project.id);
    }
  }, [feed, pendingAgentRuns]);
  useEffect(() => {
    const textarea = textareaRef.current;
    if (!textarea) return;
    const maxHeight = singleView ? 112 : 180;
    const minHeight = singleView ? 44 : 50;
    textarea.style.height = 'auto';
    const nextHeight = Math.min(textarea.scrollHeight, maxHeight);
    textarea.style.height = `${Math.max(nextHeight, minHeight)}px`;
    textarea.style.overflowY = textarea.scrollHeight > maxHeight ? 'auto' : 'hidden';
  }, [prompt, viewMode, singleView]);
  useEffect(() => {
    setIframeLoaded(false);
    setPreviewStatus(null);
  }, [activeProject?.id, activePreviewURL, activeProject?.status]);
  useEffect(() => {
    if (!activeProject?.id || !activePreviewURL || activeProject.status === 'deleting') {
      setPreviewStatus(null);
      return;
    }
    let cancelled = false;
    const load = () => api<PreviewStatus>(`/api/projects/${activeProject.id}/preview-status`)
      .then((status) => {
        if (cancelled) return;
        setPreviewStatus(status);
        const projectSnapshot = status.project;
        if (projectSnapshot) {
          setProjects((current) => current.map((project) => project.id === projectSnapshot.id ? projectSnapshot : project));
          setFeed((current) => current?.project.id === projectSnapshot.id ? { ...current, project: projectSnapshot } : current);
        }
        if (!status.ready && !status.maintenance) setIframeLoaded(false);
        if (status.ready && projectSnapshot?.status !== 'stopped' && activeProject.status !== 'ready') {
          setProjects((current) => current.map((project) => project.id === activeProject.id ? { ...project, status: 'ready', errorMessage: '' } : project));
          setFeed((current) => current?.project.id === activeProject.id ? { ...current, project: { ...current.project, status: 'ready', errorMessage: '' } } : current);
        }
      })
      .catch(() => {
        if (cancelled) return;
        setPreviewStatus({ ready: false, status: 'starting', checkedAt: new Date().toISOString() });
        setIframeLoaded(false);
      });
    void load();
    const timer = setInterval(load, (previewStatus?.ready || previewStatus?.maintenance) && !agentWorking ? 5000 : 1500);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [activeProject?.id, activePreviewURL, activeProject?.status, agentWorking, previewStatus?.maintenance, previewStatus?.ready]);
  useEffect(() => {
    setAttachments([]);
    dragDepthRef.current = 0;
    setDraggingFiles(false);
  }, [activeID]);
  useEffect(() => {
    const messages = messagesRef.current;
    if (!messages) return;
    messages.scrollTop = messages.scrollHeight;
  }, [activeID, lastRowSignature, agentWorking]);

  const createOrSend = async () => {
    if (!signedIn) return;
    const text = prompt.trim();
    const files = attachments;
    if (!text && files.length === 0) return;
    const optimisticID = `optimistic-${crypto.randomUUID()}`;
    const optimisticMessage: Message = {
      id: optimisticID,
      role: 'user',
      body: text,
      createdAt: new Date().toISOString(),
      attachments: files.map((attachment) => ({
        id: attachment.id,
        filename: attachment.file.name,
        contentType: attachment.file.type,
        size: attachment.file.size
      }))
    };
    if (activeProject) {
      setFeed((current) => {
        if (current?.project.id === activeProject.id) {
          return { ...current, localMessages: [...(current.localMessages ?? []), optimisticMessage] };
        }
        return { project: activeProject, localMessages: [optimisticMessage], messages: [], activity: [], live: null };
      });
    }
    setBusy(true);
    setMessageSubmitting(true);
    let requestAccepted = false;
    try {
      if (activeProject) {
        if (files.length > 0) {
          const form = new FormData();
          form.append('text', text);
          form.append('busy_policy', busyPolicy);
          files.forEach((attachment) => form.append('attachments', attachment.file, attachment.file.name));
          await api(`/api/projects/${activeProject.id}/messages`, { method: 'POST', body: form });
        } else {
          await api(`/api/projects/${activeProject.id}/messages`, { method: 'POST', body: JSON.stringify({ text, busy_policy: busyPolicy }) });
        }
        requestAccepted = true;
        rememberPendingAgentRun(activeProject.id);
        setPrompt('');
        setAttachments([]);
        try {
          setFeed(await api<Feed>(`/api/projects/${activeProject.id}/feed`));
        } catch (err) {
          console.error(err);
        }
        void refreshQuota();
      }
    } catch (err) {
      if (!requestAccepted) {
        setFeed((current) => {
          if (!current || current.project.id !== activeProject?.id) return current;
          return { ...current, localMessages: (current.localMessages ?? []).filter((message) => message.id !== optimisticID) };
        });
      }
      setDialog({ title: t('dialog.requestFailed.title'), body: err instanceof Error ? err.message : t('dialog.requestFailed.title'), confirmLabel: t('common.close') });
    } finally {
      setMessageSubmitting(false);
      setBusy(false);
    }
  };
  const interruptAgent = async () => {
    if (!signedIn || !activeProject) return;
    setBusy(true);
    try {
      await api(`/api/projects/${activeProject.id}/agent/interrupt`, { method: 'POST', body: JSON.stringify({}) });
      forgetPendingAgentRun(activeProject.id);
      setMessageSubmitting(false);
      setFeed(await api<Feed>(`/api/projects/${activeProject.id}/feed`));
    } catch (err) {
      setDialog({ title: t('dialog.stopFailed.title'), body: err instanceof Error ? err.message : t('dialog.requestFailed.title'), tone: 'warning', confirmLabel: t('common.close') });
    } finally {
      setBusy(false);
    }
  };
  const createProject = async (title?: string) => {
    if (!signedIn) return;
    setBusy(true);
    try {
      const trimmedTitle = title?.trim();
      const res = await api<{ project: Project }>('/api/projects', { method: 'POST', body: JSON.stringify({ confirm: true, title: trimmedTitle || undefined }) });
      setConfirmNewProject(false);
      setShowProjects(false);
      setShowProfile(false);
      await loadProjects();
      void refreshQuota();
      setActiveID(res.project.id);
      setFeed({ project: res.project, localMessages: [], messages: [], activity: [], live: null });
      nav('/');
    } catch (err) {
      setDialog({ title: t('dialog.projectFailed.title'), body: err instanceof Error ? err.message : t('dialog.requestFailed.title'), tone: 'warning', confirmLabel: t('common.close') });
    } finally {
      setBusy(false);
    }
  };
  const renameProject = async (project: Project, title: string) => {
    if (!signedIn) return;
    const trimmedTitle = title.trim();
    if (!trimmedTitle || trimmedTitle === project.title) return;
    setBusy(true);
    try {
      const res = await api<{ project: Project }>(`/api/projects/${project.id}`, { method: 'PATCH', body: JSON.stringify({ title: trimmedTitle }) });
      setProjects((current) => current.map((item) => item.id === project.id ? res.project : item));
      setFeed((current) => current?.project.id === project.id ? { ...current, project: res.project } : current);
    } catch (err) {
      setDialog({ title: t('dialog.renameFailed.title'), body: err instanceof Error ? err.message : t('dialog.requestFailed.title'), tone: 'warning', confirmLabel: t('common.close') });
    } finally {
      setBusy(false);
    }
  };
  const selectService = async (service: ProjectService) => {
    if (!signedIn || !activeProject || service.name === activeProject.selectedServiceName) return;
    setBusy(true);
    try {
      const res = await api<{ project: Project }>(`/api/projects/${activeProject.id}`, { method: 'PATCH', body: JSON.stringify({ selectedServiceName: service.name }) });
      setPreviewStatus(null);
      setIframeLoaded(false);
      setProjects((current) => current.map((item) => item.id === res.project.id ? res.project : item));
      setFeed((current) => current?.project.id === res.project.id ? { ...current, project: res.project } : current);
    } catch (err) {
      setDialog({ title: t('dialog.serviceSwitchFailed.title'), body: err instanceof Error ? err.message : t('dialog.requestFailed.title'), tone: 'warning', confirmLabel: t('common.close') });
    } finally {
      setBusy(false);
    }
  };
  const deleteProject = async () => {
    if (!signedIn || !deleteTarget) return;
    const targetID = deleteTarget.id;
    setBusy(true);
    try {
      await api(`/api/projects/${targetID}`, { method: 'DELETE' });
      forgetPendingAgentRun(targetID);
      const remaining = projects.filter((project) => project.id !== targetID);
      setProjects(remaining);
      setProjectCap((cap) => cap);
      setDeleteTarget(null);
      void refreshQuota();
      if (activeID === targetID) {
        setActiveID(remaining[0]?.id ?? '');
        setFeed(null);
      }
    } catch (err) {
      setDialog({ title: t('dialog.deleteFailed.title'), body: err instanceof Error ? err.message : t('dialog.requestFailed.title'), tone: 'danger', confirmLabel: t('common.close') });
    } finally {
      setBusy(false);
    }
  };
  const controlProjectPlayground = async (project: Project, action: 'start' | 'stop' | 'restart') => {
    if (!signedIn) return;
    setBusy(true);
    setControllingProjectID(project.id);
    try {
      const res = await api<{ project: Project }>(`/api/projects/${project.id}/playground`, { method: 'POST', body: JSON.stringify({ action }) });
      setPreviewStatus(null);
      setIframeLoaded(false);
      setProjects((current) => current.map((item) => item.id === project.id ? res.project : item));
      setFeed((current) => current?.project.id === project.id ? { ...current, project: res.project } : current);
    } catch (err) {
      setDialog({ title: t('dialog.playgroundActionFailed.title'), body: err instanceof Error ? err.message : t('dialog.requestFailed.title'), tone: 'warning', confirmLabel: t('common.close') });
    } finally {
      setControllingProjectID('');
      setBusy(false);
    }
  };
  const requestProjectExport = (project: Project) => {
    setExportTarget(project);
  };
  const connectGithub = () => {
    location.href = '/api/profile/github/start';
  };
  const exportProject = async (project: Project, repoName: string, privateRepo: boolean) => {
    if (!signedIn) return;
    setBusy(true);
    setExportingID(project.id);
    setExportingMode('github');
    try {
      const res = await api<ProjectExportResponse>(`/api/projects/${project.id}/export`, {
        method: 'POST',
        body: JSON.stringify({ repoName, private: privateRepo })
      });
      setExportTarget(null);
      setDialog({
        title: t('dialog.exportReady.title'),
        body: t('dialog.exportReady.bodyProject', { title: project.title, url: res.githubRepoUrl }),
        confirmLabel: t('dialog.exportReady.openGitHub'),
        onConfirm: () => {
          window.open(res.githubRepoUrl, '_blank', 'noopener,noreferrer');
        }
      });
    } catch (err) {
      setExportTarget(null);
      setDialog({ title: t('dialog.exportFailed.title'), body: err instanceof Error ? err.message : t('dialog.requestFailed.title'), tone: 'warning', confirmLabel: t('common.close') });
    } finally {
      setExportingID('');
      setExportingMode('');
      setBusy(false);
    }
  };
  const exportProjectZip = async (project: Project) => {
    if (!signedIn) return;
    setBusy(true);
    setExportingID(project.id);
    setExportingMode('zip');
    try {
      const res = await api<ProjectArchiveResponse>(`/api/projects/${project.id}/archive`, {
        method: 'POST',
        body: JSON.stringify({})
      });
      setExportTarget(null);
      location.href = res.downloadUrl;
    } catch (err) {
      setExportTarget(null);
      setDialog({ title: t('dialog.zipExportFailed.title'), body: err instanceof Error ? err.message : t('dialog.requestFailed.title'), tone: 'warning', confirmLabel: t('common.close') });
    } finally {
      setExportingID('');
      setExportingMode('');
      setBusy(false);
    }
  };
  const handleComposerKeyDown = (event: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (singleView) return;
    if (event.key !== 'Enter' || event.shiftKey || event.nativeEvent.isComposing) return;
    event.preventDefault();
    if (canSend) void createOrSend();
  };
  const addFiles = (fileList: FileList | File[]) => {
    const nextFiles = Array.from(fileList).filter((file) => file.size > 0);
    if (nextFiles.length === 0) return;
    setAttachments((current) => {
      const slots = Math.max(0, MAX_ATTACHMENTS - current.length);
      return [
        ...current,
        ...nextFiles.slice(0, slots).map((file) => ({
          id: `${file.name}-${file.size}-${file.lastModified}-${crypto.randomUUID()}`,
          file
        }))
      ];
    });
  };
  const removeAttachment = (id: string) => {
    setAttachments((current) => current.filter((attachment) => attachment.id !== id));
  };
  const startBasicChatResize = (event: React.PointerEvent<HTMLDivElement>) => {
    if (viewMode !== 'overlay' || basicChatCollapsed) return;
    event.preventDefault();
    event.currentTarget.setPointerCapture?.(event.pointerId);
    const startY = event.clientY;
    const startHeight = basicChatHeight;
    document.documentElement.classList.add('resizingChat');
    const onMove = (moveEvent: PointerEvent) => {
      moveEvent.preventDefault();
      setBasicChatHeight(clampBasicChatHeight(startHeight + startY - moveEvent.clientY));
    };
    const onUp = () => {
      document.documentElement.classList.remove('resizingChat');
      removeEventListener('pointermove', onMove);
      removeEventListener('pointerup', onUp);
      removeEventListener('pointercancel', onUp);
    };
    addEventListener('pointermove', onMove);
    addEventListener('pointerup', onUp);
    addEventListener('pointercancel', onUp);
  };
  const startCollapsedChatDrag = (event: React.PointerEvent<HTMLButtonElement>) => {
    if (viewMode !== 'overlay' || !basicChatCollapsed) return;
    event.preventDefault();
    event.currentTarget.setPointerCapture?.(event.pointerId);
    const startX = event.clientX;
    const startY = event.clientY;
    const startPosition = clampCollapsedChatPosition(collapsedChatPosition);
    let moved = false;
    document.documentElement.classList.add('draggingCollapsedChat');
    const onMove = (moveEvent: PointerEvent) => {
      moveEvent.preventDefault();
      const dx = moveEvent.clientX - startX;
      const dy = moveEvent.clientY - startY;
      if (Math.hypot(dx, dy) > 5) moved = true;
      setCollapsedChatPosition(clampCollapsedChatPosition({
        x: startPosition.x + dx,
        y: startPosition.y + dy
      }));
    };
    const finish = (cancelled = false) => {
      document.documentElement.classList.remove('draggingCollapsedChat');
      removeEventListener('pointermove', onMove);
      removeEventListener('pointerup', onUp);
      removeEventListener('pointercancel', onCancel);
      if (!moved && !cancelled) {
        setBasicChatCollapsed(false);
      }
    };
    const onUp = () => finish(false);
    const onCancel = () => finish(true);
    addEventListener('pointermove', onMove);
    addEventListener('pointerup', onUp);
    addEventListener('pointercancel', onCancel);
  };
  const handleCollapsedChatKeyDown = (event: React.KeyboardEvent<HTMLButtonElement>) => {
    if (event.key !== 'Enter' && event.key !== ' ') return;
    event.preventDefault();
    setBasicChatCollapsed(false);
  };
  const chatDragHandlers = {
    onDragEnter: (event: React.DragEvent) => {
      if (!event.dataTransfer.types.includes('Files')) return;
      event.preventDefault();
      dragDepthRef.current += 1;
      setDraggingFiles(true);
    },
    onDragOver: (event: React.DragEvent) => {
      if (!event.dataTransfer.types.includes('Files')) return;
      event.preventDefault();
      event.dataTransfer.dropEffect = 'copy';
    },
    onDragLeave: (event: React.DragEvent) => {
      if (!event.dataTransfer.types.includes('Files')) return;
      event.preventDefault();
      dragDepthRef.current = Math.max(0, dragDepthRef.current - 1);
      if (dragDepthRef.current === 0) setDraggingFiles(false);
    },
    onDrop: (event: React.DragEvent) => {
      if (!event.dataTransfer.files.length) return;
      event.preventDefault();
      dragDepthRef.current = 0;
      setDraggingFiles(false);
      addFiles(event.dataTransfer.files);
    }
  };

  const modeToggle = (
    singleView ? null :
    <button
      className={`chromeIconButton splitToggle tooltip tooltipBottom ${viewMode === 'split' ? 'selected' : ''}`}
      onClick={() => setMode(viewMode === 'split' ? 'overlay' : 'split')}
      aria-label={viewMode === 'split' ? t('builder.view.useBasic') : t('builder.view.useSplit')}
      data-tip={viewMode === 'split' ? t('builder.view.useBasic') : t('builder.view.useSplit')}
    >
      <LayoutPanelLeft size={16} />
    </button>
  );
  const topOpenLink = activeProject?.status === 'ready' && activePreviewURL
    ? <a className="chromeIconButton topOpenLink tooltip tooltipBottom" href={activePreviewURL} target="_blank" aria-label={t('builder.preview.open')} data-tip={t('builder.preview.open')}><ExternalLink size={16} /></a>
    : null;
  const openProjectsPanel = () => {
    if (!signedIn) return;
    setShowProjects((open) => {
      const next = !open;
      if (next) void loadProjects().catch(() => undefined);
      return next;
    });
    setShowProfile(false);
    if (location.pathname.startsWith('/profile')) nav('/');
  };
  const openProfilePanel = () => {
    if (!signedIn) return;
    setShowProfile(true);
    setShowProjects(false);
    setBasicChatCollapsed(false);
    nav('/profile');
  };
  const closeProfilePanel = () => {
    setShowProfile(false);
    if (location.pathname.startsWith('/profile')) nav('/');
  };
  const projectTitleButton = (slotClass: string) => (
    <button className={`projectTitleButton ${slotClass} tooltip tooltipBottom`} onClick={openProjectsPanel} disabled={!signedIn} aria-label={t('projects.title')} data-tip={signedIn ? t('builder.projects.tooltip') : t('auth.signInToCreateProjects')}>
      <span className="projectTitleMain">{activeProject?.title ?? (signedIn ? t('builder.project.new') : t('auth.signInToBuild'))}</span>
      <span className="projectTitleCount"><FolderOpen size={15} /><span>{signedIn ? projectCapLabel : '-'}</span></span>
    </button>
  );
  const builderChrome = (
    <div className="basicChatChrome">
      <button className="brand chatBrand tooltip tooltipBottom" onClick={() => nav('/')} aria-label={t('builder.brand.tooltip')} data-tip={t('builder.brand.tooltip')}>
        <span className={`mark small statusMark ${agentWorking ? 'working' : ''}`}><span className="markGlyph">L</span><span className="brandStatusDot" /></span>
      </button>
      {topOpenLink}
      {projectTitleButton('chromeProjectTitle')}
      <nav className="chatNav">
        {activeProject?.services && activeProject.services.length > 1 && (
          <div className="chromePill serviceSelector" aria-label={t('service.preview')}>
            {activeProject.services.map((service) => (
              <button
                key={service.name}
                className={selectedService?.name === service.name ? 'chromeIconButton selected serviceButton tooltip tooltipBottom' : 'chromeIconButton serviceButton tooltip tooltipBottom'}
                onClick={() => void selectService(service)}
                disabled={!signedIn || busy}
                aria-label={t('service.show', { name: service.name })}
                data-tip={t('service.show', { name: service.name })}
              >
                {service.name.slice(0, 2).toUpperCase()}
              </button>
            ))}
          </div>
        )}
        <div className="chromePill identityPill">
          {agentWorking && (
            <>
              <button className="chromeIconButton stopAgentButton tooltip tooltipBottom" onClick={interruptAgent} disabled={busy} aria-label={t('builder.stopAgent')} data-tip={t('builder.stopAgent')}><CircleStop size={16} /></button>
              <button
                className="chromeIconButton busyPolicyButton tooltip tooltipBottom"
                onClick={() => setBusyPolicy((policy) => policy === 'queue' ? 'steer' : 'queue')}
                aria-label={busyPolicy === 'queue' ? t('builder.busy.queue') : t('builder.busy.steer')}
                data-tip={busyPolicy === 'queue' ? t('builder.busy.queue') : t('builder.busy.steer')}
              >
                {busyPolicy === 'queue' ? 'Q' : 'S'}
              </button>
            </>
          )}
          {messageQuota && (
            <span className="messageQuotaBadge tooltip tooltipBottom" data-tip={messageQuotaTooltip} aria-label={t('builder.messages.left')}>
              {messageQuotaLabel}
            </span>
          )}
          <LanguageToggle className="chromeIconButton tooltip tooltipBottom languageChromeButton" />
          <button className={showProfile ? 'chromeIconButton selected tooltip tooltipBottom' : 'chromeIconButton tooltip tooltipBottom'} onClick={showProfile ? closeProfilePanel : openProfilePanel} disabled={!signedIn} aria-label={t('nav.profile')} data-tip={signedIn ? t('builder.profile.tooltip') : t('auth.signInToOpenProfile')}><UserRound size={16} /></button>
          {me.isAdmin && <button className="chromeIconButton tooltip tooltipBottom" onClick={() => nav('/admin')} aria-label={t('nav.admin')} data-tip={t('builder.admin.tooltip')}><Settings size={16} /></button>}
        </div>
        <div className="chromePill actionChromePill">
          {!me.user && (
            <>
              <a className={`chromeAuthLink ${!googleReady ? 'disabled' : ''}`} href="/api/auth/google/start">{t('auth.signIn')}</a>
              {me.auth?.devAuth && <button className="chromeAuthLink chromeAuthButton" onClick={() => fetch('/api/dev/login?email=admin@example.com', { method: 'POST' }).then(() => location.reload())}>{t('auth.dev')}</button>}
            </>
          )}
          {modeToggle}
          {viewMode === 'overlay' && <button className="chromeIconButton tooltip tooltipBottom" onClick={() => setBasicChatCollapsed(true)} aria-label={t('builder.chat.collapse')} data-tip={t('builder.chat.collapse')}><Minimize2 size={16} /></button>}
        </div>
      </nav>
    </div>
  );

  const chat = (
    <section className={`chatPane ${draggingFiles ? 'dragActive' : ''} ${utilityScreenOpen ? 'screenOpen' : ''}`} {...chatDragHandlers}>
      <a className="poweredBy" href="https://fibe.gg" target="_blank" rel="noopener noreferrer">
        {t('builder.poweredBy')} <span>fibe.gg</span>
      </a>
      {projectTitleButton('chatProjectTitle')}
      {builderChrome}
      {showProjects && <ProjectList projects={projects} activeID={activeID} projectCap={projectCap} busy={busy} exportingID={exportingID} controllingID={controllingProjectID} onSelect={(id) => { setActiveID(id); setShowProjects(false); }} onNew={() => setConfirmNewProject(true)} onRename={renameProject} onDelete={setDeleteTarget} onExport={requestProjectExport} onControlPlayground={controlProjectPlayground} onClose={() => setShowProjects(false)} />}
      {showProfile && <ProfilePanel me={me} onClose={closeProfilePanel} />}
      {!utilityScreenOpen && (
        <>
          <div className="messages" ref={messagesRef}>
            {rows.map((row) => row.kind === 'notification'
              ? <AgentNotificationRow key={row.id} body={row.body || agentWorkingLabel} active={row.active} elapsedMs={row.elapsedMs} />
              : <UserMessageRow key={row.id} row={row} />
            )}
            {agentWorking && !hasActiveNotification && <AgentNotificationRow body={agentWorkingLabel} active />}
          </div>
          {draggingFiles && <div className="dropOverlay"><Paperclip size={24} /> {t('builder.dropFiles')}</div>}
          <div className={`composer ${attachments.length > 0 ? 'hasAttachments' : ''}`}>
            <input ref={fileInputRef} type="file" multiple hidden onChange={(event) => {
              if (event.currentTarget.files) addFiles(event.currentTarget.files);
              event.currentTarget.value = '';
            }} />
            {attachments.length > 0 && (
              <div className="attachmentTray">
                {attachments.map((attachment) => (
                  <span className="attachmentChip" key={attachment.id}>
                    <Paperclip size={14} />
                    <span>{attachment.file.name}</span>
                    <button onClick={() => removeAttachment(attachment.id)} aria-label={t('builder.removeAttachment', { name: attachment.file.name })}><X size={13} /></button>
                  </span>
                ))}
              </div>
            )}
            <button className="attachButton" type="button" onClick={() => fileInputRef.current?.click()} disabled={!signedIn || projectArchived || attachments.length >= MAX_ATTACHMENTS} aria-label={t('builder.attachFiles')}>
              <Paperclip size={20} />
            </button>
            <textarea ref={textareaRef} value={prompt} onChange={(e) => setPrompt(e.target.value)} onKeyDown={handleComposerKeyDown} placeholder={inputPlaceholder} rows={1} disabled={!signedIn || projectArchived} />
            <button className={`sendButton ${messageSubmitting ? 'working' : ''}`} disabled={!canSend} onClick={createOrSend}>
              {messageSubmitting ? <Loader2 className="spinIcon" size={22} /> : <Send size={22} />}
            </button>
          </div>
        </>
      )}
    </section>
  );
  const minimizedChatBar = (
    <button
      className={`minimizedChatBar ${agentWorking ? 'working' : ''}`}
      aria-label={t('builder.expandChat')}
      onPointerDown={startCollapsedChatDrag}
      onKeyDown={handleCollapsedChatKeyDown}
      {...chatDragHandlers}
    >
      <span className={`mark small statusMark ${agentWorking ? 'working' : ''}`}><span className="markGlyph">L</span><span className="brandStatusDot" /></span>
    </button>
  );

  const previewTitle = activeProject?.status === 'launching' ? t('builder.preview.startingTitle') : t('builder.preview.preparingTitle');
  const previewBody = activeProject?.status === 'launching'
    ? t('builder.preview.launchingBody')
    : t('builder.preview.preparingBody');
  const connectingCanvasBody = previewReady
    ? t('builder.preview.respondedBody')
    : t('builder.preview.warmingBody');
  const previewContent = (
    <>
      {projectArchived ? (
        <CanvasLoader title={t('builder.preview.archivedTitle')} body={t('builder.preview.archivedBody')} />
      ) : activeProject?.status === 'error' && !previewMaintenance ? (
        <CanvasLoader title={t('builder.preview.launchFailedTitle')} body={t('builder.preview.launchFailedBody')} tone="error" />
      ) : activeProject?.status === 'stopped' ? (
        <CanvasLoader title={t('builder.preview.stoppedTitle')} body={t('builder.preview.stoppedBody')} />
      ) : activePreviewURL && previewDisplayable ? (
        <>
          <iframe
            title={t('builder.preview.frameTitle')}
            src={activePreviewURL}
            className={previewDisplayable && iframeLoaded ? 'loaded' : ''}
            onLoad={() => {
              if (previewDisplayable) setIframeLoaded(true);
            }}
          />
          {(!previewDisplayable || !iframeLoaded) && <CanvasLoader title={t('builder.preview.connectingTitle')} body={connectingCanvasBody} />}
        </>
      ) : isProjectStarting ? (
        <CanvasLoader title={previewTitle} body={previewBody} />
      ) : activeProject?.status === 'ready' && activePreviewURL ? (
        <>
          <iframe
            title={t('builder.preview.frameTitle')}
            src="about:blank"
            className=""
            onLoad={() => undefined}
          />
          <CanvasLoader title={t('builder.preview.connectingTitle')} body={connectingCanvasBody} />
        </>
      ) : <EmptyCanvas />}
      {viewMode === 'split' && <div className={`canvasStatus ${agentWorking ? 'working' : ''}`}><span /> {canvasStatusLabel}</div>}
    </>
  );
  const preview = (
    <section className={`previewPane ${viewMode === 'overlay' && !basicChatCollapsed ? 'chatExpanded' : ''}`}>
      <div className="previewTopChrome">{builderChrome}</div>
      <div className="previewContent">{previewContent}</div>
      {viewMode === 'overlay' && (
        <div
          className={`overlayChat ${basicChatCollapsed ? 'collapsed minimized' : ''}`}
          style={basicChatCollapsed
            ? collapsedChatStyle(collapsedChatPosition)
            : ({ '--basic-chat-height': `${basicChatHeight}px` } as React.CSSProperties)}
        >
          {!basicChatCollapsed && <div className="chatResizeHandle" aria-label={t('builder.resizeChat')} onPointerDown={startBasicChatResize} />}
          {basicChatCollapsed ? minimizedChatBar : chat}
        </div>
      )}
      {confirmNewProject && <ConfirmNewProject projectCap={projectCap} projectCount={quotaProjectCount} busy={busy} onCancel={() => setConfirmNewProject(false)} onConfirm={createProject} />}
      {deleteTarget && <ConfirmDeleteProject project={deleteTarget} busy={busy} onCancel={() => setDeleteTarget(null)} onConfirm={deleteProject} />}
      {exportTarget && <ConfirmExportProject project={exportTarget} busy={busy || exportingID === exportTarget.id} busyMode={exportingMode} githubConnected={githubConnected} githubNeedsReconnect={githubNeedsReconnect} onCancel={() => setExportTarget(null)} onGithub={(repoName, privateRepo) => void exportProject(exportTarget, repoName, privateRepo)} onZip={() => void exportProjectZip(exportTarget)} onConnectGithub={connectGithub} />}
      {dialog && <AppDialog dialog={dialog} onClose={() => setDialog(null)} />}
    </section>
  );

  return (
    <div className={viewMode === 'split' ? 'builder split' : 'builder overlay'} data-mode={modeLabel}>
      {viewMode === 'split' && chat}
      {preview}
    </div>
  );
}

function selectedProjectService(project?: Project): ProjectService | undefined {
  if (!project?.services?.length) return undefined;
  const selected = project.selectedServiceName;
  return project.services.find((service) => service.name === selected)
    ?? project.services.find((service) => service.name === 'app')
    ?? project.services[0];
}

type CollapsedChatPosition = { x: number; y: number };

function initialCollapsedChatPosition(): CollapsedChatPosition {
  const stored = localStorage.getItem(COLLAPSED_CHAT_POSITION_KEY);
  if (stored) {
    try {
      const parsed = JSON.parse(stored) as Partial<CollapsedChatPosition>;
      if (typeof parsed.x === 'number' && typeof parsed.y === 'number') {
        return clampCollapsedChatPosition({ x: parsed.x, y: parsed.y });
      }
    } catch {
      localStorage.removeItem(COLLAPSED_CHAT_POSITION_KEY);
    }
  }
  return defaultCollapsedChatPosition();
}

function defaultCollapsedChatPosition(): CollapsedChatPosition {
  return clampCollapsedChatPosition({
    x: Math.round(window.innerWidth / 2),
    y: Math.round(window.innerHeight - 48)
  });
}

function clampCollapsedChatPosition(position: CollapsedChatPosition): CollapsedChatPosition {
  const minX = COLLAPSED_CHAT_EDGE_MARGIN;
  const minY = COLLAPSED_CHAT_EDGE_MARGIN;
  const maxX = Math.max(minX, window.innerWidth - COLLAPSED_CHAT_EDGE_MARGIN);
  const maxY = Math.max(minY, window.innerHeight - COLLAPSED_CHAT_EDGE_MARGIN);
  return {
    x: Math.min(maxX, Math.max(minX, Math.round(position.x))),
    y: Math.min(maxY, Math.max(minY, Math.round(position.y)))
  };
}

function collapsedChatStyle(position: CollapsedChatPosition): React.CSSProperties {
  const clamped = clampCollapsedChatPosition(position);
  return {
    left: `${clamped.x}px`,
    top: `${clamped.y}px`,
    right: 'auto',
    bottom: 'auto',
    transform: 'translate(-50%, -50%)'
  };
}

function mergeFeedSnapshot(current: Feed | null, next: Feed): Feed {
  if (!current || current.project.id !== next.project.id) return next;
  if (next.live?.isProcessing || next.live?.streamText) return next;
  if (next.project.status !== 'ready') return next;
  const currentLive = current.live;
  if (!currentLive?.isProcessing || !currentLive.streamText) return next;
  if (feedHasAssistantAfterLatestUser(next)) return next;
  const nextLiveIdle = feedLiveIdle(next);

  return {
    ...next,
    messages: next.messages?.length ? next.messages : current.messages,
    activity: next.activity?.length ? next.activity : current.activity,
    live: {
      ...currentLive,
      conversationId: next.live?.conversationId ?? currentLive.conversationId,
      isProcessing: !nextLiveIdle,
      queuedTurns: typeof next.live?.queuedTurns === 'number' ? next.live.queuedTurns : currentLive.queuedTurns,
      startedAt: currentLive.startedAt ?? next.live?.startedAt
    }
  };
}

function normalizeActiveNotificationRows(rows: FeedRow[]): FeedRow[] {
  let latestNotificationIndex = -1;
  let latestActiveIndex = -1;
  rows.forEach((row, index) => {
    if (row.kind !== 'notification') return;
    latestNotificationIndex = index;
    if (row.active) latestActiveIndex = index;
  });
  if (latestActiveIndex === -1) return rows;

  return rows.map((row, index) => {
    if (row.kind !== 'notification' || !row.active) return row;
    if (latestActiveIndex === latestNotificationIndex && index === latestActiveIndex) return row;
    return { ...row, active: false };
  });
}

createRoot(document.getElementById('root')!).render(
  <I18nProvider>
    <App />
  </I18nProvider>
);
