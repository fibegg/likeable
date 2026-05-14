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
    status: values.status ?? 'active'
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
      return [makePoolRow({
	        label: typeof record.label === 'string' ? record.label : '',
	        agentId: typeof record.agent_id === 'string' ? record.agent_id : typeof record.agentId === 'string' ? record.agentId : '',
	        serverId: typeof record.server_id === 'string' ? record.server_id : typeof record.marquee_id === 'string' ? record.marquee_id : typeof record.marqueeId === 'string' ? record.marqueeId : '',
	        status: parseStatus(record.status ?? record.state)
	      })];
	    });
  } catch {
    return [];
  }
}

export function cleanPoolRows(rows: PoolRow[]) {
  return rows.map((row) => ({
    label: row.label.trim(),
    agent_id: row.agentId.trim(),
    server_id: row.serverId.trim(),
    status: row.status
  })).filter((row) => row.label || row.agent_id || row.server_id);
}
