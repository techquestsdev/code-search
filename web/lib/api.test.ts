import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// Mock fetch globally
const mockFetch = vi.fn();
global.fetch = mockFetch;

// Import after mocking
import { api } from "./api";

describe("API Client", () => {
  beforeEach(() => {
    mockFetch.mockReset();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  describe("search", () => {
    it("should make a POST request to /api/v1/search", async () => {
      const mockResponse = {
        results: [],
        total_count: 0,
        duration: "10ms",
        stats: { files_searched: 100, repos_searched: 5 },
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      });

      const result = await api.search({ query: "test" });

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/search"),
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({ query: "test" }),
        })
      );
      expect(result).toEqual(mockResponse);
    });

    it("should include all search options in request", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ results: [] }),
      });

      await api.search({
        query: "test",
        is_regex: true,
        case_sensitive: true,
        repos: ["repo1"],
        languages: ["Go"],
        file_patterns: ["*.go"],
        limit: 50,
        context_lines: 3,
      });

      expect(mockFetch).toHaveBeenCalledWith(
        expect.any(String),
        expect.objectContaining({
          body: JSON.stringify({
            query: "test",
            is_regex: true,
            case_sensitive: true,
            repos: ["repo1"],
            languages: ["Go"],
            file_patterns: ["*.go"],
            limit: 50,
            context_lines: 3,
          }),
        })
      );
    });

    it("should throw on API error", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 500,
        text: () => Promise.resolve("Internal Server Error"),
      });

      await expect(api.search({ query: "test" })).rejects.toThrow(
        "Internal Server Error"
      );
    });

    it("should parse JSON error messages", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 400,
        text: () =>
          Promise.resolve(JSON.stringify({ message: "Invalid query" })),
      });

      await expect(api.search({ query: "" })).rejects.toThrow("Invalid query");
    });
  });

  describe("listRepos", () => {
    it("should make a GET request to /api/v1/repos", async () => {
      const mockResponse = {
        repos: [],
        total_count: 0,
        limit: 50,
        offset: 0,
        has_more: false,
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      });

      const result = await api.listRepos();

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/repos"),
        expect.any(Object)
      );
      expect(result).toEqual(mockResponse);
    });

    it("should include query params", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ repos: [] }),
      });

      await api.listRepos({
        connectionId: 1,
        search: "test",
        status: "indexed",
        limit: 10,
      });

      const calledUrl = mockFetch.mock.calls[0][0] as string;
      expect(calledUrl).toContain("connection_id=1");
      expect(calledUrl).toContain("search=test");
      expect(calledUrl).toContain("status=indexed");
      expect(calledUrl).toContain("limit=10");
    });
  });

  describe("getRepo", () => {
    it("should make a GET request to /api/v1/repos/by-id/:id", async () => {
      const mockRepo = {
        id: 1,
        name: "test/repo",
        clone_url: "https://github.com/test/repo.git",
        branches: ["main"],
        status: "indexed",
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockRepo),
      });

      const result = await api.getRepo(1);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/repos/by-id/1"),
        expect.any(Object)
      );
      expect(result).toEqual(mockRepo);
    });
  });

  describe("listConnections", () => {
    it("should make a GET request to /api/v1/connections", async () => {
      const mockConnections = [
        { id: 1, name: "GitHub", type: "github", url: "https://github.com" },
      ];

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockConnections),
      });

      const result = await api.listConnections();

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/connections"),
        expect.any(Object)
      );
      expect(result).toEqual(mockConnections);
    });
  });

  describe("health", () => {
    it("should make a GET request to /health", async () => {
      const mockHealth = { status: "ok" };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockHealth),
      });

      const result = await api.health();

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/health"),
        expect.any(Object)
      );
      expect(result).toEqual(mockHealth);
    });
  });

  describe("file browsing", () => {
    it("should fetch tree entries", async () => {
      const mockTree = {
        entries: [
          { name: "src", type: "dir", path: "src" },
          { name: "main.go", type: "file", path: "main.go" },
        ],
        path: "",
        ref: "main",
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockTree),
      });

      const result = await api.getTree(1, "", "main");

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/repos/by-id/1/tree"),
        expect.any(Object)
      );
      expect(result.entries).toHaveLength(2);
    });

    it("should fetch blob content", async () => {
      const mockBlob = {
        content: "package main\n\nfunc main() {}",
        path: "main.go",
        size: 28,
        language: "Go",
        binary: false,
        ref: "main",
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockBlob),
      });

      const result = await api.getBlob(1, "main.go", "main");

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/repos/by-id/1/blob"),
        expect.any(Object)
      );
      expect(result.content).toContain("package main");
    });
  });

  describe("204 No Content", () => {
    it("should handle 204 responses", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 204,
      });

      // Some endpoints may return 204
      const result = await api.deleteRepo(1);
      expect(result).toEqual({});
    });
  });

  describe("getUISettings", () => {
    it("should fetch UI settings", async () => {
      const mockSettings = {
        hide_readonly_banner: false,
        hide_file_navigator: false,
        disable_browse_api: false,
        connections_readonly: false,
        repos_readonly: false,
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockSettings),
      });

      const result = await api.getUISettings();

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/ui/settings"),
        expect.any(Object)
      );
      expect(result).toEqual(mockSettings);
    });

    it("should handle disable_browse_api being true", async () => {
      const mockSettings = {
        hide_readonly_banner: false,
        hide_file_navigator: false,
        disable_browse_api: true,
        connections_readonly: false,
        repos_readonly: false,
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockSettings),
      });

      const result = await api.getUISettings();

      expect(result.disable_browse_api).toBe(true);
    });

    it("should handle readonly modes", async () => {
      const mockSettings = {
        hide_readonly_banner: true,
        hide_file_navigator: false,
        disable_browse_api: false,
        connections_readonly: true,
        repos_readonly: true,
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockSettings),
      });

      const result = await api.getUISettings();

      expect(result.repos_readonly).toBe(true);
      expect(result.connections_readonly).toBe(true);
      expect(result.hide_readonly_banner).toBe(true);
    });
  });

  describe("repo operations", () => {
    it("should add a repo", async () => {
      const mockRepo = {
        id: 1,
        name: "test/repo",
        url: "https://github.com/test/repo",
        status: "pending",
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockRepo),
      });

      const result = await api.addRepo({
        clone_url: "https://github.com/test/repo",
        name: "test/repo",
        connection_id: 1,
      });

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/repos"),
        expect.objectContaining({
          method: "POST",
        })
      );
      expect(result.name).toBe("test/repo");
    });

    it("should exclude a repo", async () => {
      const mockResponse = { message: "excluded", id: 1 };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      });

      const result = await api.excludeRepo(1);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/repos/by-id/1/exclude"),
        expect.objectContaining({
          method: "POST",
        })
      );
      expect(result.message).toBe("excluded");
    });

    it("should include a repo", async () => {
      const mockResponse = { message: "included", id: 1 };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      });

      const result = await api.includeRepo(1);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/repos/by-id/1/include"),
        expect.objectContaining({
          method: "POST",
        })
      );
      expect(result.message).toBe("included");
    });

    it("should sync a repo", async () => {
      const mockResponse = { message: "sync started", id: 1 };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      });

      const result = await api.syncRepo(1);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/repos/by-id/1/sync"),
        expect.objectContaining({
          method: "POST",
        })
      );
      expect(result.message).toBe("sync started");
    });

    it("should get repos status", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ readonly: true }),
      });

      const result = await api.getReposStatus();

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/repos/status"),
        expect.any(Object)
      );
      expect(result.readonly).toBe(true);
    });

    it("should lookup repo by name", async () => {
      const mockRepo = {
        id: 1,
        name: "github.com/test/repo",
        url: "https://github.com/test/repo",
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockRepo),
      });

      const result = await api.lookupRepoByName("github.com/test/repo");

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/repos/lookup"),
        expect.any(Object)
      );
      expect(result.name).toBe("github.com/test/repo");
    });
  });

  describe("connections", () => {
    it("should create a connection", async () => {
      const mockConnection = {
        id: 1,
        name: "test-connection",
        type: "github",
        url: "https://github.com",
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockConnection),
      });

      const result = await api.createConnection({
        name: "test-connection",
        type: "github",
        url: "https://github.com",
        token: "secret-token",
      });

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/connections"),
        expect.objectContaining({
          method: "POST",
        })
      );
      expect(result.name).toBe("test-connection");
    });

    it("should update a connection", async () => {
      const mockConnection = {
        id: 1,
        name: "updated-connection",
        type: "github",
        url: "https://github.com",
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockConnection),
      });

      const result = await api.updateConnection(1, {
        name: "updated-connection",
        url: "https://github.com",
        type: "github",
      });

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/connections/1"),
        expect.objectContaining({
          method: "PUT",
        })
      );
      expect(result.name).toBe("updated-connection");
    });

    it("should delete a connection", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 204,
      });

      await api.deleteConnection(1);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/connections/1"),
        expect.objectContaining({
          method: "DELETE",
        })
      );
    });

    it("should get connection by id", async () => {
      const mockConnection = {
        id: 1,
        name: "test-connection",
        type: "github",
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockConnection),
      });

      const result = await api.getConnection(1);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/connections/1"),
        expect.any(Object)
      );
      expect(result.id).toBe(1);
    });

    it("should test a connection", async () => {
      const mockResponse = {
        status: "success",
        message: "Connection test successful",
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      });

      const result = await api.testConnection(1);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/connections/1/test"),
        expect.objectContaining({
          method: "POST",
        })
      );
      expect(result.status).toBe("success");
    });

    it("should sync a connection", async () => {
      const mockResponse = {
        message: "Sync started",
        job_id: "123",
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      });

      const result = await api.syncConnection(1);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/connections/1/sync"),
        expect.objectContaining({
          method: "POST",
        })
      );
      expect(result.message).toBe("Sync started");
    });

    it("should get connections status", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ readonly: false }),
      });

      const result = await api.getConnectionsStatus();

      expect(result.readonly).toBe(false);
    });
  });

  describe("refs", () => {
    it("should get refs for a repo", async () => {
      const mockRefs = {
        default_branch: "main",
        branches: ["main", "develop"],
        tags: ["v1.0.0", "v1.1.0"],
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockRefs),
      });

      const result = await api.getRefs(1);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/repos/by-id/1/refs"),
        expect.any(Object)
      );
      expect(result.default_branch).toBe("main");
      expect(result.branches).toContain("develop");
      expect(result.tags).toContain("v1.0.0");
    });
  });

  describe("symbols", () => {
    it("should get file symbols", async () => {
      const mockResponse = {
        symbols: [
          {
            name: "MyClass",
            kind: "class",
            start_line: 1,
            end_line: 50,
            children: [],
          },
        ],
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      });

      const result = await api.getFileSymbols(1, "src/main.ts");

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/repos/by-id/1/symbols"),
        expect.any(Object)
      );
      expect(result.symbols[0].name).toBe("MyClass");
    });

    it("should find symbols across repos", async () => {
      const mockResponse = {
        results: [
          {
            name: "MyFunction",
            kind: "function",
            repo: "test/repo",
            file: "src/main.ts",
          },
        ],
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      });

      const _result = await api.findSymbols({
        name: "MyFunction",
        repos: ["test/repo"],
      });

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/symbols/find"),
        expect.objectContaining({
          method: "POST",
        })
      );
    });
  });

  describe("SCIP", () => {
    it("should get SCIP status for a repo", async () => {
      const mockStatus = {
        has_index: true,
        indexed_at: "2024-01-01T00:00:00Z",
        stats: {
          document_count: 100,
          symbol_count: 5000,
        },
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockStatus),
      });

      const result = await api.getSCIPStatus(1);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/scip/repos/1/status"),
        expect.any(Object)
      );
      expect(result.has_index).toBe(true);
    });

    it("should clear SCIP index", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ success: true, message: "cleared" }),
      });

      const result = await api.clearSCIPIndex(1);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/scip/repos/1/index"),
        expect.objectContaining({
          method: "DELETE",
        })
      );
      expect(result.success).toBe(true);
    });

    it("should get SCIP indexers", async () => {
      const mockIndexers = {
        indexers: [
          { language: "go", name: "scip-go" },
          { language: "typescript", name: "scip-typescript" },
        ],
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockIndexers),
      });

      const _result = await api.getSCIPIndexers();

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/scip/indexers"),
        expect.any(Object)
      );
    });
  });

  describe("find references", () => {
    it("should find symbol references", async () => {
      const mockResponse = [
        {
          symbol: "main",
          repo: "test/repo",
          file: "src/main.go",
          line: 10,
          content: "func main() {}",
        },
      ];

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      });

      const result = await api.findReferences({
        symbol: "main",
        repos: ["test/repo"],
      });

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/symbols/refs"),
        expect.objectContaining({
          method: "POST",
        })
      );
      expect(result).toHaveLength(1);
    });
  });
});

describe("Type exports", () => {
  it("should export all necessary types", async () => {
    // This test verifies the module can be imported
    // TypeScript compilation proves types exist
    expect(api).toBeDefined();
    expect(typeof api.search).toBe("function");
    expect(typeof api.listRepos).toBe("function");
    expect(typeof api.getTree).toBe("function");
    expect(typeof api.getBlob).toBe("function");
  });
});
