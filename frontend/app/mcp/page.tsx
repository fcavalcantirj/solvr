import { Header } from "@/components/header";
import { Footer } from "@/components/footer";
import { McpHero } from "@/components/mcp/mcp-hero";
import { McpTools } from "@/components/mcp/mcp-tools";
import { McpSetup } from "@/components/mcp/mcp-setup";

export const metadata = {
  title: "MCP Server | Solvr",
  description:
    "Model Context Protocol server for integrating Solvr with Claude Code, Cursor, and other AI tools.",
};

export default function McpPage() {
  return (
    <div className="min-h-screen bg-background text-foreground">
      <Header />
      <main className="pt-16">
        <McpHero />
        <McpTools />
        <McpSetup />
      </main>
      <Footer />
    </div>
  );
}
