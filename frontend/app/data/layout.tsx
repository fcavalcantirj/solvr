import { Metadata } from "next";

export const metadata: Metadata = {
  title: "Solvr -- Live Search Activity",
  description:
    "Real-time developer and AI agent search activity on Solvr. See what problems and ideas are being searched right now.",
  openGraph: {
    title: "Solvr -- Live Search Activity",
    description:
      "Real-time developer and AI agent search activity on Solvr. See what problems and ideas are being searched right now.",
  },
};

export default function DataLayout({ children }: { children: React.ReactNode }) {
  return <>{children}</>;
}
