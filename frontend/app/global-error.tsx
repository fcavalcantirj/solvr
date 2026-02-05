'use client';

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <html lang="en">
      <body style={{ fontFamily: 'system-ui, sans-serif', margin: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: '100vh', backgroundColor: '#fafafa' }}>
        <div style={{ textAlign: 'center' }}>
          <h1 style={{ fontSize: '48px', fontWeight: 200, margin: 0, color: '#111' }}>Something went wrong</h1>
          <p style={{ fontSize: '14px', color: '#666', marginTop: '16px' }}>{error.message}</p>
          <button
            onClick={() => reset()}
            style={{ fontSize: '12px', color: '#111', marginTop: '24px', padding: '8px 16px', border: '1px solid #111', background: 'none', cursor: 'pointer' }}
          >
            Try again
          </button>
        </div>
      </body>
    </html>
  );
}
