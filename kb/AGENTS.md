# Preface

This is the root index file of the dispersed knowledge base (`kb/`). It serves as the main entry point to understand system architecture, navigate different subsystems, and maintain AI-friendly documentation conventions.

# Overview

This is the root `kb/AGENTS.md` file for this repository, and every relevant directory has its own `kb/` directory and `AGENTS.md` file. Specific information is found in other `kb/*.md` files with dashed low case names (e.g. `kb/special-relativity.md`).

The design of this structure has the following key goals:

- **Mechanical** - Agents are the main actors reading and writing the knowledge base.
- **Generic** - Benefits any agentic workflow, no matter the editor or platform.
- **Distilled** - Avoids the use of verbose task logs that pollute the context window.
- **Hierarchical** - Avoids excessive information in a single place that also pollutes the context window.
- **Human** - Information is readily available in a useful readable format.

# Important

- Read local `kb/AGENTS.md` files upon navigating directories.
- Keep the `kb/` files updated whenever there is something relevant to be documented.
- Follow the "Preface" and header conventions. The presence of headers other than "Preface" is optional and should be used only when needed. Omit optional empty or trivial headers everywhere instead of using placeholder text.

# Headers

Every header used across the `kb/*.md` files in this repository MUST be documented here to maintain semantic standardizations.

*   `Preface`: A brief introduction outlining the scope and relevance of a specific `.md` file, present precisely at the top of the file to aid quick AI parsing. **Required** in all `.md` files.
*   `Overview`: High-level summary of the directory, subsystem or knowledge base layout at large. Do not use this as an index of the directory contents (see `Index`).
*   `Important`: Essential directives outlining critical constraints, behaviors, or rules.
*   `Headers`: Global registry of header definitions, uniquely hosted at the root `kb/AGENTS.md`. Do NOT use this header in any other document.
*   `Architecture`: Structural design details or boundary explanations for a given component. Only use this for software architecture concepts, NOT for defining filesystem layouts.
*   `Directory`: Details describing the contents and structure of the current directory, and potentially nested directories. Use this instead of Architecture when discussing filesystem layouts. Format items as a list, starting with the file or directory name surrounded by backticks, a hyphen, and then its description (e.g. `- \`filename\` - Description`).
*   `Index`: A list exclusively indexing local `.md` files or nested `kb/AGENTS.md` child files. Only to be used in `AGENTS.md` index files and MUST be placed at the very end of the file. Other references, local or otherwise, are okay but must be inlined where they were naturally mentioned. Format items as a list, starting with the file or directory name surrounded by backticks, a hyphen, and then a brief description (e.g. `- \`filename\` - Description`).

# Index

- `../cmd/kb/AGENTS.md` - CLI entry points
- `../internal/kb/AGENTS.md` - Core internal packages
- `../public/kb/AGENTS.md` - Public API packages
- `../tests/kb/AGENTS.md` - Integration testing
- `../snap/kb/AGENTS.md` - Snap packaging
