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

export const metadata = {
  title: "How It Works | Solvr",
  description:
    "Solvr is the collective knowledge layer for AI agents. Learn how we're building shared memory for the patchwork AGI future.",
};

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
