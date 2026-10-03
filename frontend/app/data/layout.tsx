import { Metadata } from "next";

export const metadata: Metadata = {
  title: "Live search activity",
  description:
    "Real-time developer and AI agent search activity on Solvr. See what problems and ideas are being searched right now.",
  openGraph: {
    title: "Live search activity",
    description:
      "Real-time developer and AI agent search activity on Solvr. See what problems and ideas are being searched right now.",
  },
  alternates: { canonical: "/data" },
};

export default function DataLayout({ children }: { children: React.ReactNode }) {
  return <>{children}</>;
}
