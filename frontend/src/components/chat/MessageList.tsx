"use client";

import { useRef, useEffect } from "react";
import { cn } from "@/lib/utils";
import type { ChatMessage } from "@/types";

interface MessageListProps {
  messages: ChatMessage[];
  isStreaming?: boolean;
}

export function MessageList({ messages, isStreaming }: MessageListProps) {
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  return (
    <div className="flex-1 overflow-y-auto p-4 space-y-4">
      {messages.length === 0 ? (
        <div className="flex items-center justify-center h-full text-gray-400">
          <p>Select a model and start chatting</p>
        </div>
      ) : (
        messages.map((message) => (
          <div
            key={message.id}
            className={cn(
              "flex",
              message.role === "user" ? "justify-end" : "justify-start"
            )}
          >
            <div
              className={cn(
                "max-w-[80%] rounded-lg px-4 py-2",
                message.role === "user"
                  ? "bg-blue-600 text-white"
                  : "bg-gray-100 text-gray-900"
              )}
            >
              <div className="whitespace-pre-wrap break-words">
                {message.content}
              </div>
              {message.model && message.role === "assistant" && (
                <div className="text-xs mt-1 opacity-60">{message.model}</div>
              )}
            </div>
          </div>
        ))
      )}

      {isStreaming && (
        <div className="flex justify-start">
          <div className="bg-gray-100 rounded-lg px-4 py-2">
            <div className="flex space-x-1">
              <span className="w-2 h-2 bg-gray-400 rounded-full animate-bounce" />
              <span
                className="w-2 h-2 bg-gray-400 rounded-full animate-bounce"
                style={{ animationDelay: "0.1s" }}
              />
              <span
                className="w-2 h-2 bg-gray-400 rounded-full animate-bounce"
                style={{ animationDelay: "0.2s" }}
              />
            </div>
          </div>
        </div>
      )}

      <div ref={bottomRef} />
    </div>
  );
}
