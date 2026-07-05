# Goo Built-in Agent Skills

Place your markdown skill files (`.md`) in this directory.

Each skill file should contain YAML frontmatter defining its name, description, and trigger slash command, followed by the prompt recipe:

```markdown
---
description: Perform code refactoring using Go best practices
trigger: /refactor
---
You are a master Go refactoring specialist...
```

When Goo is compiled and installed, any skills placed here can be embedded or included as default skills for the CLI.
