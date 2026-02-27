import Link from 'next/link';

export default function NotFound() {
  return (
    <div className="min-h-screen bg-background flex items-center justify-center">
      <div className="text-center space-y-6">
        <h1 className="font-mono text-6xl font-bold">404</h1>
        <p className="font-mono text-lg text-muted-foreground">Page not found</p>
        <Link
          href="/"
          className="inline-block font-mono text-sm tracking-wider bg-foreground text-background px-6 py-3 hover:bg-foreground/90 transition-colors"
        >
          GO HOME
        </Link>
      </div>
    </div>
  );
}
