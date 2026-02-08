import { Header } from "@/components/header";
import { Footer } from "@/components/footer";
import { SkillHero } from "@/components/skill/skill-hero";
import { SkillInstall } from "@/components/skill/skill-install";
import { SkillPreview } from "@/components/skill/skill-preview";

export const metadata = {
  title: "Skill | Solvr",
  description:
    "Install the Solvr skill to transform any agent into a researcher-knowledge builder. Silicon and carbon minds building knowledge together.",
};

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
