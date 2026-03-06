// @ts-check
import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";

// https://astro.build/config
export default defineConfig({
  site: "https://code-search.techquests.dev",
  trailingSlash: "always",
  integrations: [
    starlight({
      title: "Code Search",
      description:
        "Self-hosted code search and bulk operations across all your repositories. Search, find, and replace code at scale with CLI and web UI.",
      logo: {
        light: "./src/assets/logo-light-no-bg.svg",
        dark: "./src/assets/logo-dark-no-bg.svg",
        replacesTitle: true,
      },
      // Theme-aware code blocks - GitHub Light for light mode, GitHub Dark for dark mode
      expressiveCode: {
        themes: ["github-light", "github-dark"],
        themeCssSelector: (theme) => {
          // Map theme names to data-theme attribute values
          if (theme.name === "github-light") {
            return '[data-theme="light"]';
          }
          return '[data-theme="dark"]';
        },
        styleOverrides: {
          borderRadius: "0.5rem",
          codeFontFamily: '"SF Mono", "Menlo", "Monaco", "Consolas", monospace',
          frames: {
            shadowColor: "transparent",
            frameBoxShadowCssValue: "none",
          },
        },
      },
      social: [
        {
          icon: "github",
          label: "GitHub",
          href: "https://github.com/techquestsdev/code-search",
        },
      ],
      editLink: {
        baseUrl:
          "https://github.com/techquestsdev/code-search/edit/main/website/",
      },
      head: [
        // Primary Meta Tags
        {
          tag: "meta",
          attrs: {
            name: "title",
            content:
              "Code Search - Self-hosted code search across repositories",
          },
        },
        {
          tag: "meta",
          attrs: {
            name: "description",
            content:
              "Self-hosted code search and bulk operations across all your repositories. Search, find, and replace code at scale with CLI and web UI.",
          },
        },
        {
          tag: "meta",
          attrs: {
            name: "keywords",
            content:
              "code search, self-hosted, source code, repository search, bulk replace, CLI, code indexing, developer tools, DevOps",
          },
        },
        {
          tag: "meta",
          attrs: { name: "author", content: "Code Search" },
        },
        {
          tag: "meta",
          attrs: { name: "robots", content: "index, follow" },
        },
        // OpenGraph / Facebook
        {
          tag: "meta",
          attrs: { property: "og:type", content: "website" },
        },
        {
          tag: "meta",
          attrs: {
            property: "og:url",
            content: "https://code-search.techquests.dev/",
          },
        },
        {
          tag: "meta",
          attrs: {
            property: "og:title",
            content:
              "Code Search - Self-hosted code search across repositories",
          },
        },
        {
          tag: "meta",
          attrs: {
            property: "og:description",
            content:
              "Self-hosted code search and bulk operations across all your repositories. Search, find, and replace code at scale.",
          },
        },
        {
          tag: "meta",
          attrs: {
            property: "og:image",
            content: "https://code-search.techquests.dev/og-image.png",
          },
        },
        {
          tag: "meta",
          attrs: { property: "og:image:width", content: "1200" },
        },
        {
          tag: "meta",
          attrs: { property: "og:image:height", content: "630" },
        },
        {
          tag: "meta",
          attrs: { property: "og:site_name", content: "Code Search" },
        },
        {
          tag: "meta",
          attrs: { property: "og:locale", content: "en_US" },
        },
        // Twitter
        {
          tag: "meta",
          attrs: { name: "twitter:card", content: "summary_large_image" },
        },
        {
          tag: "meta",
          attrs: {
            name: "twitter:url",
            content: "https://code-search.techquests.dev/",
          },
        },
        {
          tag: "meta",
          attrs: {
            name: "twitter:title",
            content:
              "Code Search - Self-hosted code search across repositories",
          },
        },
        {
          tag: "meta",
          attrs: {
            name: "twitter:description",
            content:
              "Self-hosted code search and bulk operations across all your repositories. Search, find, and replace code at scale.",
          },
        },
        {
          tag: "meta",
          attrs: {
            name: "twitter:image",
            content: "https://code-search.techquests.dev/og-image.png",
          },
        },
        // Theme color - slate-800 matching the icon background
        {
          tag: "meta",
          attrs: { name: "theme-color", content: "#1e293b" },
        },
        // Canonical URL
        {
          tag: "link",
          attrs: {
            rel: "canonical",
            href: "https://code-search.techquests.dev/",
          },
        },
        // Favicon
        {
          tag: "link",
          attrs: { rel: "icon", type: "image/svg+xml", href: "/favicon.svg" },
        },
        // Apple Touch Icon
        {
          tag: "link",
          attrs: { rel: "apple-touch-icon", href: "/apple-touch-icon.svg" },
        },
        // JSON-LD Structured Data
        {
          tag: "script",
          attrs: { type: "application/ld+json" },
          content: JSON.stringify({
            "@context": "https://schema.org",
            "@type": "SoftwareApplication",
            name: "Code Search",
            description:
              "Self-hosted code search and bulk operations across all your repositories",
            url: "https://code-search.techquests.dev",
            applicationCategory: "DeveloperApplication",
            operatingSystem: "Linux, macOS, Windows",
            offers: {
              "@type": "Offer",
              price: "0",
              priceCurrency: "USD",
            },
            author: {
              "@type": "Organization",
              name: "Code Search",
            },
          }),
        },
      ],
      customCss: [
        "./src/styles/custom.css",
        "./src/styles/kubernetes-icon.css",
      ],
      sidebar: [
        {
          label: "Getting Started",
          items: [
            { label: "Introduction", slug: "getting-started/introduction" },
            { label: "Installation", slug: "getting-started/installation" },
            { label: "Quick Start", slug: "getting-started/quick-start" },
            { label: "Contact Us", slug: "contact" },
          ],
        },
        {
          label: "Deployment",
          items: [
            { label: "Docker", slug: "deployment/docker" },
            { label: "Docker Compose", slug: "deployment/docker-compose" },
            { label: "Kubernetes", slug: "deployment/kubernetes" },
            { label: "Helm Chart", slug: "deployment/helm" },
          ],
        },
        {
          label: "Configuration",
          items: [
            { label: "Overview", slug: "configuration/overview" },
            { label: "Server", slug: "configuration/server" },
            { label: "Database", slug: "configuration/database" },
            { label: "Redis", slug: "configuration/redis" },
            { label: "Zoekt", slug: "configuration/zoekt" },
            { label: "Indexer", slug: "configuration/indexer" },
            { label: "Scheduler", slug: "configuration/scheduler" },
            { label: "Repositories", slug: "configuration/repos" },
            { label: "Sharding", slug: "configuration/sharding" },
            { label: "Webhooks", slug: "configuration/webhooks" },
            { label: "Observability", slug: "configuration/observability" },
            { label: "Secrets", slug: "configuration/secrets" },
            {
              label: "Environment Variables",
              slug: "configuration/environment-variables",
            },
          ],
        },
        {
          label: "Code Hosts",
          items: [
            { label: "GitHub", slug: "code-hosts/github" },
            {
              label: "GitHub Enterprise",
              slug: "code-hosts/github-enterprise",
            },
            { label: "GitLab", slug: "code-hosts/gitlab" },
            {
              label: "GitLab Self-Hosted",
              slug: "code-hosts/gitlab-self-hosted",
            },
            { label: "Gitea", slug: "code-hosts/gitea" },
            { label: "Bitbucket", slug: "code-hosts/bitbucket" },
          ],
        },
        {
          label: "CLI",
          items: [
            { label: "Installation", slug: "cli/installation" },
            { label: "Configuration", slug: "cli/configuration" },
            { label: "Search", slug: "cli/search" },
            { label: "Replace", slug: "cli/replace" },
            { label: "Find", slug: "cli/find" },
            { label: "Repository Management", slug: "cli/repo-management" },
          ],
        },
        {
          label: "Web UI",
          items: [
            { label: "Overview", slug: "web-ui/overview" },
            { label: "Search", slug: "web-ui/search" },
            { label: "Query Syntax", slug: "web-ui/query-syntax" },
            { label: "Connections", slug: "web-ui/connections" },
            { label: "Repositories", slug: "web-ui/repositories" },
            { label: "Replace", slug: "web-ui/replace" },
            { label: "Jobs", slug: "web-ui/jobs" },
          ],
        },
        {
          label: "API Reference",
          items: [
            { label: "Overview", slug: "api/overview" },
            { label: "Search", slug: "api/search" },
            { label: "Repositories", slug: "api/repositories" },
            { label: "Connections", slug: "api/connections" },
            { label: "Replace", slug: "api/replace" },
            { label: "SCIP", slug: "api/scip" },
            { label: "MCP Server", slug: "api/mcp" },
            { label: "Jobs", slug: "api/jobs" },
          ],
        },
        {
          label: "Pricing",
          items: [
            { label: "Plans", slug: "pricing" },
          ],
        },
        {
          label: "Enterprise",
          items: [
            { label: "Overview", slug: "enterprise/overview" },
            { label: "Administration Guide", slug: "enterprise/admin-guide" },
          ],
        },
        {
          label: "Architecture",
          items: [
            { label: "Overview", slug: "architecture/overview" },
            { label: "Components", slug: "architecture/components" },
            { label: "Data Flow", slug: "architecture/data-flow" },
            { label: "Indexing", slug: "architecture/indexing" },
          ],
        },
        {
          label: "Development",
          items: [
            { label: "Setup", slug: "development/setup" },
            { label: "Contributing", slug: "development/contributing" },
            { label: "Building", slug: "development/building" },
          ],
        },
      ],
    }),
  ],
});
