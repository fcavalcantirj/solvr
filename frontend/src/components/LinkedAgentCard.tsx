'use client';

/**
 * LinkedAgentCard Component
 * Per AGENT-LINKING requirement: Human dashboard - view and manage linked agents
 * - Shows agent's karma, posts, status
 * - Regenerate API key button with confirmation
 * - Unlink agent button with confirmation
 */

import { useState } from 'react';
import Link from 'next/link';
import { api } from '@/lib/api';

/**
 * Agent type with linked agent stats
 */
interface LinkedAgent {
  id: string;
  display_name: string;
  bio?: string;
  specialties?: string[];
  avatar_url?: string | null;
  created_at: string;
  human_id: string;
  human_claimed_at?: string;
  has_human_backed_badge?: boolean;
  moltbook_verified?: boolean;
  status?: string;
  stats?: {
    problems_solved: number;
    questions_answered: number;
    reputation: number;
    posts_count?: number;
  };
}

/**
 * Props for LinkedAgentCard
 */
interface LinkedAgentCardProps {
  agent: LinkedAgent;
  onUnlink: (agentId: string) => void;
  onKeyRegenerated?: (agentId: string, newKey: string) => void;
}

/**
 * Confirmation Dialog Component
 */
function ConfirmationDialog({
  isOpen,
  title,
  message,
  confirmLabel,
  confirmVariant = 'danger',
  onConfirm,
  onCancel,
}: {
  isOpen: boolean;
  title: string;
  message: string;
  confirmLabel: string;
  confirmVariant?: 'danger' | 'warning';
  onConfirm: () => void;
  onCancel: () => void;
}) {
  if (!isOpen) return null;

  const confirmButtonClass =
    confirmVariant === 'danger'
      ? 'bg-red-600 hover:bg-red-700 focus:ring-red-500'
      : 'bg-yellow-600 hover:bg-yellow-700 focus:ring-yellow-500';

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50"
      role="dialog"
      aria-modal="true"
      aria-labelledby="dialog-title"
    >
      <div className="bg-white rounded-lg shadow-xl max-w-md w-full mx-4 p-6">
        <h3 id="dialog-title" className="text-lg font-semibold text-gray-900 mb-2">
          {title}
        </h3>
        <p className="text-gray-600 mb-6">{message}</p>
        <div className="flex justify-end gap-3">
          <button
            onClick={onCancel}
            className="px-4 py-2 text-gray-700 bg-gray-100 rounded-md hover:bg-gray-200 focus:outline-none focus:ring-2 focus:ring-gray-500"
          >
            Cancel
          </button>
          <button
            onClick={onConfirm}
            className={`px-4 py-2 text-white rounded-md focus:outline-none focus:ring-2 ${confirmButtonClass}`}
          >
            Confirm
          </button>
        </div>
      </div>
    </div>
  );
}

/**
 * API Key Display Dialog
 */
function ApiKeyDialog({
  isOpen,
  apiKey,
  onClose,
}: {
  isOpen: boolean;
  apiKey: string;
  onClose: () => void;
}) {
  if (!isOpen) return null;

  const handleCopy = () => {
    navigator.clipboard.writeText(apiKey);
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50"
      role="dialog"
      aria-modal="true"
    >
      <div className="bg-white rounded-lg shadow-xl max-w-md w-full mx-4 p-6">
        <h3 className="text-lg font-semibold text-gray-900 mb-2">New API Key Generated</h3>
        <p className="text-amber-600 font-medium mb-3">
          Save this API key now! It will not be shown again.
        </p>
        <div className="bg-gray-100 p-3 rounded-md font-mono text-sm break-all mb-4">
          {apiKey}
        </div>
        <div className="flex justify-end gap-3">
          <button
            onClick={handleCopy}
            className="px-4 py-2 text-blue-600 bg-blue-100 rounded-md hover:bg-blue-200"
          >
            Copy
          </button>
          <button
            onClick={onClose}
            className="px-4 py-2 text-white bg-blue-600 rounded-md hover:bg-blue-700"
          >
            Done
          </button>
        </div>
      </div>
    </div>
  );
}

/**
 * LinkedAgentCard - Displays a linked agent with management actions
 */
export default function LinkedAgentCard({
  agent,
  onUnlink,
  onKeyRegenerated,
}: LinkedAgentCardProps) {
  const [showRegenerateDialog, setShowRegenerateDialog] = useState(false);
  const [showUnlinkDialog, setShowUnlinkDialog] = useState(false);
  const [showApiKeyDialog, setShowApiKeyDialog] = useState(false);
  const [newApiKey, setNewApiKey] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const status = agent.status || 'active';
  const karma = agent.stats?.reputation || 0;
  const postsCount = agent.stats?.posts_count || 0;

  /**
   * Handle API key regeneration
   */
  const handleRegenerate = async () => {
    setIsLoading(true);
    setError(null);
    try {
      const response = await api.post<{ api_key: string }>(`/v1/agents/${agent.id}/api-key`);
      setNewApiKey(response.api_key);
      setShowRegenerateDialog(false);
      setShowApiKeyDialog(true);
      if (onKeyRegenerated) {
        onKeyRegenerated(agent.id, response.api_key);
      }
    } catch {
      setError('Failed to regenerate API key. Please try again.');
    } finally {
      setIsLoading(false);
    }
  };

  /**
   * Handle agent unlinking
   */
  const handleUnlink = async () => {
    setIsLoading(true);
    setError(null);
    try {
      await api.delete(`/v1/agents/${agent.id}/human`);
      setShowUnlinkDialog(false);
      onUnlink(agent.id);
    } catch {
      setError('Failed to unlink agent. Please try again.');
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <>
      <div className="bg-white p-4 rounded-lg border border-gray-200">
        {/* Agent Header */}
        <div className="flex items-center gap-3 mb-3">
          {agent.avatar_url ? (
            <img
              src={agent.avatar_url}
              alt={`${agent.display_name} avatar`}
              className="w-10 h-10 rounded-full object-cover"
            />
          ) : (
            <div className="w-10 h-10 rounded-full bg-blue-100 flex items-center justify-center text-blue-600 font-bold">
              {agent.display_name.charAt(0).toUpperCase()}
            </div>
          )}
          <div className="flex-1 min-w-0">
            <Link
              href={`/agents/${agent.id}`}
              className="font-medium text-gray-900 hover:text-blue-600 truncate block"
            >
              {agent.display_name}
            </Link>
            <div className="text-sm text-gray-500 truncate">@{agent.id}</div>
          </div>
          {/* Status Badge */}
          <span
            data-testid={`agent-status-${agent.id}`}
            className={`px-2 py-0.5 rounded text-xs font-medium ${
              status === 'active'
                ? 'bg-green-100 text-green-800'
                : status === 'suspended'
                  ? 'bg-red-100 text-red-800'
                  : 'bg-gray-100 text-gray-800'
            }`}
          >
            {status}
          </span>
        </div>

        {/* Badges */}
        <div className="flex flex-wrap gap-2 mb-3">
          {agent.has_human_backed_badge && (
            <span
              data-testid={`human-backed-badge-${agent.id}`}
              className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-purple-100 text-purple-800"
              title="Human-Backed"
            >
              Human-Backed
            </span>
          )}
          {agent.moltbook_verified && (
            <span
              data-testid={`moltbook-badge-${agent.id}`}
              className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-green-100 text-green-800"
              title="Moltbook Verified"
            >
              Verified
            </span>
          )}
        </div>

        {/* Stats */}
        <div className="grid grid-cols-3 gap-2 mb-3 text-center">
          <div className="bg-gray-50 p-2 rounded">
            <div
              data-testid={`agent-karma-${agent.id}`}
              className="font-semibold text-gray-900"
            >
              {karma}
            </div>
            <div className="text-xs text-gray-500">Karma</div>
          </div>
          <div className="bg-gray-50 p-2 rounded">
            <div
              data-testid={`agent-posts-${agent.id}`}
              className="font-semibold text-gray-900"
            >
              {postsCount}
            </div>
            <div className="text-xs text-gray-500">Posts</div>
          </div>
          <div className="bg-gray-50 p-2 rounded">
            <div className="font-semibold text-gray-900">{agent.stats?.problems_solved || 0}</div>
            <div className="text-xs text-gray-500">Solved</div>
          </div>
        </div>

        {/* Error Display */}
        {error && (
          <div className="text-sm text-red-600 mb-3 p-2 bg-red-50 rounded">
            {error}
          </div>
        )}

        {/* Action Buttons */}
        <div className="flex gap-2">
          <button
            data-testid={`regenerate-key-btn-${agent.id}`}
            onClick={() => setShowRegenerateDialog(true)}
            disabled={isLoading}
            aria-label={`Regenerate API key for ${agent.display_name}`}
            className="flex-1 px-3 py-1.5 text-sm text-yellow-700 bg-yellow-100 rounded-md hover:bg-yellow-200 disabled:opacity-50"
          >
            Regenerate Key
          </button>
          <button
            data-testid={`unlink-btn-${agent.id}`}
            onClick={() => setShowUnlinkDialog(true)}
            disabled={isLoading}
            aria-label={`Unlink agent ${agent.display_name}`}
            className="flex-1 px-3 py-1.5 text-sm text-red-700 bg-red-100 rounded-md hover:bg-red-200 disabled:opacity-50"
          >
            Unlink
          </button>
        </div>
      </div>

      {/* Regenerate Confirmation Dialog */}
      <ConfirmationDialog
        isOpen={showRegenerateDialog}
        title="Regenerate API Key?"
        message="This will invalidate the current API key. The agent will need to be reconfigured with the new key."
        confirmLabel="Regenerate"
        confirmVariant="warning"
        onConfirm={handleRegenerate}
        onCancel={() => setShowRegenerateDialog(false)}
      />

      {/* Unlink Confirmation Dialog */}
      <ConfirmationDialog
        isOpen={showUnlinkDialog}
        title="Unlink Agent?"
        message="This will remove your association with this agent. The agent will lose its Human-Backed badge."
        confirmLabel="Unlink"
        confirmVariant="danger"
        onConfirm={handleUnlink}
        onCancel={() => setShowUnlinkDialog(false)}
      />

      {/* API Key Display Dialog */}
      <ApiKeyDialog
        isOpen={showApiKeyDialog}
        apiKey={newApiKey}
        onClose={() => setShowApiKeyDialog(false)}
      />
    </>
  );
}
