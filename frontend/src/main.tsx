import React, { useEffect, useMemo, useRef, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { CircleStop, ExternalLink, FolderOpen, LayoutPanelLeft, Loader2, LogOut, Minimize2, MessageSquare, Paperclip, Send, Settings, UserRound, X } from 'lucide-react';
import './styles.css';
import { Admin } from './admin';
import { api } from './api';
import { AgentNotificationRow, AppDialog, CanvasLoader, ConfirmDeleteProject, ConfirmNewProject, DeleteAllAccountDialog, EmptyCanvas, ProjectList, UserMessageRow } from './builder_components';
import { BASIC_CHAT_COLLAPSED_KEY, BASIC_CHAT_HEIGHT_KEY, BUILDER_MODE_KEY, MAX_ATTACHMENTS, SINGLE_VIEW_QUERY } from './config';
import type { AppDialogConfig, BuilderMode, BusyPolicy, Feed, FeedRow, Message, MessageQuota, Me, PendingAttachment, PreviewStatus, Project, ProjectListResponse, ProjectService, UserNotice } from './domain';
import { feedAwaitingAgent, feedRows } from './feed';
import { formatResetCountdown, projectLaunchErrorMessage } from './format';
import { installPwa } from './pwa';
import { ProfilePanel } from './profile_panel';
import { clampBasicChatHeight, defaultBasicChatHeight, singleViewScreen } from './viewport';

installPwa();


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

  if (!me) return <div className="loading">Loading</div>;

  return (
    <Shell me={me} nav={nav}>
      {route.startsWith('/admin') && me.isAdmin ? <Admin /> : <Builder nav={nav} me={me} profileRoute={route.startsWith('/profile')} />}
    </Shell>
  );
}

function Shell({ me, nav, children }: { me: Me; nav: (to: string) => void; children: React.ReactNode }) {
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
        <button className="brand" onClick={() => nav('/')} aria-label="Likeable link stable">
          <span className="mark small statusMark">L<span className="brandStatusDot" /></span>
        </button>
        <nav>
          <button onClick={() => nav('/')}><MessageSquare size={18} /> Builder</button>
          {me.user && <button onClick={() => nav('/profile')}><UserRound size={18} /> Profile</button>}
          {me.isAdmin && <button onClick={() => nav('/admin')}><Settings size={18} /> Admin</button>}
        </nav>
        <div className="account">
          {me.user ? (
            <>
              <span>{me.user.email}</span>
              <button onClick={() => fetch('/api/auth/logout', { method: 'POST' }).then(() => location.reload())}><LogOut size={17} /></button>
            </>
          ) : (
            <>
              <a className={!googleReady ? 'disabled' : ''} href="/api/auth/google/start">Sign in</a>
              {me.auth?.devAuth && <button onClick={() => fetch('/api/dev/login?email=admin@example.com', { method: 'POST' }).then(() => location.reload())}>Dev</button>}
            </>
          )}
        </div>
      </header>
      {(!online || notice) && (
        <div className="noticeStack">
          {!online && (
            <div className="systemNotice warning">
              <strong>Offline</strong>
              <span>Cached app shell is available; live projects and messages need the network.</span>
            </div>
          )}
          {notice && (
            <div className={`systemNotice ${notice.severity}`}>
              <strong>System</strong>
              <span>{notice.body}</span>
              <button onClick={() => void dismissNotice(notice.id)} aria-label="Dismiss notice"><X size={14} /></button>
            </div>
          )}
        </div>
      )}
      <main className="workspace">{children}</main>
    </div>
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
  const signedIn = Boolean(me.user);
  const googleReady = me.auth?.googleConfigured !== false;
  const [projects, setProjects] = useState<Project[]>([]);
  const [activeID, setActiveID] = useState<string>('');
  const [feed, setFeed] = useState<Feed | null>(null);
  const [prompt, setPrompt] = useState('');
  const [busy, setBusy] = useState(false);
  const [messageSubmitting, setMessageSubmitting] = useState(false);
  const [projectCap, setProjectCap] = useState<number | null>(null);
  const [showProjects, setShowProjects] = useState(false);
  const [showProfile, setShowProfile] = useState(profileRoute && signedIn);
  const [confirmNewProject, setConfirmNewProject] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Project | null>(null);
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
  const agentWorking = Boolean(signedIn && activeProject?.status === 'ready' && activePreviewURL && (messageSubmitting || feed?.live?.isProcessing || feedAwaitingAgent(feed)));
  const agentWorkingLabel = messageSubmitting ? 'Transmitting request' : 'Synthesizing canvas';
  const lastRow = rows.at(-1);
  const lastRowSignature = lastRow ? `${lastRow.id}:${lastRow.body}` : '';
  const modeLabel = viewMode === 'overlay' ? 'Basic' : 'Split';
  const projectCapLabel = projectCap == null ? `${projects.length}` : `${projects.length}/${projectCap}`;
  const messageQuotaLabel = messageQuota ? `${messageQuota.remaining}/${messageQuota.limit}` : '';
  const messageQuotaTooltip = messageQuota ? `${messageQuota.paidRemaining ?? 0} paid credits · resets in ${formatResetCountdown(messageQuota.resetsAt, quotaNow)}` : '';
  const isProjectStarting = activeProject?.status === 'creating' || activeProject?.status === 'launching';
  const previewReady = Boolean(activePreviewURL && previewStatus?.ready);
  const canvasStatusLabel = agentWorking ? 'Agent working' : activeProject?.status === 'ready' ? (previewReady ? 'Canvas live' : 'Canvas starting') : isProjectStarting ? 'Canvas starting' : activeProject?.status === 'error' ? 'Canvas error' : 'Canvas idle';
  const hasDraft = Boolean(prompt.trim()) || attachments.length > 0;
  const canSend = signedIn && hasDraft && !busy && !messageSubmitting && Boolean(activePreviewURL) && (activeProject?.status === 'ready' || previewReady);
  const hasActiveNotification = rows.some((row) => row.kind === 'notification' && row.active);
  const utilityScreenOpen = showProjects || showProfile;
  const inputPlaceholder = !signedIn
    ? 'Sign in with Google to start building...'
    : isProjectStarting
    ? 'Canvas is starting...'
    : activeProject?.status === 'error'
      ? 'Project needs attention...'
      : singleView
        ? 'Describe the canvas...'
        : 'Describe what should appear on the canvas...';

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

  useEffect(() => {
    if (!signedIn) {
      setProjects([]);
      setActiveID('');
      setFeed(null);
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
    const resize = () => setBasicChatHeight((height) => clampBasicChatHeight(height));
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
    if (!activeProject?.id || !activePreviewURL || activeProject.status === 'error' || activeProject.status === 'deleting') {
      setPreviewStatus(null);
      return;
    }
    let cancelled = false;
    const load = () => api<PreviewStatus>(`/api/projects/${activeProject.id}/preview-status`)
      .then((status) => {
        if (cancelled) return;
        setPreviewStatus(status);
        if (!status.ready) setIframeLoaded(false);
        if (status.ready && activeProject.status !== 'ready') {
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
    const timer = setInterval(load, previewStatus?.ready && !agentWorking ? 5000 : 1500);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [activeProject?.id, activePreviewURL, activeProject?.status, agentWorking, previewStatus?.ready]);
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
      setDialog({ title: 'Request failed', body: err instanceof Error ? err.message : 'Request failed', confirmLabel: 'Close' });
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
      setMessageSubmitting(false);
      setFeed(await api<Feed>(`/api/projects/${activeProject.id}/feed`));
    } catch (err) {
      setDialog({ title: 'Stop failed', body: err instanceof Error ? err.message : 'Request failed', tone: 'warning', confirmLabel: 'Close' });
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
      setDialog({ title: 'Project failed', body: err instanceof Error ? err.message : 'Request failed', tone: 'warning', confirmLabel: 'Close' });
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
      setDialog({ title: 'Rename failed', body: err instanceof Error ? err.message : 'Request failed', tone: 'warning', confirmLabel: 'Close' });
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
      setDialog({ title: 'Service switch failed', body: err instanceof Error ? err.message : 'Request failed', tone: 'warning', confirmLabel: 'Close' });
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
      setDialog({ title: 'Delete failed', body: err instanceof Error ? err.message : 'Request failed', tone: 'danger', confirmLabel: 'Close' });
    } finally {
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
      aria-label={viewMode === 'split' ? 'Use basic view' : 'Use split view'}
      data-tip={viewMode === 'split' ? 'Use basic view' : 'Use split view'}
    >
      <LayoutPanelLeft size={16} />
    </button>
  );
  const topOpenLink = activeProject?.status === 'ready' && activePreviewURL
    ? <a className="chromeIconButton topOpenLink tooltip tooltipBottom" href={activePreviewURL} target="_blank" aria-label="Open preview" data-tip="Open preview"><ExternalLink size={16} /></a>
    : null;
  const expandBasicChat = () => {
    setBasicChatCollapsed(false);
  };
  const openProjectsPanel = () => {
    if (!signedIn) return;
    setShowProjects((open) => !open);
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
  const builderChrome = (
    <div className="basicChatChrome">
      <button className="brand chatBrand tooltip tooltipBottom" onClick={() => nav('/')} aria-label="Likeable link stable" data-tip="Link stable">
        <span className="mark small statusMark">L<span className="brandStatusDot" /></span>
      </button>
      {topOpenLink}
      <button className="projectTitleButton tooltip tooltipBottom" onClick={openProjectsPanel} disabled={!signedIn} aria-label="Projects" data-tip={signedIn ? 'Projects' : 'Sign in to create projects'}>
        <span className="projectTitleMain">{activeProject?.title ?? (signedIn ? 'New project' : 'Sign in to build')}</span>
        <span className="projectTitleCount"><FolderOpen size={15} /><span>{signedIn ? projectCapLabel : '-'}</span></span>
      </button>
      {!me.user && (
        <div className="account chatAccount">
          <a className={!googleReady ? 'disabled' : ''} href="/api/auth/google/start">Sign in</a>
          {me.auth?.devAuth && <button onClick={() => fetch('/api/dev/login?email=admin@example.com', { method: 'POST' }).then(() => location.reload())}>Dev</button>}
        </div>
      )}
      <nav className="chatNav">
        {activeProject?.services && activeProject.services.length > 1 && (
          <div className="chromePill serviceSelector" aria-label="Preview service">
            {activeProject.services.map((service) => (
              <button
                key={service.name}
                className={selectedService?.name === service.name ? 'chromeIconButton selected serviceButton tooltip tooltipBottom' : 'chromeIconButton serviceButton tooltip tooltipBottom'}
                onClick={() => void selectService(service)}
                disabled={!signedIn || busy}
                aria-label={`Show ${service.name}`}
                data-tip={`Show ${service.name}`}
              >
                {service.name.slice(0, 2).toUpperCase()}
              </button>
            ))}
          </div>
        )}
        <div className="chromePill identityPill">
          {agentWorking && (
            <>
              <button className="chromeIconButton stopAgentButton tooltip tooltipBottom" onClick={interruptAgent} disabled={busy} aria-label="Stop agent" data-tip="Stop agent"><CircleStop size={16} /></button>
              <button
                className="chromeIconButton busyPolicyButton tooltip tooltipBottom"
                onClick={() => setBusyPolicy((policy) => policy === 'queue' ? 'steer' : 'queue')}
                aria-label={busyPolicy === 'queue' ? 'Queue new messages' : 'Steer current run'}
                data-tip={busyPolicy === 'queue' ? 'Queue new messages' : 'Steer current run'}
              >
                {busyPolicy === 'queue' ? 'Q' : 'S'}
              </button>
            </>
          )}
          {messageQuota && (
            <span className="messageQuotaBadge tooltip tooltipBottom" data-tip={messageQuotaTooltip} aria-label="Messages left today">
              {messageQuotaLabel}
            </span>
          )}
          <button className={showProfile ? 'chromeIconButton selected tooltip tooltipBottom' : 'chromeIconButton tooltip tooltipBottom'} onClick={showProfile ? closeProfilePanel : openProfilePanel} disabled={!signedIn} aria-label="Profile" data-tip={signedIn ? 'Profile' : 'Sign in to open profile'}><UserRound size={16} /></button>
          {me.isAdmin && <button className="chromeIconButton tooltip tooltipBottom" onClick={() => nav('/admin')} aria-label="Admin" data-tip="Admin"><Settings size={16} /></button>}
        </div>
        <div className="chromePill">
          {modeToggle}
          {viewMode === 'overlay' && <button className="chromeIconButton tooltip tooltipBottom" onClick={() => setBasicChatCollapsed(true)} aria-label="Collapse chat" data-tip="Collapse chat"><Minimize2 size={16} /></button>}
        </div>
      </nav>
    </div>
  );

  const chat = (
    <section className={`chatPane ${draggingFiles ? 'dragActive' : ''} ${utilityScreenOpen ? 'screenOpen' : ''}`} {...chatDragHandlers}>
      <a className="poweredBy" href="https://fibe.gg" target="_blank" rel="noopener noreferrer">
        Powered by <span>fibe.gg</span>
      </a>
      {builderChrome}
      {showProjects && <ProjectList projects={projects} activeID={activeID} projectCap={projectCap} busy={busy} onSelect={(id) => { setActiveID(id); setShowProjects(false); }} onNew={() => setConfirmNewProject(true)} onRename={renameProject} onDelete={setDeleteTarget} onClose={() => setShowProjects(false)} />}
      {showProfile && <ProfilePanel me={me} onClose={closeProfilePanel} />}
      {!utilityScreenOpen && (
        <>
          <div className="messages" ref={messagesRef}>
            {rows.map((row) => row.kind === 'notification'
              ? <AgentNotificationRow key={row.id} body={row.body || agentWorkingLabel} active={row.active} />
              : <UserMessageRow key={row.id} row={row} />
            )}
            {agentWorking && !hasActiveNotification && <AgentNotificationRow body={agentWorkingLabel} active />}
          </div>
          {draggingFiles && <div className="dropOverlay"><Paperclip size={24} /> Drop files to attach</div>}
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
                    <button onClick={() => removeAttachment(attachment.id)} aria-label={`Remove ${attachment.file.name}`}><X size={13} /></button>
                  </span>
                ))}
              </div>
            )}
            <button className="attachButton" type="button" onClick={() => fileInputRef.current?.click()} disabled={!signedIn || attachments.length >= MAX_ATTACHMENTS} aria-label="Attach files">
              <Paperclip size={20} />
            </button>
            <textarea ref={textareaRef} value={prompt} onChange={(e) => setPrompt(e.target.value)} onKeyDown={handleComposerKeyDown} placeholder={inputPlaceholder} rows={1} disabled={!signedIn} />
            <button className={`sendButton ${messageSubmitting ? 'working' : ''}`} disabled={!canSend} onClick={createOrSend}>
              {messageSubmitting ? <Loader2 className="spinIcon" size={22} /> : <Send size={22} />}
            </button>
          </div>
        </>
      )}
    </section>
  );
  const minimizedChatBar = (
    <button className={`minimizedChatBar ${agentWorking ? 'working' : ''}`} onClick={expandBasicChat} aria-label="Expand chat" {...chatDragHandlers}>
      <span className="mark small statusMark">L<span className="brandStatusDot" /></span>
    </button>
  );

  const previewTitle = activeProject?.status === 'launching' ? 'Starting canvas' : 'Preparing canvas';
  const previewBody = activeProject?.status === 'launching'
    ? 'Likeable is preparing the project workspace. The canvas will appear when it is ready.'
    : 'Likeable is creating a private project workspace from the default starter.';
  const connectingCanvasBody = previewReady
    ? 'The canvas responded. Opening the preview.'
    : 'The canvas route is warming up. Likeable will open it automatically when it is ready.';
  const preview = (
    <section className="previewPane">
      {activeProject?.status === 'error' ? (
        <CanvasLoader title="Canvas launch failed" body={projectLaunchErrorMessage(activeProject.errorMessage)} tone="error" />
      ) : activePreviewURL && previewReady ? (
        <>
          <iframe
            title="preview"
            src={activePreviewURL}
            className={previewReady && iframeLoaded ? 'loaded' : ''}
            onLoad={() => {
              if (previewReady) setIframeLoaded(true);
            }}
          />
          {(!previewReady || !iframeLoaded) && <CanvasLoader title="Connecting canvas" body={connectingCanvasBody} />}
        </>
      ) : isProjectStarting ? (
        <CanvasLoader title={previewTitle} body={previewBody} />
      ) : activeProject?.status === 'ready' && activePreviewURL ? (
        <>
          <iframe
            title="preview"
            src="about:blank"
            className=""
            onLoad={() => undefined}
          />
          <CanvasLoader title="Connecting canvas" body={connectingCanvasBody} />
        </>
      ) : <EmptyCanvas />}
      {viewMode === 'split' && <div className={`canvasStatus ${agentWorking ? 'working' : ''}`}><span /> {canvasStatusLabel}</div>}
      {viewMode === 'overlay' && (
        <div
          className={`overlayChat ${basicChatCollapsed ? 'collapsed minimized' : ''}`}
          style={!basicChatCollapsed ? ({ '--basic-chat-height': `${basicChatHeight}px` } as React.CSSProperties) : undefined}
        >
          {!basicChatCollapsed && <div className="chatResizeHandle" aria-label="Resize chat" onPointerDown={startBasicChatResize} />}
          {basicChatCollapsed ? minimizedChatBar : chat}
        </div>
      )}
      {confirmNewProject && <ConfirmNewProject projectCap={projectCap} projectCount={projects.length} busy={busy} onCancel={() => setConfirmNewProject(false)} onConfirm={createProject} />}
      {deleteTarget && <ConfirmDeleteProject project={deleteTarget} busy={busy} onCancel={() => setDeleteTarget(null)} onConfirm={deleteProject} />}
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

function mergeFeedSnapshot(current: Feed | null, next: Feed): Feed {
  if (!current || current.project.id !== next.project.id) return next;
  if (next.live?.isProcessing || next.live?.streamText) return next;
  const currentLive = current.live;
  if (!currentLive?.isProcessing || !currentLive.streamText) return next;
  if (feedHasDurableNotifications(next)) return next;

  return {
    ...next,
    messages: next.messages?.length ? next.messages : current.messages,
    activity: next.activity?.length ? next.activity : current.activity,
    live: {
      ...currentLive,
      conversationId: next.live?.conversationId ?? currentLive.conversationId,
      isProcessing: typeof next.live?.isProcessing === 'boolean' ? next.live.isProcessing : currentLive.isProcessing,
      queuedTurns: typeof next.live?.queuedTurns === 'number' ? next.live.queuedTurns : currentLive.queuedTurns,
      startedAt: currentLive.startedAt ?? next.live?.startedAt
    }
  };
}

function feedHasDurableNotifications(feed: Feed): boolean {
  return feedRows({ ...feed, live: null }).some((row) => row.kind === 'notification');
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

createRoot(document.getElementById('root')!).render(<App />);
