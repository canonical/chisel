# Preface

This repository follows strict conventions for organizing agent-oriented knowledge documents, and this document is required reading for any agent that wants to read or write these.

Read the top-level `.kb/agents.md` before continuing below.


# Overview

Every directory in this repository, including the root, may have its own `AGENTS.md` file and `.kb/` subdirectory. The AGENTS.md file provides a general view for the directory, while more specific information is found in `.kb/*.md` files with dashed lowercase names (e.g. `.kb/special-relativity.md`).

The design of this structure has the following key goals:

- **Mechanical** - Agents are the main actors reading and writing the knowledge base.
- **Generic** - Benefits any agentic workflow, no matter the editor or platform.
- **Distilled** - Avoids the use of verbose task or plan logs that pollute the context window.
- **Hierarchical** - Avoids excessive information in a single place that also pollutes the context window.
- **Human** - Information is readily available and reviewable in a friendly format.


# Important

- Read local `AGENTS.md` files upon navigating directories.
- Keep the `.kb/*.md` files updated whenever there is something relevant to be documented or updated.
- Follow the header conventions outlined below. Only the _Preface_ header is required, and the other headers should be omitted if empty or trivial.


# Headers

The following are the ONLY top-level headers allowed across the `.kb/*.md` files in this repository, to maintain semantic standardization across projects.
Sub-headers are okay.

- _Preface_ - A brief introduction outlining the scope and relevance of a specific `*.md` file. This section MUST be at the top of every `.kb/*.md` file so agents can easily grep for it, and the last line of this section MUST be "Read the top-level `.kb/agents.md` file before continuing below." so rules are followed.
- _Overview_ - High-level summary of the directory, subsystem or knowledge base layout at large. Do NOT use this to list files or directories.
- _Important_ - Essential directives for the agent outlining critical constraints, behaviors, or rules.
- _Headers_ - Global registry of header definitions, uniquely hosted at the root `.kb/agents.md` (this file). Do NOT use this header in any other document.
- _Architecture_ - Structural design details or boundary explanations for a given component. Only use this for software architecture concepts, NOT for directory outlining. Also avoid using this as a code reference (keep that in the code itself).
- _Directory_ - Only in `AGENTS.md` files, it briefly outlines the contents and structure of the directory the `AGENTS.md` file is in, and potentially nested small directories that do not justify their own `AGENTS.md` file. For `.kb/*.md` files, use _Documents_ instead.
- _Documents_ - Only in `AGENTS.md` files, it outlines the content of `.kb/*.md` files and also immediately nested `AGENTS.md` child files, to aid agent navigation. It MUST be the last section in the file. It's okay to also mention such filenames inline when they are relevant elsewhere in the text.

For the _Directory_ and _Documents_ listings, format items as a dashed list starting with the file or directory name surrounded by backticks, a dash, and then a brief description:
```
- `filename` - Terse summary.
```

