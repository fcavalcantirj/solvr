"use client";

import { Header } from "@/components/header";
import { IPFSStatusIndicator } from "@/components/admin/ipfs-status";
import { PageHeading } from "@/components/page/page-header";
import { PageSection } from "@/components/page/page-section";

// The system page opens like /status: a plain heading, then each piece of
// infrastructure with its state set big.
export default function AdminSystemPage() {
  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="pt-16 pb-16">
        <PageHeading
          title="SYSTEM"
          caption="ADMIN"
          lede="Monitor infrastructure health and system status."
        />

        {/* Infrastructure Section */}
        <PageSection heading="INFRASTRUCTURE">
          <IPFSStatusIndicator />
        </PageSection>
      </main>
    </div>
  );
}
