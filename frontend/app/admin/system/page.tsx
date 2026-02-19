"use client";

import { Header } from "@/components/header";
import { IPFSStatusIndicator } from "@/components/admin/ipfs-status";
import { Shield } from "lucide-react";

export default function AdminSystemPage() {
  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="pt-20">
        {/* Page Header */}
        <div className="border-b border-border">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-12 py-8 sm:py-12">
            <div className="flex items-center gap-3 mb-4">
              <div className="w-10 h-10 bg-foreground flex items-center justify-center shrink-0">
                <Shield className="w-5 h-5 text-background" />
              </div>
              <span className="font-mono text-xs tracking-wider text-muted-foreground">
                ADMIN
              </span>
            </div>
            <h1 className="font-mono text-3xl sm:text-4xl md:text-5xl font-medium tracking-tight text-foreground">
              SYSTEM
            </h1>
            <p className="font-mono text-xs sm:text-sm text-muted-foreground mt-3 max-w-xl">
              Monitor infrastructure health and system status.
            </p>
          </div>
        </div>

        {/* Content */}
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-12 py-8">
          {/* Infrastructure Section */}
          <h2 className="font-mono text-xs tracking-wider text-muted-foreground mb-6">
            INFRASTRUCTURE
          </h2>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <IPFSStatusIndicator />
          </div>
        </div>
      </main>
    </div>
  );
}
