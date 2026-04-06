import { useState, useEffect, useRef, useCallback } from 'react';

export interface UseWebSocketOptions {
  reconnectInterval?: number;
  maxRetries?: number;
}

export interface UseWebSocketReturn<T = unknown> {
  connected: boolean;
  lastMessage: T | null;
  send: (data: string | object) => void;
  reconnect: () => void;
}

export function useWebSocket<T = unknown>(
  url: string | null,
  onMessage?: (data: T) => void,
  options: UseWebSocketOptions = {},
): UseWebSocketReturn<T> {
  const { reconnectInterval = 1000, maxRetries = 10 } = options;

  const [connected, setConnected] = useState(false);
  const [lastMessage, setLastMessage] = useState<T | null>(null);

  const wsRef = useRef<WebSocket | null>(null);
  const retriesRef = useRef(0);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const onMessageRef = useRef(onMessage);
  const unmountedRef = useRef(false);

  // Keep callback ref current without triggering reconnects
  onMessageRef.current = onMessage;

  const clearReconnectTimer = useCallback(() => {
    if (reconnectTimerRef.current !== null) {
      clearTimeout(reconnectTimerRef.current);
      reconnectTimerRef.current = null;
    }
  }, []);

  const connect = useCallback(() => {
    if (!url || unmountedRef.current) return;

    // Clean up any existing connection
    if (wsRef.current) {
      wsRef.current.onopen = null;
      wsRef.current.onclose = null;
      wsRef.current.onerror = null;
      wsRef.current.onmessage = null;
      wsRef.current.close();
    }

    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onopen = () => {
      if (unmountedRef.current) return;
      setConnected(true);
      retriesRef.current = 0;
    };

    ws.onmessage = (event: MessageEvent) => {
      if (unmountedRef.current) return;
      try {
        const parsed = JSON.parse(event.data) as T;
        setLastMessage(parsed);
        onMessageRef.current?.(parsed);
      } catch {
        // If not JSON, pass raw data
        setLastMessage(event.data as unknown as T);
        onMessageRef.current?.(event.data as unknown as T);
      }
    };

    ws.onclose = () => {
      if (unmountedRef.current) return;
      setConnected(false);

      // Exponential backoff: 1s, 2s, 4s, 8s, 16s, 30s max
      if (retriesRef.current < maxRetries) {
        const delay = Math.min(
          reconnectInterval * Math.pow(2, retriesRef.current),
          30_000,
        );
        retriesRef.current += 1;
        reconnectTimerRef.current = setTimeout(() => {
          if (!unmountedRef.current) {
            connect();
          }
        }, delay);
      }
    };

    ws.onerror = () => {
      // onerror is always followed by onclose, so reconnect logic lives there
    };
  }, [url, reconnectInterval, maxRetries]);

  const send = useCallback((data: string | object) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(typeof data === 'string' ? data : JSON.stringify(data));
    }
  }, []);

  const reconnect = useCallback(() => {
    clearReconnectTimer();
    retriesRef.current = 0;
    connect();
  }, [connect, clearReconnectTimer]);

  useEffect(() => {
    unmountedRef.current = false;
    connect();

    return () => {
      unmountedRef.current = true;
      clearReconnectTimer();
      if (wsRef.current) {
        wsRef.current.onopen = null;
        wsRef.current.onclose = null;
        wsRef.current.onerror = null;
        wsRef.current.onmessage = null;
        wsRef.current.close();
        wsRef.current = null;
      }
    };
  }, [connect, clearReconnectTimer]);

  return { connected, lastMessage, send, reconnect };
}
