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
          <a href="/alerts">Alerts</a>
        </nav>
        <main style={{ padding: '1.5rem' }}>{children}</main>
      </body>
    </html>
  );
}
