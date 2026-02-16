"use client";

// Force dynamic rendering - this page imports Header which uses client-side state
export const dynamic = 'force-dynamic';

import { Header } from "@/components/header";
import { Footer } from "@/components/footer";
import { SkillHero } from "@/components/skill/skill-hero";
import { SkillInstall } from "@/components/skill/skill-install";
import { SkillPreview } from "@/components/skill/skill-preview";



export default function SkillPage() {
  return (
    <div className="min-h-screen bg-background text-foreground">
      <Header />
      <main className="pt-16">
        <SkillHero />
        <SkillInstall />
        <SkillPreview />
      </main>
      <Footer />
    </div>
  );
}
