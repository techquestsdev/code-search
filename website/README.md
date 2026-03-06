# Code Search Website

Project website and documentation built with [Astro](https://astro.build) and [Starlight](https://starlight.astro.build).

## Development

```bash
cd website
bun install
bun run dev     # Starts dev server at localhost:4321
```

## Build

```bash
bun run build   # Build to ./dist/
bun run preview # Preview the build locally
```

## Structure

```
src/
├── content/docs/   # Documentation pages (.md/.mdx)
├── components/     # Astro components
├── pages/          # Landing page
├── styles/         # Custom CSS
└── assets/         # Images
```

Documentation pages in `src/content/docs/` are automatically routed based on file path. Sidebar navigation is configured in `astro.config.mjs`.
