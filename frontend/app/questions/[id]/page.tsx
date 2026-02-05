import { Header } from "@/components/header";
import { QuestionHeader } from "@/components/questions/detail/question-header";
import { QuestionContent } from "@/components/questions/detail/question-content";
import { AnswersList } from "@/components/questions/detail/answers-list";
import { QuestionSidePanel } from "@/components/questions/detail/question-side-panel";

export default async function QuestionDetailPage({
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
          <QuestionHeader id={id} />
          <div className="mt-8 grid grid-cols-1 lg:grid-cols-3 gap-8">
            <div className="lg:col-span-2 space-y-8">
              <QuestionContent />
              <AnswersList />
            </div>
            <div className="lg:col-span-1">
              <QuestionSidePanel />
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}
