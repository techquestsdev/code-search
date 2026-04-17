"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState, useEffect, useMemo } from "react";
import {
  Search,
  FolderGit2,
  RefreshCw,
  Link2,
  Zap,
  Menu,
  X,
} from "lucide-react";
import { CodeSearchIcon } from "./CodeSearchIcon";
import { ThemeToggle } from "./ThemeToggle";
import { api } from "@/lib/api";

const allNavItems = [
  { href: "/", label: "Search", icon: Search, settingKey: null },
  {
    href: "/replace",
    label: "Replace",
    icon: RefreshCw,
    settingKey: "hide_replace_page" as const,
  },
  {
    href: "/repos",
    label: "Repositories",
    icon: FolderGit2,
    settingKey: "hide_repos_page" as const,
  },
  {
    href: "/connections",
    label: "Connections",
    icon: Link2,
    settingKey: "hide_connections_page" as const,
  },
  {
    href: "/jobs",
    label: "Jobs",
    icon: Zap,
    settingKey: "hide_jobs_page" as const,
  },
];

type HideSettings = {
  hide_repos_page: boolean;
  hide_connections_page: boolean;
  hide_jobs_page: boolean;
  hide_replace_page: boolean;
};

export function Navigation() {
  const pathname = usePathname();
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const [navSettings, setNavSettings] = useState<
    HideSettings & { loaded: boolean }
  >({
    hide_repos_page: true, // Default to hidden until settings load
    hide_connections_page: true,
    hide_jobs_page: true,
    hide_replace_page: true,
    loaded: false,
  });
  const { loaded: settingsLoaded, ...hideSettings } = navSettings;

  // Load UI settings on mount
  useEffect(() => {
    api
      .getUISettings()
      .then((settings) => {
        setNavSettings({
          hide_repos_page: settings.hide_repos_page,
          hide_connections_page: settings.hide_connections_page,
          hide_jobs_page: settings.hide_jobs_page,
          hide_replace_page: settings.hide_replace_page,
          loaded: true,
        });
      })
      .catch(() => {
        // Show all pages if settings fail to load
        setNavSettings({
          hide_repos_page: false,
          hide_connections_page: false,
          hide_jobs_page: false,
          hide_replace_page: false,
          loaded: true,
        });
      });
  }, []);

  // Mark nav items visibility based on settings
  const navItems = useMemo(() => {
    return allNavItems.map((item) => ({
      ...item,
      visible: !item.settingKey || !hideSettings[item.settingKey],
    }));
  }, [hideSettings]);

  // Filter visible items for iteration
  const visibleNavItems = useMemo(() => {
    return navItems.filter((item) => item.visible);
  }, [navItems]);

  const handleHomeClick = (e: React.MouseEvent) => {
    e.preventDefault();
    setMobileMenuOpen(false);
    // Navigate to home without any query params (clears search)
    // Use window.location for a full page load to reset all state
    window.location.href = "/";
  };

  const handleNavClick = () => {
    setMobileMenuOpen(false);
  };

  return (
    <nav className="sticky top-0 z-50 !w-screen border-b border-gray-200 bg-white dark:border-gray-800 dark:bg-gray-900">
      <div className="container mx-auto px-4">
        <div className="flex h-14 items-center justify-between sm:h-16">
          <Link
            href="/"
            onClick={handleHomeClick}
            className="flex cursor-pointer items-center gap-2"
          >
            <CodeSearchIcon className="h-8 w-8 text-blue-600 dark:text-blue-500 sm:h-9 sm:w-9" />
            <span className="text-lg font-bold sm:text-xl">Code Search</span>
          </Link>

          {/* Mobile menu button */}
          <button
            onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
            className="rounded-lg p-2 text-gray-600 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-800 sm:hidden"
          >
            {mobileMenuOpen ? (
              <X className="h-5 w-5" />
            ) : (
              <Menu className="h-5 w-5" />
            )}
          </button>

          {/* Desktop navigation */}
          <div
            className={`hidden items-center gap-1 transition-opacity duration-150 sm:flex ${settingsLoaded ? "opacity-100" : "opacity-0"}`}
          >
            {visibleNavItems.map((item) => {
              const isActive = pathname === item.href;
              const Icon = item.icon;

              // For the Search nav item, use the same clear behavior
              if (item.href === "/") {
                return (
                  <Link
                    key={item.href}
                    href="/"
                    onClick={handleHomeClick}
                    className={`flex cursor-pointer items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium transition-colors lg:px-4 ${
                      isActive
                        ? "bg-blue-50 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300"
                        : "text-gray-600 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-800"
                    }`}
                  >
                    <Icon className="h-4 w-4" />
                    <span className="hidden md:inline">{item.label}</span>
                  </Link>
                );
              }

              return (
                <Link
                  key={item.href}
                  href={item.href}
                  className={`flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium transition-colors lg:px-4 ${
                    isActive
                      ? "bg-blue-50 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300"
                      : "text-gray-600 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-800"
                  }`}
                >
                  <Icon className="h-4 w-4" />
                  <span className="hidden md:inline">{item.label}</span>
                </Link>
              );
            })}
            <div className="ml-2 flex items-center gap-2 border-l border-gray-200 pl-2 dark:border-gray-700">
              <ThemeToggle />
            </div>
          </div>
        </div>

        {/* Mobile navigation menu */}
        {mobileMenuOpen && settingsLoaded && (
          <div className="border-t border-gray-100 pb-4 dark:border-gray-800 sm:hidden">
            <div className="flex flex-col gap-1 pt-2">
              {visibleNavItems.map((item) => {
                const isActive = pathname === item.href;
                const Icon = item.icon;

                if (item.href === "/") {
                  return (
                    <Link
                      key={item.href}
                      href="/"
                      onClick={handleHomeClick}
                      className={`flex cursor-pointer items-center gap-3 rounded-lg px-4 py-3 text-sm font-medium transition-colors ${
                        isActive
                          ? "bg-blue-50 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300"
                          : "text-gray-600 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-800"
                      }`}
                    >
                      <Icon className="h-5 w-5" />
                      {item.label}
                    </Link>
                  );
                }

                return (
                  <Link
                    key={item.href}
                    href={item.href}
                    onClick={handleNavClick}
                    className={`flex items-center gap-3 rounded-lg px-4 py-3 text-sm font-medium transition-colors ${
                      isActive
                        ? "bg-blue-50 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300"
                        : "text-gray-600 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-800"
                    }`}
                  >
                    <Icon className="h-5 w-5" />
                    {item.label}
                  </Link>
                );
              })}
              <div className="mt-2 space-y-2 border-t border-gray-200 px-4 pt-2 dark:border-gray-700">
                <div className="flex items-center justify-between">
                  <span className="text-sm text-gray-600 dark:text-gray-400">
                    Theme
                  </span>
                  <ThemeToggle />
                </div>
              </div>
            </div>
          </div>
        )}
      </div>
    </nav>
  );
}
