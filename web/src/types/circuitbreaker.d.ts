interface Info {
  version: string;
  nodeKey: string;
  nodeAlias: string;
  nodeVersion: string;
}

interface Counter {
  fail: number;
  success: number;
  reject: number;
}

interface Limit {
  maxHourlyRate: string;
  maxPending: string;
  mode: Mode;
}

interface NodeLimit {
  node: string;
  alias: string;
  limit: Limit;
  counter1h: Counter;
  counter24h: Counter;
  queueLen: number;
  pendingHtlcCount: number;
}

interface Limits {
  limits: NodeLimit[];
  defaultLimit: Limit;
}

type ColumnId =
  | 'alias'
  | 'counter1h.success'
  | 'counter1h.fail'
  | 'counter1h.reject'
  | 'counter24h.success'
  | 'counter24h.fail'
  | 'counter24h.reject'
  | 'node'
  | 'pendingHtlcCount'
  | 'queueLen'
  | 'limit.maxPending'
  | 'limit.maxHourlyRate'
  | 'limit.mode';

type Order = 'asc' | 'desc';
