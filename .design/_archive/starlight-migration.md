# Design: Documentation Migration to Starlight

## 1. Overview
This document outlines the plan to migrate the existing Scion documentation (currently raw Markdown files in `docs/`) to a modern, structured documentation site using [Starlight](https://starlight.astro.build/) (an Astro-based documentation theme).

## 2. Goals
- **Better User Experience**: Improved navigation, search, and readability.
- **Maintainability**: Structured content organization and component-based architecture.
- **Extensibility**: Ability to add interactive components, tabs, and advanced formatting.
- **Search**: Built-in, fast client-side search (Pagefind).

## 3. Current State
The current documentation resides in the `docs/` directory at the project root.
Structure:
```text
docs/
├── concepts.md
├── install.md
├── overview.md
├── settings.md
├── guides/
│   ├── kubernetes.md
│   ├── templates.md
│   └── ...
└── reference/
    ├── cli.md
    └── scion-config-reference.md
```

Files use standard Markdown with a single H1 (`# Title`) at the top. They do not have YAML frontmatter.

## 4. Target Architecture

### 4.1. Directory Structure
A new directory `docs-site/` (or `website/`) will be created at the project root to house the Starlight application. This separates the buildable site from the main Go codebase while keeping docs co-located.

Proposed structure:
```text
docs-site/
├── package.json          # Dependencies (Astro, Starlight, etc.)
├── astro.config.mjs      # Starlight configuration (sidebar, title)
├── src/
│   ├── assets/           # Images (moved from docs/ if any, or new)
│   └── content/
│       └── docs/         # The new home for markdown files
│           ├── index.mdx # Landing page (custom)
│           ├── overview.md
│           ├── install.md
│           ├── concepts.md
│           ├── guides/
│           │   └── ...
│           └── reference/
│               └── ...
```

### 4.2. Content Migration Strategy

1.  **File Movement**: Copy all files from `docs/` to `docs-site/src/content/docs/`.
2.  **Frontmatter Addition**: Starlight requires frontmatter for the page title. We must parse the first H1 (`# Title`) of each file and convert it to YAML frontmatter.
    *   **Old**:
        ```markdown
        # Scion Concepts
        Content...
        ```
    *   **New**:
        ```markdown
        ---
        title: Scion Concepts
        description: (Optional) Extracted from first paragraph or left empty.
        ---
        Content... (H1 removed)
        ```
3.  **Landing Page**: Create `src/content/docs/index.mdx` with `template: splash` to serve as the homepage, linking to `overview` or `install`.

### 4.3. Navigation (Sidebar)
The sidebar in `astro.config.mjs` will be configured to group pages logically:
- **Start Here**: Overview, Install, Concepts
- **Guides**: Content from `guides/`
- **Reference**: Content from `reference/` (CLI, Configuration)

## 5. Implementation Plan

### Phase 1: Initialization
1.  Initialize a new Starlight project in `docs-site/`.
    ```bash
    npm create astro@latest -- --template starlight
    ```
2.  Configure `astro.config.mjs` with the project title ("Scion").

### Phase 2: Content Porting
1.  Scripted or manual transfer of files from `docs/` to `docs-site/src/content/docs/`.
2.  Add frontmatter `title` to all files.
3.  Fix internal links (e.g., `[Link](guides/worktrees.md)` might need adjustment depending on the relative path, but standard Markdown relative links usually work well in Starlight).

### Phase 3: Refinement
1.  **Sidebar**: Define the explicit order of sidebar items in `astro.config.mjs`.
2.  **Theming**: Adjust colors to match Scion's branding (if any).
3.  **Search**: Verify Pagefind works.

### Phase 4: Integration
1.  Add a CI job to build the docs (`npm run build`).
2.  Update `README.md` in the root to point to the new docs site.

## 6. Technical Considerations
- **Node.js**: Requires Node.js (v18.14.1+).
- **Package Manager**: `npm` is standard.
- **Images**: If images are added later, they should go in `src/assets/` and be referenced via relative paths or standard Markdown image syntax.

## 7. Open Questions
- **Deployment target**: GitHub Pages? Vercel? Netlify?
- **Versioning**: Do we need versioned docs? (Starlight supports this but it adds complexity). For now, we assume single-version (latest).
