// Model types
export interface Model {
  name: string;
  description: string;
  aliases: string[];
  tags: string[];
  status: ModelStatus;
  statusError?: string;
  endpoint?: string;
  hasStartScript: boolean;
  hasStopScript: boolean;
  healthy: boolean;
  healthMessage?: string;
}

export type ModelStatus =
  | "stopped"
  | "starting"
  | "running"
  | "stopping"
  | "error";

export interface ModelHealth {
  healthy: boolean;
  message?: string;
}

// Log types
export interface LogEntry {
  timestamp: string;
  stream: "stdout" | "stderr" | "system";
  message: string;
}

export interface ModelLogsResponse {
  model: string;
  status: ModelStatus;
  logs: LogEntry[];
}

// Auth types
export interface User {
  username: string;
  role: "administrator" | "user";
}

export interface LoginRequest {
  username: string;
  password: string;
}

export interface LoginResponse {
  token: string;
  user: User;
}

// Chat types
export interface ChatMessage {
  id: string;
  role: "user" | "assistant" | "system";
  content: string;
  timestamp: string;
  model?: string;
}

export interface ChatRequest {
  model: string;
  messages: Array<{
    role: "user" | "assistant" | "system";
    content: string;
  }>;
  stream?: boolean;
}

export interface ChatResponse {
  id: string;
  choices: Array<{
    message: {
      role: string;
      content: string;
    };
    finish_reason: string;
  }>;
  model: string;
  usage?: {
    prompt_tokens: number;
    completion_tokens: number;
    total_tokens: number;
  };
}

// API response types
export interface ApiError {
  error: string;
  message?: string;
}

export interface ApiResponse<T> {
  data?: T;
  error?: ApiError;
}
