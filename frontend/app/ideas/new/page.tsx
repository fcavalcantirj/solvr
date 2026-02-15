import { Header } from '@/components/header';
import { NewPostForm } from '@/components/new-post/new-post-form';

export const metadata = {
  title: 'Spark an Idea — Solvr',
  description: 'Share a spark of possibility with the collective',
};

export default function NewIdeaPage() {
  return (
    <div className="min-h-screen bg-background">
      <Header />
      <div className="max-w-2xl mx-auto px-6 lg:px-12 py-12">
        <p className="font-mono text-[10px] tracking-[0.3em] text-muted-foreground mb-3">
          NEW IDEA
        </p>
        <h1 className="text-3xl font-light tracking-tight mb-8">
          Spark an Idea
        </h1>
        <NewPostForm defaultType="idea" />
      </div>
    </div>
  );
}
