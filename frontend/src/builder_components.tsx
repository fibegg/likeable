import { useState } from 'react';
import { Check, Loader2, Paperclip, Pencil, Plus, Trash2, X } from 'lucide-react';
import type { AppDialogConfig, MessageAttachment, Project, UserFeedRow } from './domain';
import { formatMessageTime } from './format';

export function ProjectList({ projects, activeID, projectCap, busy, onSelect, onNew, onRename, onDelete, onClose }: { projects: Project[]; activeID: string; projectCap: number | null; busy: boolean; onSelect: (id: string) => void; onNew: () => void; onRename: (project: Project, title: string) => Promise<void>; onDelete: (project: Project) => void; onClose: () => void }) {
  const [editingID, setEditingID] = useState('');
  const [draftTitle, setDraftTitle] = useState('');
  const projectCountLabel = projectCap == null
    ? `${projects.length} ${projects.length === 1 ? 'project' : 'projects'}`
    : `${projects.length}/${projectCap} projects`;
  const startEdit = (project: Project) => {
    setEditingID(project.id);
    setDraftTitle(project.title);
  };
  const cancelEdit = () => {
    setEditingID('');
    setDraftTitle('');
  };
  const saveTitle = async (project: Project) => {
    const title = draftTitle.trim();
    if (!title || title === project.title) {
      cancelEdit();
      return;
    }
    await onRename(project, title);
    cancelEdit();
  };
  return (
    <div className="projectList">
      <div className="inlinePanelHeader projectPanelHeader">
        <div>
          <span className="eyebrow">Projects</span>
          <strong>{projectCountLabel}</strong>
        </div>
        <button className="projectDelete" onClick={onClose} aria-label="Close projects"><X size={15} /></button>
      </div>
      <div className="projectRows">
        {projects.map((project) => (
          <div key={project.id} className={`projectRow ${project.id === activeID ? 'selected' : ''}`}>
            {editingID === project.id ? (
              <form className="projectTitleEdit" onSubmit={(event) => { event.preventDefault(); void saveTitle(project); }}>
                <input
                  className="projectEditInput"
                  value={draftTitle}
                  autoFocus
                  maxLength={80}
                  onChange={(event) => setDraftTitle(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === 'Escape') {
                      event.preventDefault();
                      cancelEdit();
                    }
                  }}
                  aria-label="Project name"
                />
                <button className="projectRowIcon" type="submit" disabled={busy || !draftTitle.trim()} aria-label="Save project name"><Check size={15} /></button>
                <button className="projectRowIcon" type="button" onClick={cancelEdit} aria-label="Cancel rename"><X size={15} /></button>
              </form>
            ) : (
              <>
                <button className="projectSelect" onClick={() => onSelect(project.id)}>
                  <span>{project.title}</span>
                  <em>{project.status}</em>
                </button>
                <div className="projectRowActions">
                  <button className="projectRowIcon" onClick={() => startEdit(project)} aria-label={`Rename ${project.title}`}><Pencil size={14} /></button>
                  <button className="projectDelete" onClick={() => onDelete(project)} aria-label={`Delete ${project.title}`}><Trash2 size={15} /></button>
                </div>
              </>
            )}
          </div>
        ))}
      </div>
      <button className="newProjectRow" onClick={onNew}><Plus size={15} /> New project</button>
    </div>
  );
}

export function UserMessageRow({ row }: { row: UserFeedRow }) {
  const time = formatMessageTime(row.time);
  const body = row.body.trim() || (row.attachments.length > 0 ? 'Attached files' : '');
  return (
    <article className="messageCard">
      <div className="messageMeta">
        <span>Sent</span>
        {time && <time dateTime={row.time}>{time}</time>}
      </div>
      {body && <div className="messageBody">{body}</div>}
      {row.attachments.length > 0 && <AttachmentGrid attachments={row.attachments} />}
    </article>
  );
}

function AttachmentGrid({ attachments }: { attachments: MessageAttachment[] }) {
  return (
    <div className="messageAttachments" aria-label="Attachments">
      {attachments.map((attachment) => {
        const image = Boolean(attachment.url && attachment.contentType?.startsWith('image/'));
        return (
          <a className={`messageAttachment ${image ? 'image' : ''}`} key={attachment.id} href={attachment.url || undefined} target={attachment.url ? '_blank' : undefined} rel="noreferrer">
            {image ? <img src={attachment.url} alt={attachment.filename} loading="lazy" /> : <Paperclip size={15} />}
            <span>{attachment.filename}</span>
          </a>
        );
      })}
    </div>
  );
}

export function AgentNotificationRow({ body, active }: { body: string; active?: boolean }) {
  const text = body.trim() || (active ? 'Receiving update' : 'Canvas updated');
  return (
    <div className={`notificationRow ${active ? 'active' : ''}`} aria-live="polite">
      <div className="notificationBubble">
        {active ? <Loader2 className="notificationSpinner" size={14} /> : <Check className="notificationDone" size={14} />}
        <span className="notificationText">{text}</span>
      </div>
    </div>
  );
}

export function EmptyCanvas() {
  return (
    <div className="emptyPreview">
      <div className="corner tl" />
      <div className="corner tr" />
      <div className="corner bl" />
      <div className="corner br" />
      <div className="stars" />
      <div className="reticle" />
      <div className="emptyCopy">
        <h1>Awaiting transmission</h1>
        <p>The canvas is ready. Describe the scene and the agent will rebuild this view.</p>
      </div>
    </div>
  );
}

export function CanvasLoader({ title, body, tone }: { title: string; body: string; tone?: 'error' }) {
  return (
    <div className={`emptyPreview canvasLoader ${tone === 'error' ? 'error' : ''}`}>
      <div className="corner tl" />
      <div className="corner tr" />
      <div className="corner bl" />
      <div className="corner br" />
      <div className="stars" />
      <div className="loaderRing" />
      <div className="emptyCopy">
        <h1>{title}</h1>
        <p>{body}</p>
      </div>
    </div>
  );
}

export function AppDialog({ dialog, onClose }: { dialog: AppDialogConfig; onClose: () => void }) {
  const [working, setWorking] = useState(false);
  const isConfirm = Boolean(dialog.onConfirm);
  const tone = dialog.tone ?? 'info';
  const confirmClass = tone === 'danger' ? 'dangerButton' : 'primaryButton';
  const confirmLabel = dialog.confirmLabel ?? (isConfirm ? 'Confirm' : 'Close');

  const handleConfirm = async () => {
    if (!dialog.onConfirm) {
      onClose();
      return;
    }
    setWorking(true);
    try {
      await dialog.onConfirm();
      onClose();
    } finally {
      setWorking(false);
    }
  };

  return (
    <div className="modalScrim appDialogScrim" role="presentation">
      <section className={`confirmDialog appDialog ${tone}`} role="dialog" aria-modal="true" aria-labelledby="app-dialog-title">
        <button className="dialogClose" onClick={onClose} aria-label="Close dialog"><X size={16} /></button>
        <span className="eyebrow">{tone}</span>
        <h2 id="app-dialog-title">{dialog.title}</h2>
        <p>{dialog.body}</p>
        <div className="dialogActions appDialogActions">
          {isConfirm && (
            <button className="ghostButton" disabled={working} onClick={onClose}>
              {dialog.cancelLabel ?? 'Cancel'}
            </button>
          )}
          <button className={confirmClass} disabled={working} onClick={() => void handleConfirm()}>
            {working ? <Loader2 className="spinIcon" size={15} /> : null}
            {confirmLabel}
          </button>
        </div>
      </section>
    </div>
  );
}

export function ConfirmNewProject({ projectCap, projectCount, busy, onCancel, onConfirm }: { projectCap: number | null; projectCount: number; busy: boolean; onCancel: () => void; onConfirm: (title?: string) => void }) {
  const [title, setTitle] = useState('');
  const capReached = projectCap != null && projectCount >= projectCap;
  return (
    <div className="modalScrim">
      <section className="confirmDialog">
        <button className="dialogClose" onClick={onCancel}><X size={16} /></button>
        <span className="eyebrow">New project</span>
        <h2>Create another project?</h2>
        <p>Likeable will immediately start a fresh private workspace using the default starter.</p>
        <label className="dialogField">
          <span>Project name</span>
          <input className="dialogInput" value={title} maxLength={80} onChange={(event) => setTitle(event.target.value)} placeholder={`New playground ${projectCount + 1}`} />
        </label>
        {projectCap != null && <p className="quotaLine">Projects: {projectCount}/{projectCap}</p>}
        <div className="dialogActions">
          <button className="ghostButton" onClick={onCancel}>Cancel</button>
          <button className="primaryButton" disabled={busy || capReached} onClick={() => onConfirm(title)}>{capReached ? 'Cap reached' : 'Create'}</button>
        </div>
      </section>
    </div>
  );
}

export function ConfirmDeleteProject({ project, busy, onCancel, onConfirm }: { project: Project; busy: boolean; onCancel: () => void; onConfirm: () => void }) {
  return (
    <div className="modalScrim">
      <section className="confirmDialog">
        <button className="dialogClose" onClick={onCancel}><X size={16} /></button>
        <span className="eyebrow">Delete project</span>
        <h2>Delete this project?</h2>
        <p>{project.title} will be removed from Likeable, including its workspace and private source archive.</p>
        <div className="dialogActions">
          <button className="ghostButton" onClick={onCancel}>Cancel</button>
          <button className="dangerButton" disabled={busy} onClick={onConfirm}>Delete</button>
        </div>
      </section>
    </div>
  );
}

export function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="customerMetric">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

export function DeleteAllAccountDialog({ email, busy, onCancel, onConfirm }: { email: string; busy: boolean; onCancel: () => void; onConfirm: (email: string) => void }) {
  const [typedEmail, setTypedEmail] = useState('');
  const confirmed = typedEmail.trim().toLowerCase() === email.trim().toLowerCase() && email.trim() !== '';
  return (
    <div className="modalScrim appDialogScrim" role="presentation">
      <section className="confirmDialog appDialog danger deleteAllDialog" role="dialog" aria-modal="true" aria-labelledby="delete-all-title">
        <button className="dialogClose" disabled={busy} onClick={onCancel} aria-label="Close dialog"><X size={16} /></button>
        <span className="eyebrow">danger zone</span>
        <h2 id="delete-all-title">Delete everything?</h2>
        <p>This permanently deletes every Likeable record for this account and first attempts to remove the related workspaces, conversations, source data, and private repositories.</p>
        <p>Type <strong>{email}</strong> to confirm.</p>
        <input
          className="dangerConfirmInput"
          autoFocus
          value={typedEmail}
          onChange={(event) => setTypedEmail(event.target.value)}
          placeholder={email}
          disabled={busy}
        />
        <div className="dialogActions appDialogActions">
          <button className="ghostButton" disabled={busy} onClick={onCancel}>Cancel</button>
          <button className="dangerButton" disabled={!confirmed || busy} onClick={() => onConfirm(typedEmail)}>
            {busy ? <Loader2 className="spinIcon" size={15} /> : <Trash2 size={15} />}
            DELETE ALL
          </button>
        </div>
      </section>
    </div>
  );
}
