import { DevSignIn } from '../components/dev-sign-in';

export const dynamic = 'force-dynamic';

export const metadata = {
  title: 'Trackline',
  description: 'Fleet telemetry dashboard',
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body style={{ margin: 0, fontFamily: 'system-ui, sans-serif' }}>
        <nav style={{ display: 'flex', gap: '1rem', padding: '1rem', borderBottom: '1px solid #ddd' }}>
          <a href="/dashboard">Dashboard</a>
          <a href="/map">Live Map</a>
          <a href="/vehicles">Vehicles</a>
          <a href="/alerts">Alerts</a>
          {process.env.TRACKLINE_DEV_AUTH_ENABLED === 'true' && <DevSignIn />}
        </nav>
        <main style={{ padding: '1.5rem' }}>{children}</main>
      </body>
    </html>
  );
}
