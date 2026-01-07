"use client";

import { useState, useEffect, useCallback } from "react";
import { Header } from "@/components/layout/Header";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from "@/components/ui/card";
import { api } from "@/lib/api";
import type { Model, User } from "@/types";

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

export default function ModelsPage() {
  const [user, setUser] = useState<User | null>(null);
  const [models, setModels] = useState<Model[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [actionInProgress, setActionInProgress] = useState<string | null>(null);

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

  const handleStart = async (name: string) => {
    setActionInProgress(name);
    try {
      await api.startModel(name);
      await loadModels();
    } catch (error) {
      console.error("Failed to start model:", error);
    } finally {
      setActionInProgress(null);
    }
  };

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
                  <CardFooter className="gap-2">
                    {model.hasStartScript && model.status === "stopped" && (
                      <Button
                        size="sm"
                        onClick={() => handleStart(model.name)}
                        disabled={actionInProgress === model.name || !isAdmin}
                      >
                        {actionInProgress === model.name ? "Starting..." : "Start"}
                      </Button>
                    )}
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
    </div>
  );
}
