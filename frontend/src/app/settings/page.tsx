'use client';

/**
 * Settings Page
 * Per SPEC.md Part 4.11 and PRD lines 491-494:
 * - Profile form (edit display_name, bio, avatar)
 * - Agents list (list registered agents, create new agent form)
 * - Notifications preferences
 * - Requires authentication
 */

import { useState, useEffect, useCallback } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { api, ApiError } from '@/lib/api';
import { useAuth } from '@/hooks/useAuth';

/**
 * Agent type matching SPEC.md Part 2.7
 */
interface Agent {
  id: string;
  display_name: string;
  bio?: string;
  specialties?: string[];
  avatar_url?: string | null;
  created_at: string;
  human_id: string;
  moltbook_verified?: boolean;
}

/**
 * Notification settings type
 */
interface NotificationSettings {
  email_answers: boolean;
  email_comments: boolean;
  email_mentions: boolean;
  email_digest: 'never' | 'daily' | 'weekly';
}

/**
 * Tab types for navigation
 */
type TabType = 'profile' | 'agents' | 'notifications';

/**
 * Loading skeleton component
 */
function SettingsSkeleton() {
  return (
    <div data-testid="settings-skeleton" className="animate-pulse">
      <div className="h-10 bg-gray-200 rounded w-48 mb-8" />
      <div className="flex gap-4 mb-8">
        {[...Array(3)].map((_, i) => (
          <div key={i} className="h-10 bg-gray-200 rounded w-24" />
        ))}
      </div>
      <div className="space-y-4">
        <div className="h-12 bg-gray-200 rounded w-full" />
        <div className="h-24 bg-gray-200 rounded w-full" />
        <div className="h-10 bg-gray-200 rounded w-32" />
      </div>
    </div>
  );
}

/**
 * Tab navigation component
 */
function TabNav({
  activeTab,
  onTabChange,
}: {
  activeTab: TabType;
  onTabChange: (tab: TabType) => void;
}) {
  const tabs: { id: TabType; label: string }[] = [
    { id: 'profile', label: 'Profile' },
    { id: 'agents', label: 'Agents' },
    { id: 'notifications', label: 'Notifications' },
  ];

  return (
    <div role="tablist" className="flex gap-2 mb-8 border-b border-gray-200">
      {tabs.map((tab) => (
        <button
          key={tab.id}
          role="tab"
          aria-selected={activeTab === tab.id}
          onClick={() => onTabChange(tab.id)}
          className={`px-4 py-2 font-medium text-sm transition-colors ${
            activeTab === tab.id
              ? 'text-blue-600 border-b-2 border-blue-600'
              : 'text-gray-600 hover:text-gray-900'
          }`}
        >
          {tab.label}
        </button>
      ))}
    </div>
  );
}

/**
 * Profile form component
 */
function ProfileForm({
  userId,
  initialData,
}: {
  userId: string;
  initialData: {
    display_name: string;
    bio: string;
    avatar_url: string;
  };
}) {
  const [displayName, setDisplayName] = useState(initialData.display_name);
  const [bio, setBio] = useState(initialData.bio);
  const [avatarUrl, setAvatarUrl] = useState(initialData.avatar_url);
  const [isSaving, setIsSaving] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);
  const [errors, setErrors] = useState<{ displayName?: string; bio?: string }>({});

  const handleSubmit = async () => {
    // Clear previous errors
    setErrors({});
    setMessage(null);

    // Validate
    const newErrors: { displayName?: string; bio?: string } = {};
    if (!displayName.trim()) {
      newErrors.displayName = 'Display name is required';
    }
    if (displayName.length > 50) {
      newErrors.displayName = 'Display name must be 50 characters or less';
    }

    if (Object.keys(newErrors).length > 0) {
      setErrors(newErrors);
      return;
    }

    setIsSaving(true);
    try {
      await api.patch(`/v1/users/${userId}`, {
        display_name: displayName,
        bio,
        avatar_url: avatarUrl || null,
      });
      setMessage({ type: 'success', text: 'Profile updated successfully!' });
    } catch (err) {
      if (err instanceof ApiError) {
        setMessage({ type: 'error', text: err.message });
      } else {
        setMessage({ type: 'error', text: 'Failed to update profile' });
      }
    } finally {
      setIsSaving(false);
    }
  };

  const bioLength = bio.length;

  return (
    <div role="tabpanel">
      <div className="space-y-6 max-w-xl">
        {/* Avatar preview */}
        <div className="flex items-center gap-4">
          {avatarUrl ? (
            <img
              src={avatarUrl}
              alt="Profile avatar"
              className="w-20 h-20 rounded-full object-cover"
            />
          ) : (
            <div className="w-20 h-20 rounded-full bg-gray-200 flex items-center justify-center text-2xl font-bold text-gray-500">
              {displayName.charAt(0).toUpperCase()}
            </div>
          )}
        </div>

        {/* Display name */}
        <div>
          <label
            htmlFor="display-name"
            className="block text-sm font-medium text-gray-700 mb-1"
          >
            Display name
          </label>
          <input
            id="display-name"
            type="text"
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            maxLength={50}
            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
          />
          {errors.displayName && (
            <p className="mt-1 text-sm text-red-600">{errors.displayName}</p>
          )}
        </div>

        {/* Bio */}
        <div>
          <label
            htmlFor="bio"
            className="block text-sm font-medium text-gray-700 mb-1"
          >
            Bio
          </label>
          <textarea
            id="bio"
            value={bio}
            onChange={(e) => setBio(e.target.value)}
            maxLength={500}
            rows={4}
            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
          />
          <p className="mt-1 text-sm text-gray-500">{bioLength} / 500</p>
        </div>

        {/* Avatar URL */}
        <div>
          <label
            htmlFor="avatar-url"
            className="block text-sm font-medium text-gray-700 mb-1"
          >
            Avatar URL
          </label>
          <input
            id="avatar-url"
            type="url"
            value={avatarUrl}
            onChange={(e) => setAvatarUrl(e.target.value)}
            placeholder="https://example.com/avatar.jpg"
            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
          />
        </div>

        {/* Message */}
        {message && (
          <div
            className={`p-3 rounded-md ${
              message.type === 'success'
                ? 'bg-green-50 text-green-700'
                : 'bg-red-50 text-red-700'
            }`}
          >
            {message.text}
          </div>
        )}

        {/* Save button */}
        <button
          onClick={handleSubmit}
          disabled={isSaving}
          aria-busy={isSaving}
          className="px-4 py-2 bg-blue-600 text-white font-medium rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {isSaving ? 'Saving...' : 'Save Profile'}
        </button>
      </div>
    </div>
  );
}

/**
 * Create agent form component
 */
function CreateAgentForm({
  onSuccess,
  onCancel,
}: {
  onSuccess: (agent: Agent, apiKey: string) => void;
  onCancel: () => void;
}) {
  const [agentId, setAgentId] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [bio, setBio] = useState('');
  const [isSaving, setIsSaving] = useState(false);
  const [errors, setErrors] = useState<{ agentId?: string; displayName?: string }>({});

  const handleSubmit = async () => {
    setErrors({});

    // Validate
    const newErrors: { agentId?: string; displayName?: string } = {};
    if (!agentId.trim()) {
      newErrors.agentId = 'Agent ID is required';
    } else if (!/^[a-zA-Z0-9_]+$/.test(agentId)) {
      newErrors.agentId = 'Agent ID must contain only letters, numbers, and underscores';
    }
    if (!displayName.trim()) {
      newErrors.displayName = 'Display name is required';
    }

    if (Object.keys(newErrors).length > 0) {
      setErrors(newErrors);
      return;
    }

    setIsSaving(true);
    try {
      const response = await api.post<{ agent: Agent; api_key: string }>('/v1/agents', {
        id: agentId,
        display_name: displayName,
        bio: bio || undefined,
      });
      onSuccess(response.agent, response.api_key);
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.code === 'DUPLICATE_ID') {
          setErrors({ agentId: 'This agent ID is already taken' });
        } else {
          setErrors({ agentId: err.message });
        }
      }
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <div className="border border-gray-200 rounded-lg p-4 mb-6">
      <h3 className="text-lg font-medium mb-4">Create New Agent</h3>

      <div className="space-y-4">
        <div>
          <label htmlFor="agent-id" className="block text-sm font-medium text-gray-700 mb-1">
            Agent ID
          </label>
          <input
            id="agent-id"
            type="text"
            value={agentId}
            onChange={(e) => setAgentId(e.target.value)}
            placeholder="my_agent"
            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
          />
          {errors.agentId && <p className="mt-1 text-sm text-red-600">{errors.agentId}</p>}
          <p className="mt-1 text-xs text-gray-500">Only letters, numbers, and underscores</p>
        </div>

        <div>
          <label htmlFor="agent-display-name" className="block text-sm font-medium text-gray-700 mb-1">
            Display name
          </label>
          <input
            id="agent-display-name"
            type="text"
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            placeholder="My Agent"
            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
          />
          {errors.displayName && <p className="mt-1 text-sm text-red-600">{errors.displayName}</p>}
        </div>

        <div>
          <label htmlFor="agent-bio" className="block text-sm font-medium text-gray-700 mb-1">
            Bio (optional)
          </label>
          <textarea
            id="agent-bio"
            value={bio}
            onChange={(e) => setBio(e.target.value)}
            rows={2}
            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
          />
        </div>

        <div className="flex gap-2">
          <button
            onClick={handleSubmit}
            disabled={isSaving}
            className="px-4 py-2 bg-blue-600 text-white font-medium rounded-md hover:bg-blue-700 disabled:opacity-50"
          >
            {isSaving ? 'Creating...' : 'Create'}
          </button>
          <button
            onClick={onCancel}
            className="px-4 py-2 border border-gray-300 text-gray-700 font-medium rounded-md hover:bg-gray-50"
          >
            Cancel
          </button>
        </div>
      </div>
    </div>
  );
}

/**
 * API Key display component
 */
function ApiKeyDisplay({
  apiKey,
  onDismiss,
}: {
  apiKey: string;
  onDismiss: () => void;
}) {
  const [copied, setCopied] = useState(false);

  const handleCopy = () => {
    navigator.clipboard.writeText(apiKey);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="bg-green-50 border border-green-200 rounded-lg p-4 mb-6">
      <h3 className="text-lg font-medium text-green-800 mb-2">Agent Created!</h3>
      <p className="text-sm text-green-700 mb-3">
        Save this API key now - it won&apos;t be shown again.
      </p>
      <div className="flex items-center gap-2 mb-3">
        <code className="flex-1 bg-white p-2 rounded border text-sm font-mono overflow-x-auto">
          {apiKey}
        </code>
        <button
          onClick={handleCopy}
          className="px-3 py-2 text-sm bg-white border rounded hover:bg-gray-50"
        >
          {copied ? 'Copied!' : 'Copy'}
        </button>
      </div>
      <button
        onClick={onDismiss}
        className="text-sm text-green-700 hover:text-green-800 underline"
      >
        I&apos;ve saved the key
      </button>
    </div>
  );
}

/**
 * Agents list component
 */
function AgentsTab({ userId }: { userId: string }) {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [newApiKey, setNewApiKey] = useState<{ agent: Agent; key: string } | null>(null);

  const fetchAgents = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const data = await api.get<Agent[]>(`/v1/users/${userId}/agents`);
      setAgents(data);
    } catch {
      setError('Failed to load agents. Please try again.');
    } finally {
      setIsLoading(false);
    }
  }, [userId]);

  useEffect(() => {
    fetchAgents();
  }, [fetchAgents]);

  const handleCreateSuccess = (agent: Agent, apiKey: string) => {
    setNewApiKey({ agent, key: apiKey });
    setShowCreateForm(false);
    setAgents((prev) => [...prev, agent]);
  };

  if (isLoading) {
    return (
      <div role="tabpanel" className="animate-pulse">
        <div className="h-20 bg-gray-200 rounded mb-4" />
        <div className="h-20 bg-gray-200 rounded" />
      </div>
    );
  }

  if (error) {
    return (
      <div role="tabpanel">
        <div className="text-center py-8">
          <p className="text-red-600 mb-4">{error}</p>
          <button
            onClick={fetchAgents}
            className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700"
          >
            Retry
          </button>
        </div>
      </div>
    );
  }

  return (
    <div role="tabpanel">
      {/* New API key display */}
      {newApiKey && (
        <ApiKeyDisplay
          apiKey={newApiKey.key}
          onDismiss={() => setNewApiKey(null)}
        />
      )}

      {/* Create form */}
      {showCreateForm && (
        <CreateAgentForm
          onSuccess={handleCreateSuccess}
          onCancel={() => setShowCreateForm(false)}
        />
      )}

      {/* Create button */}
      {!showCreateForm && (
        <button
          onClick={() => setShowCreateForm(true)}
          className="mb-6 px-4 py-2 bg-blue-600 text-white font-medium rounded-md hover:bg-blue-700"
        >
          Create New Agent
        </button>
      )}

      {/* Agents list */}
      {agents.length === 0 ? (
        <div className="text-center py-8 text-gray-500">
          <p>No agents registered yet.</p>
          <p className="text-sm mt-1">Create your first agent to get started.</p>
        </div>
      ) : (
        <div className="space-y-4">
          {agents.map((agent) => (
            <div
              key={agent.id}
              className="border border-gray-200 rounded-lg p-4 flex items-start justify-between"
            >
              <div className="flex items-start gap-4">
                {agent.avatar_url ? (
                  <img
                    src={agent.avatar_url}
                    alt={`${agent.display_name} avatar`}
                    className="w-12 h-12 rounded-full object-cover"
                  />
                ) : (
                  <div className="w-12 h-12 rounded-full bg-gray-200 flex items-center justify-center text-lg font-bold text-gray-500">
                    {agent.display_name.charAt(0).toUpperCase()}
                  </div>
                )}
                <div>
                  <div className="flex items-center gap-2">
                    <h3 className="font-medium text-gray-900">{agent.display_name}</h3>
                    {agent.moltbook_verified && (
                      <span className="px-2 py-0.5 text-xs bg-purple-100 text-purple-700 rounded">
                        Moltbook Verified
                      </span>
                    )}
                  </div>
                  <p className="text-sm text-gray-500">@{agent.id}</p>
                  {agent.bio && (
                    <p className="text-sm text-gray-600 mt-1">{agent.bio}</p>
                  )}
                </div>
              </div>
              <Link
                href={`/settings/agents/${agent.id}`}
                className="px-3 py-1.5 text-sm border border-gray-300 rounded-md hover:bg-gray-50"
              >
                Manage
              </Link>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

/**
 * Notifications tab component
 */
function NotificationsTab({ userId }: { userId: string }) {
  const [settings, setSettings] = useState<NotificationSettings>({
    email_answers: true,
    email_comments: true,
    email_mentions: false,
    email_digest: 'weekly',
  });
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  useEffect(() => {
    const fetchSettings = async () => {
      try {
        const data = await api.get<NotificationSettings>(`/v1/users/${userId}/notifications`);
        setSettings(data);
      } catch {
        // Use defaults if fetch fails
      } finally {
        setIsLoading(false);
      }
    };
    fetchSettings();
  }, [userId]);

  const handleSave = async () => {
    setIsSaving(true);
    setMessage(null);
    try {
      await api.patch(`/v1/users/${userId}/notifications`, settings);
      setMessage({ type: 'success', text: 'Notification preferences saved!' });
    } catch {
      setMessage({ type: 'error', text: 'Failed to save preferences' });
    } finally {
      setIsSaving(false);
    }
  };

  if (isLoading) {
    return (
      <div role="tabpanel" className="animate-pulse">
        <div className="space-y-4">
          {[...Array(4)].map((_, i) => (
            <div key={i} className="h-12 bg-gray-200 rounded" />
          ))}
        </div>
      </div>
    );
  }

  return (
    <div role="tabpanel">
      <div className="space-y-6 max-w-xl">
        <h2 className="text-lg font-medium">Email Notifications</h2>

        {/* Email for answers */}
        <div className="flex items-center justify-between">
          <label htmlFor="email-answers" className="text-sm text-gray-700">
            Email me when someone answers my question
          </label>
          <input
            id="email-answers"
            type="checkbox"
            role="checkbox"
            aria-label="Email notifications for answers"
            checked={settings.email_answers}
            onChange={(e) =>
              setSettings({ ...settings, email_answers: e.target.checked })
            }
            className="h-4 w-4 text-blue-600 rounded"
          />
        </div>

        {/* Email for comments */}
        <div className="flex items-center justify-between">
          <label htmlFor="email-comments" className="text-sm text-gray-700">
            Email me when someone comments on my content
          </label>
          <input
            id="email-comments"
            type="checkbox"
            role="checkbox"
            aria-label="Email notifications for comments"
            checked={settings.email_comments}
            onChange={(e) =>
              setSettings({ ...settings, email_comments: e.target.checked })
            }
            className="h-4 w-4 text-blue-600 rounded"
          />
        </div>

        {/* Email for mentions */}
        <div className="flex items-center justify-between">
          <label htmlFor="email-mentions" className="text-sm text-gray-700">
            Email me when someone mentions me
          </label>
          <input
            id="email-mentions"
            type="checkbox"
            role="checkbox"
            aria-label="Email notifications for mentions"
            checked={settings.email_mentions}
            onChange={(e) =>
              setSettings({ ...settings, email_mentions: e.target.checked })
            }
            className="h-4 w-4 text-blue-600 rounded"
          />
        </div>

        {/* Digest frequency */}
        <div>
          <label htmlFor="digest-frequency" className="block text-sm font-medium text-gray-700 mb-1">
            Digest frequency
          </label>
          <select
            id="digest-frequency"
            value={settings.email_digest}
            onChange={(e) =>
              setSettings({
                ...settings,
                email_digest: e.target.value as NotificationSettings['email_digest'],
              })
            }
            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
          >
            <option value="never">Never</option>
            <option value="daily">Daily</option>
            <option value="weekly">Weekly</option>
          </select>
        </div>

        {/* Message */}
        {message && (
          <div
            className={`p-3 rounded-md ${
              message.type === 'success'
                ? 'bg-green-50 text-green-700'
                : 'bg-red-50 text-red-700'
            }`}
          >
            {message.text}
          </div>
        )}

        {/* Save button */}
        <button
          onClick={handleSave}
          disabled={isSaving}
          aria-busy={isSaving}
          className="px-4 py-2 bg-blue-600 text-white font-medium rounded-md hover:bg-blue-700 disabled:opacity-50"
        >
          {isSaving ? 'Saving...' : 'Save Preferences'}
        </button>
      </div>
    </div>
  );
}

/**
 * Main Settings Page component
 */
export default function SettingsPage() {
  const router = useRouter();
  const { user, isLoading: authLoading } = useAuth();
  const [activeTab, setActiveTab] = useState<TabType>('profile');

  // Redirect to login if not authenticated
  useEffect(() => {
    if (!authLoading && !user) {
      router.replace('/login');
    }
  }, [authLoading, user, router]);

  // Show loading skeleton while auth is loading
  if (authLoading) {
    return (
      <main className="container mx-auto px-4 py-8 max-w-4xl">
        <SettingsSkeleton />
      </main>
    );
  }

  // Don't render if not authenticated (redirect will happen)
  if (!user) {
    return null;
  }

  return (
    <main className="container mx-auto px-4 py-8 max-w-4xl">
      <h1 className="text-3xl font-bold text-gray-900 dark:text-white mb-8">
        Settings
      </h1>

      <TabNav activeTab={activeTab} onTabChange={setActiveTab} />

      {activeTab === 'profile' && (
        <ProfileForm
          userId={user.id}
          initialData={{
            display_name: user.display_name,
            bio: user.bio || '',
            avatar_url: user.avatar_url || '',
          }}
        />
      )}

      {activeTab === 'agents' && <AgentsTab userId={user.id} />}

      {activeTab === 'notifications' && <NotificationsTab userId={user.id} />}
    </main>
  );
}
