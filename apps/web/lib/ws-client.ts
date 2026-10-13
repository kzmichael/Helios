// WebSocket subscription for live vehicle positions — the mechanism that makes
// the "position freshness p99 < 5s" SLO (docs/01_APPLICATION.md §5) visible on
// screen. Falls back to nothing automatically; the map page decides whether to
// poll REST instead while disconnected.
import type { Position } from './api-client';

export type PositionHandler = (position: Position) => void;

export interface WsClient {
  close: () => void;
}

const WS_BASE = process.env.NEXT_PUBLIC_WS_BASE ?? 'ws://localhost:8080/v1';

/**
 * NOTE — the `/v1/positions/stream` endpoint this connects to does not exist
 * in fleet-api yet. apps/_contracts/openapi.yaml currently documents `/positions`
 * as a plain REST poll endpoint only. Adding the WebSocket upgrade path to
 * fleet-api (or a small dedicated realtime service) is unfinished application
 * work — track it as a follow-up story before this file is relied upon.
 */
export function connectPositions(onPosition: PositionHandler, onError?: (e: Event) => void): WsClient {
  const token = typeof window !== 'undefined' ? window.localStorage.getItem('trackline_dev_token') : null;
  const url = `${WS_BASE}/positions/stream${token ? `?token=${encodeURIComponent(token)}` : ''}`;

  let socket: WebSocket | null = null;
  let closedByCaller = false;

  const connect = () => {
    socket = new WebSocket(url);

    socket.onmessage = (event) => {
      try {
        const position = JSON.parse(event.data) as Position;
        onPosition(position);
      } catch {
        // Malformed frame — drop it, do not crash the subscription.
      }
    };

    socket.onerror = (e) => onError?.(e);

    socket.onclose = () => {
      if (!closedByCaller) {
        // Simple reconnect with a fixed delay. A real backoff/jitter policy
        // (matching notifier's retry.rs reasoning) is worth adding once this
        // is exercised under real network conditions.
        setTimeout(connect, 2000);
      }
    };
  };

  connect();

  return {
    close: () => {
      closedByCaller = true;
      socket?.close();
    },
  };
}
