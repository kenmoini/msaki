"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";

interface HeaderProps {
  user?: { username: string; role: string } | null;
  onLogout?: () => void;
}

export function Header({ user, onLogout }: HeaderProps) {
  const pathname = usePathname();

  const navItems = [
    { href: "/", label: "Chat" },
    { href: "/models/", label: "Models" },
  ];

  return (
    <header className="border-b border-gray-200 bg-white">
      <div className="flex h-16 items-center justify-between px-4">
        <div className="flex items-center gap-8">
          <Link href="/" className="flex items-center gap-2">
            <span className="text-xl font-bold text-gray-900">MSAKI</span>
          </Link>

          <nav className="hidden md:flex items-center gap-6">
            {navItems.map((item) => (
              <Link
                key={item.href}
                href={item.href}
                className={cn(
                  "text-sm font-medium transition-colors hover:text-gray-900",
                  pathname === item.href
                    ? "text-gray-900"
                    : "text-gray-500"
                )}
              >
                {item.label}
              </Link>
            ))}
          </nav>
        </div>

        <div className="flex items-center gap-4">
          {user ? (
            <>
              <span className="text-sm text-gray-600">
                {user.username}
                <span className="ml-1 text-xs text-gray-400">
                  ({user.role})
                </span>
              </span>
              <Button variant="ghost" size="sm" onClick={onLogout}>
                Logout
              </Button>
            </>
          ) : (
            <Link href="/login/">
              <Button variant="primary" size="sm">
                Login
              </Button>
            </Link>
          )}
        </div>
      </div>
    </header>
  );
}
