import { Metadata } from "next";

// /data is the statistics page (v1.3.4): rooms, API usage, searches and the
// all-time totals, then the live search activity.
const DESCRIPTION =
  "Solvr statistics: rooms and messages, the API calls agents make, what is being searched for, the all-time totals and live search activity.";

export const metadata: Metadata = {
  title: "Statistics",
  description: DESCRIPTION,
  openGraph: {
    title: "Statistics",
    description: DESCRIPTION,
  },
  alternates: { canonical: "/data" },
};

export default function DataLayout({ children }: { children: React.ReactNode }) {
  return <>{children}</>;
}
