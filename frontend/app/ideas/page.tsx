import { Header } from "@/components/header";
import { IdeasFilters } from "@/components/ideas/ideas-filters";
import { IdeasList } from "@/components/ideas/ideas-list";
import { IdeasSidebar } from "@/components/ideas/ideas-sidebar";
import { Lightbulb, Plus } from "lucide-react";
import { Button } from "@/components/ui/button";

export default function IdeasPage() {
  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="pt-20">
        {/* Page Header */}
        <div className="border-b border-border overflow-hidden">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 py-8 sm:py-12">
            <div className="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-4">
              <div>
                <div className="flex items-center gap-3 mb-4">
                  <div className="w-10 h-10 bg-foreground flex items-center justify-center shrink-0">
                    <Lightbulb className="w-5 h-5 text-background" />
                  </div>
                  <span className="font-mono text-xs tracking-wider text-muted-foreground">
                    COLLECTIVE INNOVATION
                  </span>
                </div>
                <h1 className="font-mono text-3xl sm:text-4xl md:text-5xl font-medium tracking-tight text-foreground">
                  IDEAS
                </h1>
                <p className="font-mono text-xs sm:text-sm text-muted-foreground mt-3 max-w-xl">
                  Seeds of possibility. Sparks before the fire. The raw, unpolished thoughts that could become breakthroughs.
                </p>
              </div>
              <Button className="font-mono text-xs tracking-wider w-full sm:w-auto shrink-0">
                <Plus className="w-3 h-3 mr-2" />
                SPARK IDEA
              </Button>
            </div>

            {/* Quick Stats */}
            <div className="grid grid-cols-2 sm:flex sm:items-center gap-4 sm:gap-8 mt-8 pt-6 border-t border-border">
              <div className="flex flex-col sm:flex-row sm:items-baseline">
                <span className="font-mono text-xl sm:text-2xl font-medium text-foreground">2,847</span>
                <span className="font-mono text-[10px] sm:text-xs text-muted-foreground sm:ml-2">TOTAL</span>
              </div>
              <div className="flex flex-col sm:flex-row sm:items-baseline">
                <span className="font-mono text-xl sm:text-2xl font-medium text-amber-600">342</span>
                <span className="font-mono text-[10px] sm:text-xs text-muted-foreground sm:ml-2">SPARKS</span>
              </div>
              <div className="flex flex-col sm:flex-row sm:items-baseline">
                <span className="font-mono text-xl sm:text-2xl font-medium text-blue-600">156</span>
                <span className="font-mono text-[10px] sm:text-xs text-muted-foreground sm:ml-2">DEVELOPING</span>
              </div>
              <div className="flex flex-col sm:flex-row sm:items-baseline">
                <span className="font-mono text-xl sm:text-2xl font-medium text-emerald-600">89</span>
                <span className="font-mono text-[10px] sm:text-xs text-muted-foreground sm:ml-2">REALIZED</span>
              </div>
            </div>
          </div>
        </div>

        {/* Filters */}
        <IdeasFilters />

        {/* Main Content */}
        <div className="max-w-7xl mx-auto px-4 sm:px-6 py-8">
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
            <div className="lg:col-span-2">
              <IdeasList />
            </div>
            <div className="lg:col-span-1">
              <IdeasSidebar />
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}
