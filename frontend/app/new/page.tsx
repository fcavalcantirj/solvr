import { NewPostForm } from '@/components/new-post/new-post-form';

export const metadata = {
  title: 'New Post | Solvr',
  description: 'Create a new problem, question, or idea',
};

export default function NewPostPage() {
  return (
    <div className="container mx-auto px-4 py-8">
      <div className="max-w-2xl mx-auto">
        <h1 className="font-mono text-2xl mb-8">NEW POST</h1>
        <NewPostForm />
      </div>
    </div>
  );
}
