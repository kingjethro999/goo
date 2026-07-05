---
name: authentication-authorization
description: Use this skill whenever Goo is asked to add login, signup, sessions, password handling, tokens, or access control to a project. Covers password hashing, session vs token strategy, anonymous-to-authenticated upgrade flows, and encryption at rest. Trigger on "add login", "add auth", "protect this route", "hash passwords", "add sessions", or any file touching credentials or tokens.
---

# Authentication & Authorization Skill

## When to use
Any task involving login, signup, sessions, tokens, password handling, or route protection.

## Non-negotiables (never skip these regardless of deadline pressure)
- Passwords are hashed with Argon2id — never MD5/SHA1/plain SHA256, never stored reversibly.
- Secrets (API keys, signing keys) come from environment variables — never hardcoded, never committed.
- Session/JWT tokens travel over HTTPS only, and are `HttpOnly` + `Secure` when stored in cookies.
- Every protected route checks authorization (does this user own this resource), not just authentication (is this a logged-in user).

## Password hashing (Node.js)
```ts
import argon2 from "argon2";

export async function hashPassword(plain: string) {
  return argon2.hash(plain, { type: argon2.argon2id });
}

export async function verifyPassword(hash: string, plain: string) {
  return argon2.verify(hash, plain);
}
```

## Password hashing (Go) — consistent with Goo's own credential storage
```go
import "golang.org/x/crypto/argon2"

func HashPassword(password string, salt []byte) []byte {
    return argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
}
```

## Session strategy decision
| Situation | Recommended approach |
|---|---|
| Single web app, same domain | Server-side session with an `HttpOnly` cookie |
| API consumed by multiple clients (web + mobile + CLI) | Short-lived JWT access token + refresh token |
| CLI tool storing a provider API key locally | Encrypt at rest — this is not a session/JWT problem at all |

## Anonymous-to-authenticated upgrade flow
For products that let a user start anonymously and later create an account:
1. The anonymous session gets a random session ID stored client-side, tied to any generated data server-side.
2. On signup/login, look up the anonymous session ID and reassign its records to the new authenticated user ID in one transaction.
3. Invalidate the anonymous session ID after transfer so it can't be reused or collide with a future session.

## Encryption at rest (local secrets, config files, CLI tools)
AES-256-GCM with a key derived via Argon2id from a user-supplied passphrase — the same pattern already used for Goo's own encrypted storage, so reuse it rather than introducing a second encryption scheme in the same codebase.

```go
import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "io"
)

func Encrypt(plaintext, key []byte) ([]byte, error) {
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, err
    }
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return nil, err
    }
    return gcm.Seal(nonce, nonce, plaintext, nil), nil
}
```

## Authorization checklist
- [ ] Every route touching user data checks `resource.owner_id == session.user_id` (or a role-based equivalent) — not just "is a token present"
- [ ] IDs in URLs aren't sequential integers where guessing them exposes other users' data (use UUIDs, or check ownership regardless)
- [ ] Login and password-reset endpoints are rate-limited specifically — these are brute-force targets
- [ ] Password reset tokens are single-use and time-limited (15–60 minutes)

## Common pitfalls to avoid
- Storing JWTs in `localStorage` (readable by any injected script) instead of an `HttpOnly` cookie, when a cookie-compatible setup is available
- Rolling a custom crypto scheme instead of well-reviewed primitives (Argon2id, AES-GCM)
- Checking authentication but forgetting authorization — "logged in" is not "allowed to see this specific record"
- Returning different error messages for "wrong password" vs "no such user" (enables user enumeration)
