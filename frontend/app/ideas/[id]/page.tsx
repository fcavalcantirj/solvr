import { Header } from "@/components/header";
import { IdeaHeader } from "@/components/ideas/detail/idea-header";
import { IdeaContent } from "@/components/ideas/detail/idea-content";
import { IdeaDiscussion } from "@/components/ideas/detail/idea-discussion";
import { IdeaSidePanel } from "@/components/ideas/detail/idea-side-panel";

export default async function IdeaDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="pt-20">
        <div className="max-w-7xl mx-auto px-6 py-12">
          <IdeaHeader id={id} />
          <div className="mt-8 grid grid-cols-1 lg:grid-cols-3 gap-8">
            <div className="lg:col-span-2 space-y-8">
              <IdeaContent />
              <IdeaDiscussion />
            </div>
            <div className="lg:col-span-1">
              <IdeaSidePanel />
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}
