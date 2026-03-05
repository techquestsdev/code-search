import type { Metadata } from "next";
import "./globals.css";
import { Navigation } from "@/components/Navigation";
import { ThemeProvider } from "@/components/ThemeProvider";
import { ContextProvider } from "@/hooks/useContexts";
import { AuthProvider } from "@/hooks/useAuth";

export const metadata: Metadata = {
  title: "Code Search",
  description: "Search and replace code across repositories",
  icons: {
    icon: "/icon.svg",
  },
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en" className="h-full overflow-hidden" suppressHydrationWarning>
      <head suppressHydrationWarning>
        <script
          dangerouslySetInnerHTML={{
            __html: `
              (function() {
                const theme = localStorage.getItem('theme') || 'system';
                const systemDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
                const isDark = theme === 'dark' || (theme === 'system' && systemDark);
                if (isDark) {
                  document.documentElement.classList.add('dark');
                }
              })();
            `,
          }}
        />
      </head>
      <body className="h-full bg-white dark:bg-gray-950 overflow-hidden">
        <ThemeProvider>
          <AuthProvider>
            <ContextProvider>
              <div className="h-full flex flex-col overflow-hidden">
                <Navigation />
                <main className="flex-1 overflow-hidden">{children}</main>
              </div>
            </ContextProvider>
          </AuthProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}
