"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { Header } from "@/components/layout/Header";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from "@/components/ui/card";
import { api } from "@/lib/api";
import type { Model, User, LogEntry } from "@/types";

function getStatusBadgeVariant(status: string) {
  switch (status) {
    case "running":
      return "success";
    case "starting":
    case "stopping":
      return "warning";
    case "error":
      return "error";
    default:
      return "default";
  }
}

// Log viewer modal component
function LogViewer({
  modelName,
  isOpen,
  onClose,
  logs,
  status,
  statusError,
}: {
  modelName: string;
  isOpen: boolean;
  onClose: () => void;
  logs: LogEntry[];
  status: string;
  statusError?: string;
}) {
  const logsEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    logsEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [logs]);

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-white rounded-lg shadow-xl w-full max-w-3xl max-h-[80vh] flex flex-col m-4">
        <div className="flex items-center justify-between p-4 border-b">
          <div>
            <h2 className="text-lg font-semibold">Logs: {modelName}</h2>
            <div className="flex items-center gap-2 mt-1">
              <Badge variant={getStatusBadgeVariant(status)}>{status}</Badge>
              {statusError && (
                <span className="text-sm text-red-600">{statusError}</span>
              )}
            </div>
          </div>
          <button
            onClick={onClose}
            className="text-gray-500 hover:text-gray-700 text-xl font-bold"
          >
            &times;
          </button>
        </div>
        <div className="flex-1 overflow-auto p-4 bg-gray-900 font-mono text-sm">
          {logs.length === 0 ? (
            <div className="text-gray-500 italic">No logs available</div>
          ) : (
            logs.map((entry, i) => (
              <div
                key={i}
                className={`py-0.5 ${
                  entry.stream === "stderr"
                    ? "text-red-400"
                    : entry.stream === "system"
                    ? "text-blue-400"
                    : "text-green-400"
                }`}
              >
                <span className="text-gray-500 text-xs mr-2">
                  {new Date(entry.timestamp).toLocaleTimeString()}
                </span>
                <span className="text-gray-600 mr-2">[{entry.stream}]</span>
                {entry.message}
              </div>
            ))
          )}
          <div ref={logsEndRef} />
        </div>
        <div className="p-4 border-t flex justify-end">
          <Button variant="outline" onClick={onClose}>
            Close
          </Button>
        </div>
      </div>
    </div>
  );
}

export default function ModelsPage() {
  const [user, setUser] = useState<User | null>(null);
  const [models, setModels] = useState<Model[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [actionInProgress, setActionInProgress] = useState<string | null>(null);

  // Log viewer state
  const [logViewerModel, setLogViewerModel] = useState<string | null>(null);
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [logStatus, setLogStatus] = useState<string>("");
  const [logStatusError, setLogStatusError] = useState<string | undefined>();
  const stopStreamRef = useRef<(() => void) | null>(null);

  const loadModels = useCallback(async () => {
    try {
      setIsLoading(true);
      const data = await api.getModels();
      setModels(data);
    } catch (error) {
      console.error("Failed to load models:", error);
      // Mock data for development
      setModels([
        {
          name: "gpt-oss-120b",
          description: "GPT OSS 120b via vLLM",
          aliases: ["gpt-oss"],
          tags: ["general", "chat", "gpt", "120b", "vllm"],
          status: "stopped",
          hasStartScript: true,
          hasStopScript: true,
          healthy: false,
        },
        {
          name: "ext-openapi",
          description: "External proxy to OpenAI",
          aliases: ["openai", "chatgpt"],
          tags: ["general", "chat", "gpt", "external"],
          status: "running",
          endpoint: "https://api.openai.com/v1",
          hasStartScript: false,
          hasStopScript: false,
          healthy: true,
        },
        {
          name: "ext-ollama",
          description: "External Ollama server",
          aliases: ["ollama"],
          tags: ["external", "ollama", "chat"],
          status: "running",
          endpoint: "https://remote-ollama:11434",
          hasStartScript: false,
          hasStopScript: false,
          healthy: true,
        },
      ]);
    } finally {
      setIsLoading(false);
    }
  }, []);

  const loadUser = useCallback(async () => {
    try {
      const userData = await api.getCurrentUser();
      setUser(userData);
    } catch {
      setUser(null);
    }
  }, []);

  useEffect(() => {
    loadUser();
    loadModels();
  }, [loadUser, loadModels]);

  const handleStop = async (name: string) => {
    setActionInProgress(name);
    try {
      await api.stopModel(name);
      await loadModels();
    } catch (error) {
      console.error("Failed to stop model:", error);
    } finally {
      setActionInProgress(null);
    }
  };

  const handleRestart = async (name: string) => {
    setActionInProgress(name);
    openLogViewer(name);
    try {
      await api.restartModel(name);
      // Don't reload immediately - log viewer will show progress
    } catch (error) {
      console.error("Failed to restart model:", error);
      await loadModels();
    } finally {
      setActionInProgress(null);
    }
  };

  const openLogViewer = (name: string) => {
    // Stop any existing stream
    if (stopStreamRef.current) {
      stopStreamRef.current();
    }

    setLogViewerModel(name);
    setLogs([]);
    setLogStatus("");
    setLogStatusError(undefined);

    // Start streaming logs
    const stopStream = api.streamModelLogs(
      name,
      (entry) => {
        setLogs((prev) => [...prev, entry]);
      },
      (status, statusError) => {
        setLogStatus(status);
        setLogStatusError(statusError);
        // Reload models when status changes
        loadModels();
      },
      () => {
        // Stream done
        loadModels();
      }
    );

    stopStreamRef.current = stopStream;
  };

  const closeLogViewer = () => {
    if (stopStreamRef.current) {
      stopStreamRef.current();
      stopStreamRef.current = null;
    }
    setLogViewerModel(null);
    loadModels();
  };

  const handleStartWithLogs = async (name: string) => {
    setActionInProgress(name);
    openLogViewer(name);
    try {
      await api.startModel(name);
      // Don't reload immediately - log viewer will show progress
    } catch (error) {
      console.error("Failed to start model:", error);
      await loadModels();
    } finally {
      setActionInProgress(null);
    }
  };

  const handleLogout = async () => {
    try {
      await api.logout();
      setUser(null);
    } catch (error) {
      console.error("Logout failed:", error);
    }
  };

  const isAdmin = user?.role === "administrator";

  return (
    <div className="flex flex-col min-h-screen">
      <Header user={user} onLogout={handleLogout} />
      <main className="flex-1 p-6">
        <div className="max-w-7xl mx-auto">
          <div className="flex items-center justify-between mb-6">
            <h1 className="text-2xl font-bold text-gray-900">Models</h1>
            <Button variant="outline" onClick={loadModels} disabled={isLoading}>
              Refresh
            </Button>
          </div>

          {isLoading ? (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {[...Array(3)].map((_, i) => (
                <div
                  key={i}
                  className="h-48 bg-gray-200 rounded-lg animate-pulse"
                />
              ))}
            </div>
          ) : models.length === 0 ? (
            <div className="text-center py-12 text-gray-500">
              No models configured
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {models.map((model) => (
                <Card key={model.name}>
                  <CardHeader>
                    <div className="flex items-center justify-between">
                      <CardTitle className="text-lg">{model.name}</CardTitle>
                      <Badge variant={getStatusBadgeVariant(model.status)}>
                        {model.status}
                      </Badge>
                    </div>
                    <CardDescription>{model.description}</CardDescription>
                  </CardHeader>
                  <CardContent>
                    {model.aliases.length > 0 && (
                      <div className="mb-3">
                        <span className="text-xs text-gray-500">Aliases: </span>
                        <span className="text-sm text-gray-700">
                          {model.aliases.join(", ")}
                        </span>
                      </div>
                    )}
                    {model.endpoint && (
                      <div className="mb-3">
                        <span className="text-xs text-gray-500">Endpoint: </span>
                        <span className="text-sm text-gray-700 break-all">
                          {model.endpoint}
                        </span>
                      </div>
                    )}
                    <div className="flex flex-wrap gap-1">
                      {model.tags.map((tag) => (
                        <span
                          key={tag}
                          className="text-xs bg-gray-100 text-gray-600 px-2 py-1 rounded"
                        >
                          {tag}
                        </span>
                      ))}
                    </div>
                  </CardContent>
                  <CardFooter className="gap-2 flex-wrap">
                    {/* Start button - shown when stopped */}
                    {model.hasStartScript && model.status === "stopped" && (
                      <Button
                        size="sm"
                        onClick={() => handleStartWithLogs(model.name)}
                        disabled={actionInProgress === model.name || !isAdmin}
                      >
                        {actionInProgress === model.name ? "Starting..." : "Start"}
                      </Button>
                    )}
                    {/* Stop button - shown when running */}
                    {model.hasStopScript && model.status === "running" && (
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => handleStop(model.name)}
                        disabled={actionInProgress === model.name || !isAdmin}
                      >
                        {actionInProgress === model.name ? "Stopping..." : "Stop"}
                      </Button>
                    )}
                    {/* Restart button - shown when in error state */}
                    {model.hasStartScript && model.status === "error" && (
                      <Button
                        size="sm"
                        onClick={() => handleRestart(model.name)}
                        disabled={actionInProgress === model.name || !isAdmin}
                      >
                        {actionInProgress === model.name ? "Restarting..." : "Restart"}
                      </Button>
                    )}
                    {/* View Logs button - shown when there's a start script and not stopped */}
                    {model.hasStartScript && model.status !== "stopped" && (
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => openLogViewer(model.name)}
                      >
                        View Logs
                      </Button>
                    )}
                    {/* Error message */}
                    {model.status === "error" && model.statusError && (
                      <div className="w-full mt-2 text-sm text-red-600 bg-red-50 p-2 rounded">
                        {model.statusError}
                      </div>
                    )}
                    {/* External endpoint notice */}
                    {!model.hasStartScript && !model.hasStopScript && (
                      <span className="text-xs text-gray-400">
                        External endpoint (always available)
                      </span>
                    )}
                  </CardFooter>
                </Card>
              ))}
            </div>
          )}
        </div>
      </main>

      {/* Log Viewer Modal */}
      <LogViewer
        modelName={logViewerModel || ""}
        isOpen={logViewerModel !== null}
        onClose={closeLogViewer}
        logs={logs}
        status={logStatus}
        statusError={logStatusError}
      />
    </div>
  );
}
