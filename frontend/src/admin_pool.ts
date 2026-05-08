import type { PoolRow } from './domain';

function poolRowID() {
  return globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random()}`;
}

export function makePoolRow(values: Partial<PoolRow> = {}): PoolRow {
  return {
    id: values.id ?? poolRowID(),
    label: values.label ?? '',
    agentId: values.agentId ?? '',
    serverId: values.serverId ?? ''
  };
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
        serverId: typeof record.server_id === 'string' ? record.server_id : typeof record.marquee_id === 'string' ? record.marquee_id : typeof record.marqueeId === 'string' ? record.marqueeId : ''
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
    server_id: row.serverId.trim()
  })).filter((row) => row.label || row.agent_id || row.server_id);
}
