const tenantId = '11111111-1111-4111-8111-111111111111';
const subjectId = '11111111-0000-4000-8000-000000000001';
const noStore = { 'Cache-Control': 'no-store' };

export const dynamic = 'force-dynamic';

export async function POST() {
  if (process.env.TRACKLINE_DEV_AUTH_ENABLED !== 'true') {
    return Response.json({ title: 'Not found' }, { status: 404, headers: noStore });
  }

  const baseUrl = process.env.TRACKLINE_DEV_AUTH_BASE_URL ?? 'http://fleet-api:3000';
  const tokenUrl = new URL(`/dev/token/${tenantId}/${subjectId}/free`, baseUrl);

  try {
    const response = await fetch(tokenUrl, {
      cache: 'no-store',
      signal: AbortSignal.timeout(5000),
    });
    if (!response.ok) {
      return Response.json(
        { title: 'Could not start the local development session' },
        { status: 503, headers: noStore },
      );
    }

    const payload: { access_token?: unknown } = await response.json();
    if (typeof payload.access_token !== 'string' || payload.access_token.length === 0) {
      return Response.json(
        { title: 'Dev token service returned an invalid response' },
        { status: 502, headers: noStore },
      );
    }

    return Response.json({ access_token: payload.access_token }, { headers: noStore });
  } catch {
    return Response.json(
      { title: 'Local development auth is unavailable; check that fleet-api is healthy' },
      { status: 503, headers: noStore },
    );
  }
}