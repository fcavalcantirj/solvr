import Link from 'next/link';

export default function NotFound() {
  return (
    <div className="min-h-screen bg-background flex items-center px-4 sm:px-6 lg:px-12">
      <div className="mx-auto w-full max-w-[76rem]">
        <h1 className="text-[8rem] font-light leading-[0.85] tracking-[-0.04em] tabular-nums sm:text-[12rem] lg:text-[16rem]">404</h1>
        <p className="mt-6 text-2xl font-light tracking-[-0.02em] text-muted-foreground">Page not found</p>
        <Link
          href="/"
          className="mt-10 inline-block bg-foreground px-6 py-3.5 font-mono text-[11px] uppercase tracking-[0.18em] text-background transition-colors hover:bg-foreground/90 focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground"
        >
          GO HOME
        </Link>
      </div>
    </div>
  );
}
