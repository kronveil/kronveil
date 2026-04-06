import { useState, useCallback, useMemo } from 'react';
import { useWebSocket } from './useWebSocket';
import { getWebSocketUrl } from '../services/api';

export interface TelemetryEvent {
  id: string;
  timestamp: string;
  type: 'anomaly' | 'incident' | 'remediation' | 'info';
  title: string;
  description: string;
  severity?: 'critical' | 'high' | 'medium' | 'low';
  source?: string;
}

const MAX_BUFFER_SIZE = 100;

export interface UseEventStreamReturn {
  events: TelemetryEvent[];
  incidents: TelemetryEvent[];
  anomalies: TelemetryEvent[];
  connected: boolean;
  reconnect: () => void;
}

export function useEventStream(): UseEventStreamReturn {
  const [events, setEvents] = useState<TelemetryEvent[]>([]);

  const handleMessage = useCallback((data: TelemetryEvent | TelemetryEvent[]) => {
    setEvents((prev) => {
      const incoming = Array.isArray(data) ? data : [data];
      const updated = [...incoming, ...prev];
      // Keep only the most recent events
      return updated.slice(0, MAX_BUFFER_SIZE);
    });
  }, []);

  const wsUrl = getWebSocketUrl('/api/v1/ws/events');

  const { connected, reconnect } = useWebSocket<TelemetryEvent | TelemetryEvent[]>(
    wsUrl,
    handleMessage,
    { reconnectInterval: 1000, maxRetries: 10 },
  );

  const incidents = useMemo(
    () => events.filter((e) => e.type === 'incident'),
    [events],
  );

  const anomalies = useMemo(
    () => events.filter((e) => e.type === 'anomaly'),
    [events],
  );

  return { events, incidents, anomalies, connected, reconnect };
}
