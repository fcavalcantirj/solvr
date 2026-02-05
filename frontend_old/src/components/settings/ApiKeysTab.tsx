'use client';

/**
 * API Keys Tab for Settings Page
 * Per prd-v2.json API-KEYS requirements:
 * - List existing keys with usage stats
 * - Create new key with name input
 * - Show key ONCE on creation (copy button)
 * - Revoke button with confirmation
 */

import { useState, useEffect, useCallback } from 'react';
import { api, ApiError } from '@/lib/api';

/**
 * API Key type matching backend response
 */
interface ApiKey {
  id: string;
  name: string;
  key_prefix: string;
  created_at: string;
  last_used_at: string | null;
  revoked_at: string | null;
}

/**
 * Response when creating a new API key
 */
interface CreateKeyResponse {
  id: string;
  name: string;
  api_key: string;
  created_at: string;
}

/**
 * Format date for display
 */
function formatDate(dateString: string): string {
  const date = new Date(dateString);
  return date.toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  });
}

/**
 * Loading skeleton for API keys list
 */
function LoadingSkeleton() {
  return (
    <div data-testid="api-keys-loading" className="animate-pulse space-y-4">
      <div className="h-16 bg-gray-200 rounded" />
      <div className="h-16 bg-gray-200 rounded" />
    </div>
  );
}

/**
 * New API key display component - shows key only once
 */
function NewKeyDisplay({
  apiKey,
  keyName,
  onDismiss,
}: {
  apiKey: string;
  keyName: string;
  onDismiss: () => void;
}) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(apiKey);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Fallback for browsers without clipboard API
      const textArea = document.createElement('textarea');
      textArea.value = apiKey;
      document.body.appendChild(textArea);
      textArea.select();
      document.execCommand('copy');
      document.body.removeChild(textArea);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  return (
    <div className="bg-green-50 border border-green-200 rounded-lg p-4 mb-6">
      <h3 className="text-lg font-medium text-green-800 mb-2">
        API Key Created: {keyName}
      </h3>
      <p className="text-sm text-green-700 mb-3">
        Save this key now - it won&apos;t be shown again.
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
 * Create API key form component
 */
function CreateKeyForm({
  onSuccess,
  onCancel,
}: {
  onSuccess: (response: CreateKeyResponse) => void;
  onCancel: () => void;
}) {
  const [name, setName] = useState('');
  const [isCreating, setIsCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async () => {
    setError(null);

    // Validate name
    if (!name.trim()) {
      setError('Name is required');
      return;
    }

    setIsCreating(true);
    try {
      const response = await api.post<CreateKeyResponse>('/v1/users/me/api-keys', {
        name: name.trim(),
      });
      onSuccess(response);
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message);
      } else {
        setError('Failed to create API key');
      }
    } finally {
      setIsCreating(false);
    }
  };

  return (
    <div className="border border-gray-200 rounded-lg p-4 mb-6">
      <h3 className="text-lg font-medium mb-4">Create New API Key</h3>

      <div className="space-y-4">
        <div>
          <label
            htmlFor="key-name"
            className="block text-sm font-medium text-gray-700 mb-1"
          >
            Key Name
          </label>
          <input
            id="key-name"
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="e.g., Development, Production, CI/CD"
            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
          />
          {error && <p className="mt-1 text-sm text-red-600">{error}</p>}
          <p className="mt-1 text-xs text-gray-500">
            A descriptive name to help you identify this key
          </p>
        </div>

        <div className="flex gap-2">
          <button
            onClick={handleSubmit}
            disabled={isCreating}
            className="px-4 py-2 bg-blue-600 text-white font-medium rounded-md hover:bg-blue-700 disabled:opacity-50"
          >
            {isCreating ? 'Creating...' : 'Create'}
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
 * Revoke confirmation dialog
 */
function RevokeConfirmDialog({
  keyName,
  onConfirm,
  onCancel,
}: {
  keyName: string;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  return (
    <div
      role="dialog"
      aria-modal="true"
      className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50"
    >
      <div className="bg-white rounded-lg shadow-xl max-w-md w-full mx-4 p-6">
        <h3 className="text-lg font-medium text-gray-900 mb-2">Revoke API Key</h3>
        <p className="text-gray-600 mb-4">
          Are you sure you want to revoke &quot;{keyName}&quot;? This action cannot be
          undone and any applications using this key will stop working immediately.
        </p>
        <div className="flex justify-end gap-2">
          <button
            onClick={onCancel}
            className="px-4 py-2 border border-gray-300 text-gray-700 font-medium rounded-md hover:bg-gray-50"
          >
            Cancel
          </button>
          <button
            onClick={onConfirm}
            className="px-4 py-2 bg-red-600 text-white font-medium rounded-md hover:bg-red-700"
          >
            Yes, Revoke
          </button>
        </div>
      </div>
    </div>
  );
}

/**
 * Single API key card component
 */
function ApiKeyCard({
  apiKey,
  onRevoke,
}: {
  apiKey: ApiKey;
  onRevoke: (id: string) => void;
}) {
  return (
    <div className="border border-gray-200 rounded-lg p-4 flex items-start justify-between">
      <div className="flex-1">
        <div className="flex items-center gap-2">
          <h3 className="font-medium text-gray-900">{apiKey.name}</h3>
        </div>
        <p className="text-sm font-mono text-gray-500 mt-1">{apiKey.key_prefix}</p>
        <div className="text-xs text-gray-500 mt-2 space-y-1">
          <p>Created: {formatDate(apiKey.created_at)}</p>
          <p>
            Last used:{' '}
            {apiKey.last_used_at ? formatDate(apiKey.last_used_at) : 'Never used'}
          </p>
        </div>
      </div>
      <button
        onClick={() => onRevoke(apiKey.id)}
        className="px-3 py-1.5 text-sm text-red-600 border border-red-300 rounded-md hover:bg-red-50"
      >
        Revoke
      </button>
    </div>
  );
}

/**
 * Main API Keys Tab component
 */
export default function ApiKeysTab({ userId }: { userId: string }) {
  const [apiKeys, setApiKeys] = useState<ApiKey[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [newKey, setNewKey] = useState<{ apiKey: string; name: string } | null>(null);
  const [revokeTarget, setRevokeTarget] = useState<ApiKey | null>(null);
  const [revokeError, setRevokeError] = useState<string | null>(null);

  // Fetch API keys
  const fetchKeys = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const data = await api.get<ApiKey[]>('/v1/users/me/api-keys');
      setApiKeys(data);
    } catch {
      setError('Failed to load API keys. Please try again.');
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchKeys();
  }, [fetchKeys]);

  // Handle successful key creation
  const handleCreateSuccess = (response: CreateKeyResponse) => {
    setNewKey({ apiKey: response.api_key, name: response.name });
    setShowCreateForm(false);
    // Add new key to list (with masked prefix since we don't have it from this response)
    setApiKeys((prev) => [
      ...prev,
      {
        id: response.id,
        name: response.name,
        key_prefix: response.api_key.slice(0, 15) + '...',
        created_at: response.created_at,
        last_used_at: null,
        revoked_at: null,
      },
    ]);
  };

  // Handle key revocation
  const handleRevoke = async () => {
    if (!revokeTarget) return;

    setRevokeError(null);
    try {
      await api.delete(`/v1/users/me/api-keys/${revokeTarget.id}`);
      setApiKeys((prev) => prev.filter((key) => key.id !== revokeTarget.id));
      setRevokeTarget(null);
    } catch (err) {
      if (err instanceof ApiError) {
        setRevokeError(err.message);
      } else {
        setRevokeError('Failed to revoke API key');
      }
    }
  };

  return (
    <div role="tabpanel">
      <h2 className="text-xl font-semibold text-gray-900 mb-6">API Keys</h2>

      {/* Error message */}
      {revokeError && (
        <div className="bg-red-50 border border-red-200 text-red-700 rounded-md p-3 mb-4">
          {revokeError}
          <button
            onClick={() => setRevokeError(null)}
            className="ml-2 text-red-800 hover:text-red-900"
          >
            &times;
          </button>
        </div>
      )}

      {/* New key display */}
      {newKey && (
        <NewKeyDisplay
          apiKey={newKey.apiKey}
          keyName={newKey.name}
          onDismiss={() => setNewKey(null)}
        />
      )}

      {/* Create form */}
      {showCreateForm && (
        <CreateKeyForm
          onSuccess={handleCreateSuccess}
          onCancel={() => setShowCreateForm(false)}
        />
      )}

      {/* Create button */}
      {!showCreateForm && !newKey && (
        <button
          onClick={() => setShowCreateForm(true)}
          className="mb-6 px-4 py-2 bg-blue-600 text-white font-medium rounded-md hover:bg-blue-700"
        >
          Create API Key
        </button>
      )}

      {/* Loading state */}
      {isLoading && <LoadingSkeleton />}

      {/* Error state */}
      {error && !isLoading && (
        <div className="text-center py-8">
          <p className="text-red-600 mb-4">{error}</p>
          <button
            onClick={fetchKeys}
            className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700"
          >
            Retry
          </button>
        </div>
      )}

      {/* Empty state */}
      {!isLoading && !error && apiKeys.length === 0 && (
        <div className="text-center py-8 text-gray-500">
          <p>No API keys yet.</p>
          <p className="text-sm mt-1">Create your first API key to get started.</p>
        </div>
      )}

      {/* API keys list */}
      {!isLoading && !error && apiKeys.length > 0 && (
        <div className="space-y-4">
          {apiKeys.map((key) => (
            <ApiKeyCard
              key={key.id}
              apiKey={key}
              onRevoke={() => setRevokeTarget(key)}
            />
          ))}
        </div>
      )}

      {/* Revoke confirmation dialog */}
      {revokeTarget && (
        <RevokeConfirmDialog
          keyName={revokeTarget.name}
          onConfirm={handleRevoke}
          onCancel={() => setRevokeTarget(null)}
        />
      )}
    </div>
  );
}
