"use client";

import { Sun, Moon, Monitor } from "lucide-react";
import { useTheme } from "./ThemeProvider";
import { useState, useRef, useEffect } from "react";

export function ThemeToggle() {
  const { theme, setTheme, resolvedTheme } = useTheme();
  const [isOpen, setIsOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  // Close menu when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };

    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  const options = [
    { value: "light" as const, label: "Light", icon: Sun },
    { value: "dark" as const, label: "Dark", icon: Moon },
    { value: "system" as const, label: "System", icon: Monitor },
  ];

  const CurrentIcon = resolvedTheme === "dark" ? Moon : Sun;

  return (
    <div className="relative" ref={menuRef}>
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="rounded-lg p-2 text-gray-600 transition-colors hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-800"
        aria-label="Toggle theme"
      >
        <CurrentIcon className="h-5 w-5" />
      </button>

      {isOpen && (
        <div className="absolute right-0 z-50 mt-2 w-36 rounded-lg border border-gray-200 bg-white py-1 shadow-lg dark:border-gray-700 dark:bg-gray-800">
          {options.map((option) => {
            const Icon = option.icon;
            const isSelected = theme === option.value;

            return (
              <button
                key={option.value}
                onClick={() => {
                  setTheme(option.value);
                  setIsOpen(false);
                }}
                className={`flex w-full items-center gap-2 px-3 py-2 text-sm transition-colors ${
                  isSelected
                    ? "bg-blue-50 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300"
                    : "text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-700"
                }`}
              >
                <Icon className="h-4 w-4" />
                {option.label}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
