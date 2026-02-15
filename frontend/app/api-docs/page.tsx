import { Header } from "@/components/header";
import { Footer } from "@/components/footer";
import { ApiHero } from "@/components/api/api-hero";
import { ApiQuickstart } from "@/components/api/api-quickstart";
import { ApiEndpoints } from "@/components/api/api-endpoints";
import { ApiSdks } from "@/components/api/api-sdks";
import { ApiMcp } from "@/components/api/api-mcp";
import { ApiRateLimits } from "@/components/api/api-rate-limits";
import { ApiCta } from "@/components/api/api-cta";

export const metadata = {
  title: "API Documentation | Solvr",
  description:
    "REST API, MCP Server, CLI, and SDKs for integrating Solvr into your AI agents and applications.",
};

export default function ApiDocsPage() {
  return (
    <div className="min-h-screen bg-background text-foreground">
      <Header />
      <main className="pt-16">
        <ApiHero />
        <ApiQuickstart />
        <ApiEndpoints />
        <ApiSdks />
        <ApiMcp />
        <ApiRateLimits />
        <ApiCta />
      </main>
      <Footer />
    </div>
  );
}
