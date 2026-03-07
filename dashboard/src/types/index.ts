export interface Incident {
  id: string;
  title: string;
  description: string;
  severity: 'critical' | 'high' | 'medium' | 'low';
  status: 'active' | 'acknowledged' | 'resolved';
  rootCause: string;
  affectedResources: string[];
  createdAt: string;
  resolvedAt?: string;
  mttr?: number;
  timeline: TimelineEntry[];
}

export interface TimelineEntry {
  timestamp: string;
  action: string;
  details: string;
  actor: 'system' | 'user' | 'ai';
}

export interface Anomaly {
  id: string;
  signal: string;
  score: number;
  timestamp: string;
  predicted: boolean;
  description: string;
  source: string;
  severity: 'critical' | 'high' | 'medium' | 'low';
}

export interface Collector {
  name: string;
  type: string;
  status: 'healthy' | 'degraded' | 'critical' | 'unknown';
  eventsPerSec: number;
  lastEvent: string;
  targets: number;
  errors: number;
}

export interface Policy {
  id: string;
  name: string;
  description: string;
  severity: string;
  enabled: boolean;
  violations: number;
  lastEvaluated: string;
}

export interface MetricsSummary {
  totalEvents: number;
  eventsPerSec: number;
  activeIncidents: number;
  anomaliesDetected: number;
  mttrAvg: number;
  remediationsExecuted: number;
  collectors: number;
  clustersMonitored: number;
}

export interface HealthStatus {
  status: 'healthy' | 'degraded' | 'critical';
  components: ComponentHealth[];
  uptime: number;
}

export interface ComponentHealth {
  name: string;
  status: 'healthy' | 'degraded' | 'critical';
  message: string;
  lastCheck: string;
}
