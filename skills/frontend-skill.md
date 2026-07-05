---
name: frontend-development
description: Use this skill whenever Goo is asked to build, modify, or review frontend code — React/Next.js components, pages, styling, or client-side state. Covers component architecture, Tailwind CSS conventions, accessibility, responsive design, and design-token usage. Trigger on "build a component", "create a page", "style this", "make this responsive", "add a form", or any .tsx/.jsx/.css file work.
---

# Frontend Development Skill

## When to use
Any task that touches a UI: new components, pages, layouts, styling changes, or client-side interactivity.

## Stack defaults (use unless the project says otherwise)
- Framework: React with Next.js (App Router) unless the repo already uses the Pages Router — check for an existing `app/` vs `pages/` directory before picking.
- Styling: Tailwind CSS utility classes. Avoid inline `style={}` and ad hoc CSS files when Tailwind is already in use.
- State: local component state or React context for small-to-medium apps. Don't introduce a state library that isn't already in the project.

## Before writing any component
1. Look for an existing design-token file (colors, spacing, typography) and reuse it. Never hardcode hex colors, shadow values, or pixel spacing if tokens already exist.
2. Check whether components live in `components/`, `src/components/`, or another existing convention, and match it.
3. Assume a WCAG 2.2 AA accessibility bar unless told otherwise — that's the standard already in place across similar projects.

## Component checklist
- [ ] Semantic HTML (`button`, `nav`, `header`, `main`) instead of generic `div`s for interactive or structural elements
- [ ] Every interactive element is keyboard-reachable and has a visible focus state
- [ ] Color contrast meets WCAG AA (4.5:1 for body text) — check against the token palette, don't eyeball it
- [ ] Images have `alt` text; icon-only buttons have `aria-label`
- [ ] Loading and empty states are handled, not just the happy path
- [ ] Responsive at minimum three breakpoints: mobile (<640px), tablet (768–1024px), desktop (>1280px)

## Example: accessible, tokenized button component
```tsx
// components/ui/Button.tsx
import { type ButtonHTMLAttributes } from "react";

const variants = {
  primary: "bg-brand-600 text-white hover:bg-brand-700 focus-visible:ring-brand-500",
  secondary: "bg-surface-100 text-surface-900 hover:bg-surface-200 focus-visible:ring-surface-400",
} as const;

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: keyof typeof variants;
}

export function Button({ variant = "primary", className = "", ...props }: ButtonProps) {
  return (
    <button
      className={`rounded-lg px-4 py-2 font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 disabled:opacity-50 disabled:pointer-events-none ${variants[variant]} ${className}`}
      {...props}
    />
  );
}
```

## Data fetching pattern (Next.js)
- Server Components for read-only data by default.
- Client Components (`"use client"`) only where interactivity is actually required.
- Co-locate `loading.tsx` / `error.tsx` per route segment instead of hand-rolled spinners scattered through the tree.

## When the project has no design system yet
Don't invent an elaborate one. Define a minimal token set — five or six colors, a type scale, a spacing scale — in one file (`tailwind.config.ts` theme extension) before writing components, so the first ten components aren't already inconsistent with each other.

## Common pitfalls to avoid
- Shipping a desktop-only layout and "fixing responsive later"
- Adding a one-off color that isn't in the token file "just for this one thing"
- Using `<div onClick>` instead of `<button>` for anything clickable
- Skipping loading/error states in data-fetching components
- Using array index as a list `key` when the list order can change
