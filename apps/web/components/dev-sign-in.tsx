'use client';

import { useState } from 'react';
import { DEV_TOKEN_STORAGE_KEY } from '../lib/api-client';

export function DevSignIn() {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function signIn() {
    setBusy(true);
    setError(null);

    try {
      const response = await fetch('/api/dev-session', {
        method: 'POST',
        cache: 'no-store',
      });
      const payload: { access_token?: unknown; title?: string } = await response.json();
      if (!response.ok || typeof payload.access_token !== 'string') {
        throw new Error(payload.title ?? 'Local sign-in failed');
      }

      window.localStorage.setItem(DEV_TOKEN_STORAGE_KEY, payload.access_token);
      window.location.reload();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Local sign-in failed');
      setBusy(false);
    }
  }

  return (
    <span style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
      {error && <span role="alert">{error}</span>}
      <button type="button" onClick={signIn} disabled={busy}>
        {busy ? 'Signing in…' : 'Use local dev account'}
      </button>
    </span>
  );
}