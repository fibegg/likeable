import { useEffect, useMemo, useRef, useState } from 'react';
import { ExternalLink, FolderOpen, GitBranch, Loader2, Send, Trash2, Wallet, X } from 'lucide-react';
import { api } from './api';
import { DeleteAllAccountDialog } from './builder_components';
import type { Me, ProjectArchive, UserNotice } from './domain';
import { formatMessageTime, formatResetCountdown, formatShortDate, userInitials } from './format';

export function ProfilePanel({ me, onClose }: { me: Me; onClose: () => void }) {
  const [messages, setMessages] = useState<UserNotice[]>([]);
  const [archives, setArchives] = useState<ProjectArchive[]>([]);
  const [supportBody, setSupportBody] = useState('');
  const [busyPack, setBusyPack] = useState<number | null>(null);
  const [sendingSupport, setSendingSupport] = useState(false);
  const [profileError, setProfileError] = useState('');
  const [deleteAllOpen, setDeleteAllOpen] = useState(false);
  const [deletingAll, setDeletingAll] = useState(false);
  const profileMessagesRef = useRef<HTMLDivElement | null>(null);
  const orderedMessages = useMemo(() => [...messages].sort((left, right) => {
    const leftTime = Date.parse(left.createdAt);
    const rightTime = Date.parse(right.createdAt);
    if (Number.isNaN(leftTime) || Number.isNaN(rightTime)) return 0;
    return leftTime - rightTime;
  }), [messages]);
  const loadMessages = async () => {
    try {
      const res = await api<{ messages: UserNotice[] }>('/api/messages?limit=80');
      setMessages(res.messages ?? []);
    } catch (err) {
      setProfileError(err instanceof Error ? err.message : 'Could not load messages');
    }
  };
  const loadArchives = async () => {
    try {
      const res = await api<{ archives: ProjectArchive[] }>('/api/profile/archives');
      setArchives(res.archives ?? []);
    } catch (err) {
      setProfileError(err instanceof Error ? err.message : 'Could not load archives');
    }
  };
  useEffect(() => {
    void loadMessages();
    void loadArchives();
  }, []);
  useEffect(() => {
    const container = profileMessagesRef.current;
    if (!container) return;
    container.scrollTop = container.scrollHeight;
  }, [orderedMessages.length]);
  const checkout = async (pack: number) => {
    setBusyPack(pack);
    setProfileError('');
    try {
      const res = await api<{ url: string }>('/api/billing/checkout', {
        method: 'POST',
        body: JSON.stringify({ pack })
      });
      location.href = res.url;
    } catch (err) {
      setProfileError(err instanceof Error ? err.message : 'Checkout failed');
      setBusyPack(null);
    }
  };
  const checkoutProjectSlot = async () => {
    setBusyPack(-1);
    setProfileError('');
    try {
      const res = await api<{ url: string }>('/api/billing/checkout', {
        method: 'POST',
        body: JSON.stringify({ product: 'project_quota', slots: 1 })
      });
      location.href = res.url;
    } catch (err) {
      setProfileError(err instanceof Error ? err.message : 'Checkout failed');
      setBusyPack(null);
    }
  };
  const sendSupportMessage = async () => {
    if (!supportBody.trim()) return;
    setSendingSupport(true);
    setProfileError('');
    try {
      await api('/api/messages', {
        method: 'POST',
        body: JSON.stringify({ body: supportBody.trim() })
      });
      setSupportBody('');
      await loadMessages();
    } catch (err) {
      setProfileError(err instanceof Error ? err.message : 'Message failed');
    } finally {
      setSendingSupport(false);
    }
  };
  const handleSupportKeyDown = (event: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key !== 'Enter' || event.shiftKey || event.nativeEvent.isComposing) return;
    event.preventDefault();
    if (supportBody.trim() && !sendingSupport) void sendSupportMessage();
  };
  const deleteAll = async (email: string) => {
    setDeletingAll(true);
    setProfileError('');
    try {
      await api('/api/profile/delete-all', {
        method: 'POST',
        body: JSON.stringify({ email })
      });
      location.href = '/';
    } catch (err) {
      setProfileError(err instanceof Error ? err.message : 'Delete all failed');
      setDeletingAll(false);
    }
  };
  const displayName = me.user?.name || me.user?.email || 'Signed in';
  const quota = me.messageQuota;
  const projectQuota = me.projectQuota;
  const quotaResetLabel = quota ? formatResetCountdown(quota.resetsAt) : 'daily';
  return (
    <section className="inlinePanel profileInline">
      <div className="inlinePanelHeader">
        <div>
          <span className="eyebrow">Profile</span>
          <strong>{me.user?.email}</strong>
        </div>
        <button className="projectDelete" onClick={onClose} aria-label="Close profile"><X size={15} /></button>
      </div>
      {profileError && <div className="adminError inlineAdminError">{profileError}</div>}
      <div className="profileGrid">
        <div className="profileCard profileIdentityCard">
          <div className="profileAvatar">{userInitials(me.user)}</div>
          <div>
            <span className="profileLabel">Signed in as</span>
            <strong>{displayName}</strong>
            <em>{me.user?.email}</em>
          </div>
          {me.isAdmin && <span className="profileBadge">Admin</span>}
        </div>
        <div className="profileCard profileActionCard">
          <div>
            <span className="profileLabel">GitHub</span>
            <strong>Repository export</strong>
          </div>
          <a className="ghostButton" href="/api/profile/github/start"><GitBranch size={18} /> Connect</a>
        </div>
        <div className="profileCard profileActionCard">
          <div>
            <span className="profileLabel">Messages</span>
            <strong>{quota ? `${quota.remaining}/${quota.limit} free today` : 'Daily free quota'}</strong>
            <em>{quota?.paidRemaining ?? 0} paid credits · resets in {quotaResetLabel} · {quota?.lifetimeUsed ?? 0} lifetime sent</em>
          </div>
          <div className="packButtons">
            {[10, 100, 1000].map((pack) => (
              <button className="primaryButton" key={pack} disabled={busyPack != null} onClick={() => void checkout(pack)}>
                {busyPack === pack ? <Loader2 size={16} className="spin" /> : <Wallet size={16} />} {pack}
              </button>
            ))}
          </div>
        </div>
        <div className="profileCard profileActionCard">
          <div>
            <span className="profileLabel">Projects</span>
            <strong>{projectQuota ? `${projectQuota.used}/${projectQuota.limit} project slots` : 'Project quota'}</strong>
            <em>{projectQuota?.paidSlots ?? 0} paid monthly slots{projectQuota?.nextExpiresAt ? ` · next reset ${formatShortDate(projectQuota.nextExpiresAt)}` : ''}</em>
          </div>
          <button className="primaryButton" disabled={busyPack != null} onClick={() => void checkoutProjectSlot()}>
            {busyPack === -1 ? <Loader2 size={16} className="spin" /> : <FolderOpen size={16} />} +1 slot
          </button>
        </div>
        <div className="profileCard profileActionCard profileDangerCard">
          <div>
            <span className="profileLabel">Danger zone</span>
            <strong>Delete all account data</strong>
            <em>Removes projects, workspaces, messages, OAuth connections, payments, sessions, and this profile.</em>
          </div>
          <button className="dangerButton" disabled={deletingAll} onClick={() => setDeleteAllOpen(true)}>
            {deletingAll ? <Loader2 size={16} className="spin" /> : <Trash2 size={16} />} DELETE ALL
          </button>
        </div>
      </div>
      {archives.length > 0 && (
        <div className="profileArchives">
          <div className="adminCardHeader compactHeader">
            <h3>Archived Projects</h3>
            <p>Archives are kept for 90 days after quota cleanup. Download them here if GitHub export was unavailable.</p>
          </div>
          <div className="archiveList">
            {archives.map((archive) => (
              <div className={`archiveRow ${archive.status}`} key={archive.id}>
                <div>
                  <strong>{archive.projectTitle}</strong>
                  <em>{archive.status}{archive.expiresAt ? ` · expires ${formatShortDate(archive.expiresAt)}` : ''}</em>
                  {archive.error && <small>{archive.error}</small>}
                </div>
                {archive.githubRepoUrl && <a className="ghostButton" href={archive.githubRepoUrl} target="_blank" rel="noreferrer"><ExternalLink size={15} /> GitHub</a>}
                {archive.downloadUrl && <a className="primaryButton" href={archive.downloadUrl}><FolderOpen size={15} /> Zip</a>}
              </div>
            ))}
          </div>
        </div>
      )}
      <div className="profileMailbox">
        <div className="adminCardHeader compactHeader">
          <h3>Messages</h3>
          <p>System messages from Likeable and your messages to admin support stay here.</p>
        </div>
        <div className="profileMessageList" ref={profileMessagesRef}>
          {orderedMessages.map((message) => (
            <div className={`profileMessage ${message.sender} ${message.severity}`} key={message.id}>
              <div className="messageMeta">
                <span>{message.sender === 'user' ? 'You' : 'System'}</span>
                <time dateTime={message.createdAt}>{formatMessageTime(message.createdAt)}</time>
              </div>
              <p>{message.body}</p>
              {message.dismissedAt && <em>Dismissed</em>}
            </div>
          ))}
          {messages.length === 0 && <div className="emptyPool">No system messages yet.</div>}
        </div>
        <div className="supportComposer">
          <textarea className="adminTextarea compactTextarea" rows={1} value={supportBody} onChange={(event) => setSupportBody(event.target.value)} onKeyDown={handleSupportKeyDown} placeholder="Message admin support..." />
          <button className="primaryButton" disabled={!supportBody.trim() || sendingSupport} onClick={() => void sendSupportMessage()} aria-label="Send support message">
            {sendingSupport ? <Loader2 size={16} className="spin" /> : <Send size={16} />} Send
          </button>
        </div>
      </div>
      {deleteAllOpen && (
        <DeleteAllAccountDialog
          email={me.user?.email ?? ''}
          busy={deletingAll}
          onCancel={() => setDeleteAllOpen(false)}
          onConfirm={(email) => void deleteAll(email)}
        />
      )}
    </section>
  );
}
