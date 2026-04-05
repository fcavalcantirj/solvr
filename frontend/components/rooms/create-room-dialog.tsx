"use client";

import { useState, useCallback } from 'react';
import { useRouter } from 'next/navigation';
import { Plus } from 'lucide-react';
import { useAuth } from '@/hooks/use-auth';
import { api } from '@/lib/api';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';

export function CreateRoomDialog() {
  const { isAuthenticated, setShowAuthModal } = useAuth();
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [category, setCategory] = useState('');
  const [tags, setTags] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleOpen = useCallback(() => {
    if (!isAuthenticated) {
      setShowAuthModal(true);
      return;
    }
    setOpen(true);
  }, [isAuthenticated, setShowAuthModal]);

  const handleSubmit = useCallback(async () => {
    if (!name.trim() || submitting) return;

    setSubmitting(true);
    setError(null);

    try {
      const payload: {
        display_name: string;
        description?: string;
        category?: string;
        tags?: string[];
      } = { display_name: name.trim() };

      if (description.trim()) payload.description = description.trim();
      if (category.trim()) payload.category = category.trim();
      if (tags.trim()) payload.tags = tags.split(',').map((t) => t.trim()).filter(Boolean);

      const result = await api.createRoom(payload);
      const slug = (result.data.room as Record<string, string>).slug;
      setOpen(false);
      setName('');
      setDescription('');
      setCategory('');
      setTags('');
      router.push(`/rooms/${slug}`);
    } catch {
      setError('Failed to create room');
    } finally {
      setSubmitting(false);
    }
  }, [name, description, category, tags, submitting, router]);

  return (
    <>
      <Button onClick={handleOpen} size="sm" className="gap-1.5">
        <Plus className="w-4 h-4" />
        Create Room
      </Button>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Create a Room</DialogTitle>
          </DialogHeader>

          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <Label htmlFor="room-name">Room Name</Label>
              <Input
                id="room-name"
                placeholder="Room name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                maxLength={100}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="room-description">Description (optional)</Label>
              <Textarea
                id="room-description"
                placeholder="What is this room about?"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                rows={3}
                maxLength={500}
              />
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="room-category">Category (optional)</Label>
                <Input
                  id="room-category"
                  placeholder="e.g. debugging"
                  value={category}
                  onChange={(e) => setCategory(e.target.value)}
                  maxLength={50}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="room-tags">Tags (optional)</Label>
                <Input
                  id="room-tags"
                  placeholder="go, postgres"
                  value={tags}
                  onChange={(e) => setTags(e.target.value)}
                  maxLength={200}
                />
              </div>
            </div>

            {error && (
              <p className="text-sm text-destructive">{error}</p>
            )}
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button
              onClick={handleSubmit}
              disabled={!name.trim() || submitting}
            >
              {submitting ? 'Creating...' : 'Create'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
