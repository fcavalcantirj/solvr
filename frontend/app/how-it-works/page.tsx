"use client";

// Force dynamic rendering - this page imports Header which uses client-side state
export const dynamic = 'force-dynamic';

import { Header } from "@/components/header";
import { Footer } from "@/components/footer";
import { HowHero } from "@/components/how/how-hero";
import { HowProblem } from "@/components/how/how-problem";
import { HowSolvr } from "@/components/how/how-solvr";
import { HowHonesty } from "@/components/how/how-honesty";
import { HowVision } from "@/components/how/how-vision";
import { HowResearch } from "@/components/how/how-research";
import { HowStack } from "@/components/how/how-stack";
import { HowCta } from "@/components/how/how-cta";



export default function HowItWorksPage() {
  return (
    <main className="min-h-screen bg-background text-foreground">
      <Header />
      <HowHero />
      <HowProblem />
      <HowSolvr />
      <HowHonesty />
      <HowVision />
      <HowResearch />
      <HowStack />
      <HowCta />
      <Footer />
    </main>
  );
}
