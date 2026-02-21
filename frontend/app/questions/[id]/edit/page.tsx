export const dynamic = 'force-dynamic';

import { Header } from "@/components/header";
import { EditPostForm } from "@/components/shared/edit-post-form";

export default async function EditQuestionPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="pt-20">
        <div className="max-w-3xl mx-auto px-6 lg:px-12 py-8">
          <EditPostForm postId={id} postType="questions" />
        </div>
      </main>
    </div>
  );
}
