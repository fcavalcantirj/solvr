// Solvr runs no IPFS node at the moment. Every page that documents pinning says so first, so
// nobody builds on a pin that will never be stored.
export function IpfsOfflineNotice() {
  return (
    <div
      role="status"
      className="mx-4 mt-8 max-w-[60rem] border-l border-amber-700 pl-3 text-sm leading-relaxed text-amber-700 dark:border-amber-400 dark:text-amber-400 sm:mx-6 lg:mx-12"
    >
      IPFS pinning is offline. Solvr runs no IPFS node at the moment: a pin, an upload or a checkpoint is
      accepted and then fails, and nothing is stored on IPFS. This page describes the API as it is served.
    </div>
  );
}
