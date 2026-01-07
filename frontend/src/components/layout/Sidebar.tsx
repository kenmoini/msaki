"use client";

import { cn } from "@/lib/utils";
import type { Model } from "@/types";
import { Badge } from "@/components/ui/badge";

interface SidebarProps {
  models: Model[];
  selectedModel: string | null;
  onSelectModel: (name: string) => void;
  isLoading?: boolean;
}

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

export function Sidebar({
  models,
  selectedModel,
  onSelectModel,
  isLoading,
}: SidebarProps) {
  return (
    <aside className="w-64 border-r border-gray-200 bg-gray-50 flex flex-col">
      <div className="p-4 border-b border-gray-200">
        <h2 className="text-sm font-semibold text-gray-900">Models</h2>
      </div>

      <div className="flex-1 overflow-y-auto p-2">
        {isLoading ? (
          <div className="space-y-2">
            {[...Array(3)].map((_, i) => (
              <div
                key={i}
                className="h-16 bg-gray-200 rounded-md animate-pulse"
              />
            ))}
          </div>
        ) : models.length === 0 ? (
          <p className="text-sm text-gray-500 p-2">No models available</p>
        ) : (
          <div className="space-y-1">
            {models.map((model) => (
              <button
                key={model.name}
                onClick={() => onSelectModel(model.name)}
                className={cn(
                  "w-full text-left p-3 rounded-md transition-colors",
                  selectedModel === model.name
                    ? "bg-blue-100 text-blue-900"
                    : "hover:bg-gray-100"
                )}
              >
                <div className="flex items-center justify-between">
                  <span className="font-medium text-sm truncate">
                    {model.name}
                  </span>
                  <Badge variant={getStatusBadgeVariant(model.status)}>
                    {model.status}
                  </Badge>
                </div>
                {model.description && (
                  <p className="text-xs text-gray-500 mt-1 truncate">
                    {model.description}
                  </p>
                )}
                {model.tags.length > 0 && (
                  <div className="flex flex-wrap gap-1 mt-2">
                    {model.tags.slice(0, 3).map((tag) => (
                      <span
                        key={tag}
                        className="text-xs bg-gray-200 text-gray-600 px-1.5 py-0.5 rounded"
                      >
                        {tag}
                      </span>
                    ))}
                    {model.tags.length > 3 && (
                      <span className="text-xs text-gray-400">
                        +{model.tags.length - 3}
                      </span>
                    )}
                  </div>
                )}
              </button>
            ))}
          </div>
        )}
      </div>
    </aside>
  );
}
