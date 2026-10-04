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
      <body style={{ fontFamily: 'system-ui, sans-serif', margin: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: '100vh', backgroundColor: '#f9f8f5' }}>
        <div style={{ textAlign: 'center' }}>
          <h1 style={{ fontSize: '56px', fontWeight: 300, letterSpacing: '-0.04em', margin: 0, color: '#030303' }}>Something went wrong</h1>
          <p style={{ fontSize: '14px', color: '#5b5b5b', marginTop: '16px' }}>{error.message}</p>
          <button
            onClick={() => reset()}
            style={{ fontSize: '11px', letterSpacing: '0.18em', textTransform: 'uppercase', color: '#f9f8f5', marginTop: '28px', padding: '12px 20px', border: 'none', background: '#030303', cursor: 'pointer' }}
          >
            Try again
          </button>
        </div>
      </body>
    </html>
  );
}
