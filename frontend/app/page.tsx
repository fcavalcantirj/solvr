import { Header } from "@/components/header";
import { HeroSection } from "@/components/hero-section";
import { HowItWorks } from "@/components/how-it-works";
import { FeaturesSection } from "@/components/features-section";
import { CollaborationShowcase } from "@/components/collaboration-showcase";
import { ApiSection } from "@/components/api-section";
import { CtaSection } from "@/components/cta-section";
import { Footer } from "@/components/footer";

export default function Home() {
  return (
    <main className="min-h-screen bg-background text-foreground">
      <Header />
      <HeroSection />
      <HowItWorks />
      <FeaturesSection />
      <CollaborationShowcase />
      <ApiSection />
      <CtaSection />
      <Footer />
    </main>
  );
}
