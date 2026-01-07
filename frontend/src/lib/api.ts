import type {
  Model,
  User,
  LoginRequest,
  LoginResponse,
  ChatRequest,
  ChatResponse,
  ApiError,
  LogEntry,
  ModelLogsResponse,
} from "@/types";

const API_BASE = "";

class ApiClient {
  private token: string | null = null;

  setToken(token: string | null) {
    this.token = token;
    if (token) {
      localStorage.setItem("msaki_token", token);
    } else {
      localStorage.removeItem("msaki_token");
    }
  }

  getToken(): string | null {
    if (!this.token) {
      this.token = localStorage.getItem("msaki_token");
    }
    return this.token;
  }

  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<T> {
    const headers: HeadersInit = {
      "Content-Type": "application/json",
      ...options.headers,
    };

    const token = this.getToken();
    if (token) {
      (headers as Record<string, string>)["Authorization"] = `Bearer ${token}`;
    }

    const response = await fetch(`${API_BASE}${endpoint}`, {
      ...options,
      headers,
    });

    if (!response.ok) {
      const error: ApiError = await response.json().catch(() => ({
        error: `HTTP ${response.status}`,
        message: response.statusText,
      }));
      throw new Error(error.message || error.error);
    }

    return response.json();
  }

  // Auth endpoints
  async login(credentials: LoginRequest): Promise<LoginResponse> {
    const response = await this.request<LoginResponse>("/api/auth/login", {
      method: "POST",
      body: JSON.stringify(credentials),
    });
    this.setToken(response.token);
    return response;
  }

  async logout(): Promise<void> {
    try {
      await this.request("/api/auth/logout", { method: "POST" });
    } finally {
      this.setToken(null);
    }
  }

  async getCurrentUser(): Promise<User> {
    return this.request<User>("/api/auth/me");
  }

  // Model endpoints
  async getModels(): Promise<Model[]> {
    return this.request<Model[]>("/api/models");
  }

  async getModel(name: string): Promise<Model> {
    return this.request<Model>(`/api/models/${encodeURIComponent(name)}`);
  }

  async startModel(name: string): Promise<void> {
    await this.request(`/api/models/${encodeURIComponent(name)}/start`, {
      method: "POST",
    });
  }

  async stopModel(name: string): Promise<void> {
    await this.request(`/api/models/${encodeURIComponent(name)}/stop`, {
      method: "POST",
    });
  }

  async restartModel(name: string): Promise<void> {
    await this.request(`/api/models/${encodeURIComponent(name)}/restart`, {
      method: "POST",
    });
  }

  async getModelLogs(name: string): Promise<ModelLogsResponse> {
    return this.request<ModelLogsResponse>(
      `/api/models/${encodeURIComponent(name)}/logs`
    );
  }

  streamModelLogs(
    name: string,
    onLog: (entry: LogEntry) => void,
    onStatus: (status: string, statusError?: string) => void,
    onDone: () => void
  ): () => void {
    const token = this.getToken();
    const url = `${API_BASE}/api/models/${encodeURIComponent(name)}/logs/stream`;

    const eventSource = new EventSource(url);

    // For auth, we need to use fetch with SSE parsing since EventSource doesn't support headers
    // Fall back to polling if auth is required
    if (token) {
      eventSource.close();
      return this.pollModelLogs(name, onLog, onStatus, onDone);
    }

    eventSource.addEventListener("log", (event) => {
      try {
        const entry = JSON.parse(event.data);
        onLog(entry);
      } catch {
        // Ignore parse errors
      }
    });

    eventSource.addEventListener("status", (event) => {
      try {
        const data = JSON.parse(event.data);
        onStatus(data.status, data.statusError);
      } catch {
        // Ignore parse errors
      }
    });

    eventSource.addEventListener("done", () => {
      eventSource.close();
      onDone();
    });

    eventSource.onerror = () => {
      eventSource.close();
      onDone();
    };

    return () => eventSource.close();
  }

  private pollModelLogs(
    name: string,
    onLog: (entry: LogEntry) => void,
    onStatus: (status: string, statusError?: string) => void,
    onDone: () => void
  ): () => void {
    let cancelled = false;
    let lastTimestamp = 0;
    let lastStatus = "";

    const poll = async () => {
      while (!cancelled) {
        try {
          const response = await this.getModelLogs(name);

          // Send new logs
          for (const entry of response.logs) {
            const ts = new Date(entry.timestamp).getTime();
            if (ts > lastTimestamp) {
              onLog(entry);
              lastTimestamp = ts;
            }
          }

          // Check status change
          if (response.status !== lastStatus) {
            onStatus(response.status);
            lastStatus = response.status;

            // Stop polling on terminal states
            if (
              response.status === "running" ||
              response.status === "stopped" ||
              response.status === "error"
            ) {
              onDone();
              return;
            }
          }
        } catch {
          // Ignore errors during polling
        }

        // Wait before next poll
        await new Promise((resolve) => setTimeout(resolve, 500));
      }
    };

    poll();

    return () => {
      cancelled = true;
    };
  }

  // Chat endpoints
  async chat(request: ChatRequest): Promise<ChatResponse> {
    return this.request<ChatResponse>("/v1/chat/completions", {
      method: "POST",
      body: JSON.stringify(request),
    });
  }

  async chatStream(
    request: ChatRequest,
    onChunk: (content: string) => void,
    onDone: () => void
  ): Promise<void> {
    const token = this.getToken();
    const headers: HeadersInit = {
      "Content-Type": "application/json",
    };
    if (token) {
      (headers as Record<string, string>)["Authorization"] = `Bearer ${token}`;
    }

    const response = await fetch(`${API_BASE}/v1/chat/completions`, {
      method: "POST",
      headers,
      body: JSON.stringify({ ...request, stream: true }),
    });

    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`);
    }

    const reader = response.body?.getReader();
    if (!reader) {
      throw new Error("No response body");
    }

    const decoder = new TextDecoder();
    let buffer = "";

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split("\n");
      buffer = lines.pop() || "";

      for (const line of lines) {
        if (line.startsWith("data: ")) {
          const data = line.slice(6);
          if (data === "[DONE]") {
            onDone();
            return;
          }
          try {
            const parsed = JSON.parse(data);
            const content = parsed.choices?.[0]?.delta?.content;
            if (content) {
              onChunk(content);
            }
          } catch {
            // Ignore parse errors for incomplete chunks
          }
        }
      }
    }
    onDone();
  }
}

export const api = new ApiClient();
