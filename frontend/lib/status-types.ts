// Status page API types — separated from api-types.ts to respect ~900 line limit.

export interface APIStatusService {
  name: string;
  description: string;
  status: 'operational' | 'degraded' | 'outage';
  uptime: string;
  latency_ms: number | null;
  last_checked: string | null;
}

export interface APIStatusCategory {
  category: string;
  items: APIStatusService[];
}

export interface APIStatusSummary {
  uptime_30d: number | null;
  avg_response_time_ms: number | null;
  service_count: number;
  last_checked: string | null;
}

export interface APIStatusUptimeDay {
  date: string;
  status: 'operational' | 'degraded' | 'outage';
}

export interface APIStatusIncidentUpdate {
  time: string;
  message: string;
  status: string;
}

export interface APIStatusIncident {
  id: string;
  title: string;
  status: 'investigating' | 'identified' | 'monitoring' | 'resolved';
  severity: 'minor' | 'major' | 'critical';
  created_at: string;
  updated_at: string;
  updates: APIStatusIncidentUpdate[];
}

export interface APIStatusData {
  overall_status: 'operational' | 'degraded' | 'outage';
  services: APIStatusCategory[];
  summary: APIStatusSummary;
  uptime_history: APIStatusUptimeDay[];
  incidents: APIStatusIncident[];
}

export interface APIStatusResponse {
  data: APIStatusData;
}
