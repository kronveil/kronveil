import type { Incident, Anomaly, Collector, Policy, MetricsSummary, HealthStatus } from '../types';

const API_BASE = '/api/v1';

async function fetchJSON<T>(path: string): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`);
  if (!res.ok) {
    throw new Error(`API error: ${res.status} ${res.statusText}`);
  }
  return res.json();
}

export const api = {
  health: () => fetchJSON<HealthStatus>('/health'),
  metrics: () => fetchJSON<MetricsSummary>('/metrics/summary'),
  incidents: {
    list: (status?: string) =>
      fetchJSON<Incident[]>(`/incidents${status ? `?status=${status}` : ''}`),
    get: (id: string) => fetchJSON<Incident>(`/incidents/${id}`),
    acknowledge: (id: string) =>
      fetch(`${API_BASE}/incidents/${id}/acknowledge`, { method: 'POST' }),
    resolve: (id: string) =>
      fetch(`${API_BASE}/incidents/${id}/resolve`, { method: 'POST' }),
  },
  anomalies: {
    list: () => fetchJSON<Anomaly[]>('/anomalies'),
  },
  collectors: {
    list: () => fetchJSON<Collector[]>('/collectors'),
  },
  policies: {
    list: () => fetchJSON<Policy[]>('/policies'),
    create: (policy: Partial<Policy>) =>
      fetch(`${API_BASE}/policies`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(policy),
      }),
    delete: (id: string) =>
      fetch(`${API_BASE}/policies/${id}`, { method: 'DELETE' }),
  },
};
