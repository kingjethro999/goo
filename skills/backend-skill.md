---
name: backend-development
description: Use this skill whenever Goo is asked to build or modify server-side code — API routes, services, background jobs, or integrations with third-party providers. Covers API design, error handling, and provider-integration patterns. Trigger on "build an API", "add an endpoint", "integrate [provider]", "write a service", or any server-side route/handler file.
---

# Backend Development Skill

## When to use
Any task touching a server: API routes, services, background jobs, or a third-party integration.

## Stack defaults
- Node.js projects: Express or Next.js Route Handlers, matching whatever the repo already uses — never introduce a second server framework into a project that already picked one.
- Go projects: `net/http` with a lightweight router (chi) unless something else is already wired up.
- AI provider: Groq is the default. Check for an existing `lib/groq.ts` / `internal/groq` client before writing a new one.

## API design checklist
- [ ] One consistent response envelope across all endpoints (e.g. `{ data, error }`), not ad hoc shapes per route
- [ ] Input is validated before touching the database or an external API — reject with 400 and a specific message, don't let bad input reach business logic
- [ ] Errors from external providers are caught and translated into a clean error shape — never let a raw provider stack trace reach the client
- [ ] A timeout on every external call
- [ ] Idempotency considered for anything that writes data and might be retried (webhooks especially)

## Provider integration pattern (Groq example)
```ts
// lib/groq.ts
import Groq from "groq-sdk";

const groq = new Groq({ apiKey: process.env.GROQ_API_KEY });

export async function completeChat(messages: Groq.Chat.ChatCompletionMessageParam[]) {
  try {
    const res = await groq.chat.completions.create({
      model: "llama-3.3-70b-versatile",
      messages,
      temperature: 0.3,
    });
    return { data: res.choices[0]?.message?.content ?? "", error: null };
  } catch (err) {
    console.error("groq completion failed", err);
    return { data: null, error: "AI completion failed, please retry" };
  }
}
```
Provider keys never reach client-side code — API routes only, never a client component.

## Error handling shape (reuse across every endpoint in a project)
```ts
type ApiResult<T> = { data: T; error: null } | { data: null; error: string };
```

## Go service handler pattern
```go
func (s *Server) handleCreateThing(w http.ResponseWriter, r *http.Request) {
    var req CreateThingRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }
    if err := req.Validate(); err != nil {
        writeError(w, http.StatusBadRequest, err.Error())
        return
    }
    thing, err := s.things.Create(r.Context(), req)
    if err != nil {
        s.logger.Error("create thing failed", "error", err)
        writeError(w, http.StatusInternalServerError, "could not create thing")
        return
    }
    writeJSON(w, http.StatusCreated, thing)
}
```

## Background / async work
For anything that shouldn't block the response (sending an email, a slow AI pipeline), return `202 Accepted` immediately and process it asynchronously, rather than making the client wait on a long request.

## Common pitfalls to avoid
- A different response shape for every endpoint instead of one reused envelope
- Trusting a client-supplied ID without checking the requester actually owns that resource
- Expensive work (AI calls, large queries) inside a request handler with no timeout, blocking the event loop or a goroutine indefinitely
- Logging full request bodies that might contain secrets or PII
- Business logic embedded directly in route handlers instead of a separate service layer — makes it untestable
