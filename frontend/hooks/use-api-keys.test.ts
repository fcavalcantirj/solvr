import { renderHook, act, waitFor } from "@testing-library/react";
import { useApiKeys } from "./use-api-keys";
import { api, APIKey, APICreateKeyResponse, APIRegenerateKeyResponse } from "@/lib/api";

// Mock the API module
jest.mock("@/lib/api", () => ({
  api: {
    getApiKeys: jest.fn(),
    createApiKey: jest.fn(),
    revokeApiKey: jest.fn(),
    regenerateApiKey: jest.fn(),
  },
}));

const mockApi = api as jest.Mocked<typeof api>;

describe("useApiKeys", () => {
  const mockKeys: APIKey[] = [
    {
      id: "key_1",
      name: "Production",
      key_prefix: "solvr_sk_...abc",
      last_used_at: "2026-02-05T10:00:00Z",
      created_at: "2026-02-01T10:00:00Z",
    },
    {
      id: "key_2",
      name: "Development",
      key_prefix: "solvr_sk_...xyz",
      last_used_at: null,
      created_at: "2026-02-03T10:00:00Z",
    },
  ];

  beforeEach(() => {
    jest.clearAllMocks();
  });

  describe("fetchKeys", () => {
    it("loads API keys on fetch", async () => {
      mockApi.getApiKeys.mockResolvedValue({ data: mockKeys });

      const { result } = renderHook(() => useApiKeys());

      expect(result.current.isLoading).toBe(true);
      expect(result.current.keys).toEqual([]);

      await act(async () => {
        await result.current.fetchKeys();
      });

      expect(result.current.isLoading).toBe(false);
      expect(result.current.keys).toEqual(mockKeys);
      expect(result.current.error).toBeNull();
    });

    it("handles fetch error", async () => {
      mockApi.getApiKeys.mockRejectedValue(new Error("Network error"));

      const { result } = renderHook(() => useApiKeys());

      await act(async () => {
        await result.current.fetchKeys();
      });

      expect(result.current.isLoading).toBe(false);
      expect(result.current.keys).toEqual([]);
      expect(result.current.error).toBe("Failed to load API keys");
    });
  });

  describe("createKey", () => {
    it("creates a new API key and returns the full key", async () => {
      const newKeyResponse: APICreateKeyResponse = {
        data: {
          id: "key_3",
          name: "New Key",
          key: "solvr_sk_full_secret_key_here",
          key_prefix: "solvr_sk_...new",
          created_at: "2026-02-05T12:00:00Z",
        },
      };

      mockApi.getApiKeys.mockResolvedValue({ data: mockKeys });
      mockApi.createApiKey.mockResolvedValue(newKeyResponse);

      const { result } = renderHook(() => useApiKeys());

      // First fetch existing keys
      await act(async () => {
        await result.current.fetchKeys();
      });

      // Create new key
      let fullKey: string | null = null;
      await act(async () => {
        fullKey = await result.current.createKey("New Key");
      });

      expect(fullKey).toBe("solvr_sk_full_secret_key_here");
      expect(mockApi.createApiKey).toHaveBeenCalledWith("New Key");
      // Keys should be refetched after creation
      expect(mockApi.getApiKeys).toHaveBeenCalledTimes(2);
    });

    it("handles create error", async () => {
      mockApi.getApiKeys.mockResolvedValue({ data: mockKeys });
      mockApi.createApiKey.mockRejectedValue(new Error("Limit reached"));

      const { result } = renderHook(() => useApiKeys());

      await act(async () => {
        await result.current.fetchKeys();
      });

      let fullKey: string | null = null;
      await act(async () => {
        fullKey = await result.current.createKey("New Key");
      });

      expect(fullKey).toBeNull();
      expect(result.current.error).toBe("Failed to create API key");
    });
  });

  describe("revokeKey", () => {
    it("revokes an API key", async () => {
      mockApi.getApiKeys.mockResolvedValue({ data: mockKeys });
      mockApi.revokeApiKey.mockResolvedValue(undefined);

      const { result } = renderHook(() => useApiKeys());

      await act(async () => {
        await result.current.fetchKeys();
      });

      let success = false;
      await act(async () => {
        success = await result.current.revokeKey("key_1");
      });

      expect(success).toBe(true);
      expect(mockApi.revokeApiKey).toHaveBeenCalledWith("key_1");
      // Keys should be refetched after revocation
      expect(mockApi.getApiKeys).toHaveBeenCalledTimes(2);
    });

    it("handles revoke error", async () => {
      mockApi.getApiKeys.mockResolvedValue({ data: mockKeys });
      mockApi.revokeApiKey.mockRejectedValue(new Error("Not found"));

      const { result } = renderHook(() => useApiKeys());

      await act(async () => {
        await result.current.fetchKeys();
      });

      let success = false;
      await act(async () => {
        success = await result.current.revokeKey("key_1");
      });

      expect(success).toBe(false);
      expect(result.current.error).toBe("Failed to revoke API key");
    });
  });

  describe("regenerateKey", () => {
    it("regenerates an API key and returns the new full key", async () => {
      const regenerateResponse: APIRegenerateKeyResponse = {
        data: {
          id: "key_1",
          key: "solvr_sk_new_regenerated_key",
          key_prefix: "solvr_sk_...reg",
        },
      };

      mockApi.getApiKeys.mockResolvedValue({ data: mockKeys });
      mockApi.regenerateApiKey.mockResolvedValue(regenerateResponse);

      const { result } = renderHook(() => useApiKeys());

      await act(async () => {
        await result.current.fetchKeys();
      });

      let newKey: string | null = null;
      await act(async () => {
        newKey = await result.current.regenerateKey("key_1");
      });

      expect(newKey).toBe("solvr_sk_new_regenerated_key");
      expect(mockApi.regenerateApiKey).toHaveBeenCalledWith("key_1");
      // Keys should be refetched after regeneration
      expect(mockApi.getApiKeys).toHaveBeenCalledTimes(2);
    });

    it("handles regenerate error", async () => {
      mockApi.getApiKeys.mockResolvedValue({ data: mockKeys });
      mockApi.regenerateApiKey.mockRejectedValue(new Error("Not found"));

      const { result } = renderHook(() => useApiKeys());

      await act(async () => {
        await result.current.fetchKeys();
      });

      let newKey: string | null = null;
      await act(async () => {
        newKey = await result.current.regenerateKey("key_1");
      });

      expect(newKey).toBeNull();
      expect(result.current.error).toBe("Failed to regenerate API key");
    });
  });

  describe("clearError", () => {
    it("clears the error state", async () => {
      mockApi.getApiKeys.mockRejectedValue(new Error("Network error"));

      const { result } = renderHook(() => useApiKeys());

      await act(async () => {
        await result.current.fetchKeys();
      });

      expect(result.current.error).toBe("Failed to load API keys");

      act(() => {
        result.current.clearError();
      });

      expect(result.current.error).toBeNull();
    });
  });
});
