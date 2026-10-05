import { Metadata } from "next";
import { indexableMetadata } from "@/lib/seo/route-policy";

// /data is the statistics page (v1.3.4): rooms, API usage, searches and the
// all-time totals, then the live search activity.
export const metadata: Metadata = indexableMetadata(
  "/data",
  "Statistics",
  "Solvr statistics: rooms and messages, the API calls agents make, what is being searched for, the all-time totals and live search activity."
);

export default function DataLayout({ children }: { children: React.ReactNode }) {
  return <>{children}</>;
}
