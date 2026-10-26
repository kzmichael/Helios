// WebSocket subscription for live vehicle positions — the mechanism that makes
// the "position freshness p99 < 5s" SLO (docs/01_APPLICATION.md §5) visible on
// screen. The server side is fleet-api's /v1/positions/stream, reached through
// edge-gateway, which accepts the access token as ?token= on this path only,
// because a browser WebSocket cannot send an Authorization header.
import { DEV_TOKEN_STORAGE_KEY, type Position } from './api-client';

export type PositionHandler = (position: Position) => void;

export interface WsClient {
  close: () => void;
}

const WS_BASE = process.env.NEXT_PUBLIC_WS_BASE ?? 'ws://localhost:8080/v1';

const BASE_DELAY_MS = 1000;
const MAX_DELAY_MS = 30_000;

/**
 * Connects, and keeps reconnecting until closed. Reconnects back off
 * exponentially with full jitter, so a gateway restart does not bring every open
 * dashboard back in the same instant (the same reasoning as notifier's retry.rs).
 * `onStatus(true)` fires on every (re)connect: the caller should refetch the REST
 * snapshot then, because frames sent while disconnected are not replayed.
 */
export function connectPositions(
  onPosition: PositionHandler,
  onStatus?: (connected: boolean) => void,
): WsClient {
  let socket: WebSocket | null = null;
  let closedByCaller = false;
  let attempt = 0;
  let timer: ReturnType<typeof setTimeout> | undefined;

  const connect = () => {
    // Read the token on every attempt: it may have been refreshed since the last one.
    const token = typeof window !== 'undefined' ? window.localStorage.getItem(DEV_TOKEN_STORAGE_KEY) : null;
    socket = new WebSocket(`${WS_BASE}/positions/stream${token ? `?token=${encodeURIComponent(token)}` : ''}`);

    socket.onopen = () => {
      attempt = 0;
      onStatus?.(true);
    };

    socket.onmessage = (event) => {
      try {
        onPosition(JSON.parse(event.data) as Position);
      } catch {
        // Malformed frame — drop it, do not crash the subscription.
      }
    };

    socket.onclose = () => {
      onStatus?.(false);
      if (closedByCaller) return;
      const ceiling = Math.min(MAX_DELAY_MS, BASE_DELAY_MS * 2 ** attempt++);
      timer = setTimeout(connect, Math.random() * ceiling);
    };
  };

  connect();

  return {
    close: () => {
      closedByCaller = true;
      if (timer) clearTimeout(timer);
      socket?.close();
    },
  };
}
