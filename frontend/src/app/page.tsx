"use client";

import { useState, useEffect, useCallback } from "react";
import { Header } from "@/components/layout/Header";
import { Sidebar } from "@/components/layout/Sidebar";
import { ChatContainer } from "@/components/chat/ChatContainer";
import { api } from "@/lib/api";
import type { Model, User } from "@/types";

export default function Home() {
  const [user, setUser] = useState<User | null>(null);
  const [models, setModels] = useState<Model[]>([]);
  const [selectedModelName, setSelectedModelName] = useState<string | null>(null);
  const [isLoadingModels, setIsLoadingModels] = useState(true);

  const selectedModel = models.find((m) => m.name === selectedModelName) || null;

  const loadModels = useCallback(async () => {
    try {
      setIsLoadingModels(true);
      const data = await api.getModels();
      setModels(data);
    } catch (error) {
      console.error("Failed to load models:", error);
      // For development, set some mock data
      setModels([
        {
          name: "gpt-oss-120b",
          description: "GPT OSS 120b via vLLM",
          aliases: ["gpt-oss"],
          tags: ["general", "chat", "gpt"],
          status: "stopped",
          hasStartScript: true,
          hasStopScript: true,
        },
        {
          name: "ext-openapi",
          description: "External proxy to OpenAI",
          aliases: ["openai", "chatgpt"],
          tags: ["general", "chat", "external"],
          status: "running",
          endpoint: "https://api.openai.com/v1",
          hasStartScript: false,
          hasStopScript: false,
        },
      ]);
    } finally {
      setIsLoadingModels(false);
    }
  }, []);

  const loadUser = useCallback(async () => {
    try {
      const userData = await api.getCurrentUser();
      setUser(userData);
    } catch {
      // Not logged in
      setUser(null);
    }
  }, []);

  useEffect(() => {
    loadUser();
    loadModels();
  }, [loadUser, loadModels]);

  const handleLogout = async () => {
    try {
      await api.logout();
      setUser(null);
    } catch (error) {
      console.error("Logout failed:", error);
    }
  };

  return (
    <div className="flex flex-col h-screen">
      <Header user={user} onLogout={handleLogout} />
      <div className="flex flex-1 overflow-hidden">
        <Sidebar
          models={models}
          selectedModel={selectedModelName}
          onSelectModel={setSelectedModelName}
          isLoading={isLoadingModels}
        />
        <main className="flex-1 overflow-hidden">
          <ChatContainer selectedModel={selectedModel} />
        </main>
      </div>
    </div>
  );
}
