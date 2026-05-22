import type { AgentPoolStatus, PoolRow } from './domain';

const POOL_STATUSES: AgentPoolStatus[] = ['active', 'draining', 'retiring', 'retired'];

function poolRowID() {
  return globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random()}`;
}

export function makePoolRow(values: Partial<PoolRow> = {}): PoolRow {
  return {
    id: values.id ?? poolRowID(),
    label: values.label ?? '',
    agentId: values.agentId ?? '',
    serverId: values.serverId ?? '',
    status: values.status ?? 'active',
    capacity: values.capacity ?? ''
  };
}

function parseStatus(value: unknown): AgentPoolStatus {
  if (typeof value !== 'string') return 'active';
  const clean = value.trim().toLowerCase();
  return POOL_STATUSES.includes(clean as AgentPoolStatus) ? clean as AgentPoolStatus : 'active';
}

export function parsePoolRows(raw: string): PoolRow[] {
  if (!raw.trim()) return [];
  try {
    const parsed: unknown = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed.flatMap((item) => {
      if (!item || typeof item !== 'object') return [];
      const record = item as Record<string, unknown>;
      const capacity = parseCapacity(record.capacity ?? record.max_projects ?? record.maxProjects);
      return [makePoolRow({
        label: typeof record.label === 'string' ? record.label : '',
        agentId: typeof record.agent_id === 'string' ? record.agent_id : typeof record.agentId === 'string' ? record.agentId : '',
        serverId: typeof record.server_id === 'string' ? record.server_id : typeof record.marquee_id === 'string' ? record.marquee_id : typeof record.marqueeId === 'string' ? record.marqueeId : '',
        status: parseStatus(record.status ?? record.state),
        capacity
      })];
    });
  } catch {
    return [];
  }
}

export function cleanPoolRows(rows: PoolRow[]) {
  return rows.map((row) => {
    const capacity = parseCapacity(row.capacity);
    return {
      label: row.label.trim(),
      agent_id: row.agentId.trim(),
      server_id: row.serverId.trim(),
      status: row.status,
      ...(capacity ? { capacity: Number(capacity) } : {})
    };
  }).filter((row) => row.label || row.agent_id || row.server_id);
}

function parseCapacity(value: unknown): string {
  if (typeof value === 'number' && Number.isFinite(value) && value > 0) return String(Math.floor(value));
  if (typeof value !== 'string') return '';
  const clean = value.trim();
  if (!/^\d+$/.test(clean)) return '';
  const numeric = Number(clean);
  return numeric > 0 ? String(Math.floor(numeric)) : '';
}
