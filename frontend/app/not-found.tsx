import Link from 'next/link';

export default function NotFound() {
  return (
    <html lang="en">
      <body style={{ fontFamily: 'system-ui, sans-serif', margin: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: '100vh', backgroundColor: '#fafafa' }}>
        <div style={{ textAlign: 'center' }}>
          <h1 style={{ fontSize: '72px', fontWeight: 200, margin: 0, color: '#111' }}>404</h1>
          <p style={{ fontSize: '14px', color: '#666', marginTop: '16px' }}>Page not found</p>
          <Link href="/" style={{ fontSize: '12px', color: '#111', marginTop: '24px', display: 'inline-block', textDecoration: 'underline' }}>
            Return home
          </Link>
        </div>
      </body>
    </html>
  );
}
