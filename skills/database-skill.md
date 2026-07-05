---
name: database-design
description: Use this skill whenever Goo is asked to design a schema, write migrations, add queries, or work with Postgres/Supabase/Neon. Covers schema design, migrations, indexing, and safe query patterns. Trigger on "add a table", "write a migration", "design the schema", "query the database", or any .sql file or ORM model file.
---

# Database Skill

## When to use
Any task involving schema design, migrations, or queries.

## Stack defaults
- Postgres as the default relational database (via Supabase or Neon) unless the project already uses something else.
- Migrations are files, checked into the repo, applied in order — never hand-edit the production schema directly.

## Schema design checklist
- [ ] Every table has a primary key; prefer UUID (`gen_random_uuid()`) over sequential integers for anything user-facing or referenced in a URL
- [ ] Foreign keys have an explicit `ON DELETE` behavior (`CASCADE`, `RESTRICT`, or `SET NULL`) — don't leave it to the default and find out later
- [ ] Timestamps: `created_at timestamptz default now()`, and `updated_at` maintained by a trigger or the ORM, not by application code remembering to set it
- [ ] Every column has an explicit NOT NULL decision made, not left to chance

## Migration pattern
```sql
-- migrations/0007_add_analysis_history.sql
create table if not exists analysis_history (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null references users(id) on delete cascade,
    document_hash text not null,
    summary text not null,
    created_at timestamptz not null default now()
);

create index if not exists idx_analysis_history_user_id on analysis_history(user_id);
```
Migrations are additive and forward-only. Don't edit an already-applied migration file — write a new one, even to fix a mistake in a previous one.

## Query safety checklist
- [ ] Every query is parameterized — no string-concatenated SQL, regardless of how trusted the input source seems
- [ ] Queries that can return unbounded rows have a `LIMIT`
- [ ] Any query touching another user's data is scoped by `user_id` in the `WHERE` clause, not filtered after the fact in application code
- [ ] N+1 query patterns are caught before shipping — batch or join instead of looping a query per row

## Indexing guidance
Add an index when a column is used in a `WHERE`, `JOIN`, or `ORDER BY` on a table expected to grow past a few thousand rows. Don't index every column pre-emptively — each index has a write cost. Foreign key columns almost always want an index; Postgres doesn't create one automatically.

## Privacy-sensitive data (relevant to document-intelligence style projects)
For a "process transiently, never store" product model, don't add a table "just in case" for data the product explicitly promises not to retain. If a cache is needed for performance, expire it aggressively (minutes, not days) with an explicit TTL in the schema or config, not an assumed one.

## Common pitfalls to avoid
- Storing money as `float`/`double` instead of `numeric` or integer cents
- Adding a column to "fix" a slow query when an index was the actual fix
- Cascading deletes on a relationship meant to preserve history (deleting a user shouldn't silently delete their audit log)
- Running a migration directly against production without testing it against a staging copy first
