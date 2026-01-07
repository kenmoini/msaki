"use client";

import { useState, useCallback } from "react";
import { MessageList } from "./MessageList";
import { MessageInput } from "./MessageInput";
import { api } from "@/lib/api";
import type { ChatMessage, Model } from "@/types";

// Generate a UUID with fallback for non-secure contexts
function generateId(): string {
  if (typeof crypto !== "undefined" && crypto.randomUUID) {
    return crypto.randomUUID();
  }
  // Fallback for non-secure contexts (HTTP)
  return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    const v = c === "x" ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}

interface ChatContainerProps {
  selectedModel: Model | null;
}

export function ChatContainer({ selectedModel }: ChatContainerProps) {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [isStreaming, setIsStreaming] = useState(false);

  const handleSend = useCallback(
    async (content: string) => {
      if (!selectedModel) return;

      const userMessage: ChatMessage = {
        id: generateId(),
        role: "user",
        content,
        timestamp: new Date().toISOString(),
      };

      setMessages((prev) => [...prev, userMessage]);
      setIsStreaming(true);

      try {
        const assistantMessageId = generateId();
        let assistantContent = "";

        // Add empty assistant message that we'll update
        setMessages((prev) => [
          ...prev,
          {
            id: assistantMessageId,
            role: "assistant",
            content: "",
            timestamp: new Date().toISOString(),
            model: selectedModel.name,
          },
        ]);

        await api.chatStream(
          {
            model: selectedModel.name,
            messages: [
              ...messages.map((m) => ({
                role: m.role as "user" | "assistant" | "system",
                content: m.content,
              })),
              { role: "user" as const, content },
            ],
            stream: true,
          },
          (chunk) => {
            assistantContent += chunk;
            setMessages((prev) =>
              prev.map((m) =>
                m.id === assistantMessageId
                  ? { ...m, content: assistantContent }
                  : m
              )
            );
          },
          () => {
            setIsStreaming(false);
          }
        );
      } catch (error) {
        console.error("Chat error:", error);
        setIsStreaming(false);

        // Add error message
        setMessages((prev) => [
          ...prev,
          {
            id: generateId(),
            role: "assistant",
            content: `Error: ${error instanceof Error ? error.message : "Failed to get response"}`,
            timestamp: new Date().toISOString(),
            model: selectedModel.name,
          },
        ]);
      }
    },
    [selectedModel, messages]
  );

  const isDisabled = !selectedModel || selectedModel.status !== "running";

  return (
    <div className="flex flex-col h-full">
      <MessageList messages={messages} isStreaming={isStreaming} />
      <MessageInput
        onSend={handleSend}
        disabled={isDisabled || isStreaming}
        placeholder={
          !selectedModel
            ? "Select a model to start chatting"
            : selectedModel.status !== "running"
              ? `Model "${selectedModel.name}" is ${selectedModel.status}`
              : "Type a message..."
        }
      />
    </div>
  );
}
