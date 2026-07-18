---
name: do-task
description: Use when picking up a challenger issue — receiving a new issue to elaborate, starting implementation, or preparing a PR. Enforces the two human gates (plan approval, merge).
---

# Do Task (challenger)

## Overview

The challenger workflow has **two human gates**: the user approves the plan (before any code) and the user merges the PR. Between them, work autonomously.

Conventions (git format, repo layout, DoD, API contract) live in `CLAUDE.md` — reference it, don't duplicate it here. This skill is the **process**.

## The gate that gets skipped

**No code before the plan is approved.** This is gate №1 and the easiest to violate under momentum.

Red flags — STOP if you catch yourself thinking:
- "The issue is obvious, I'll just start coding"
- "I'll write the plan and start implementing in parallel"
- "It's a tiny change, no plan needed"

All of these mean: write the plan (or ask questions), post it, wait for approval.

## Steps

1. **Input** — user hands over an issue (short description + their vision).

2. **Elaborate** (my zone) — board status → `Проработка`. Run `superpowers:brainstorming` on this specific piece, then produce **exactly one** of:
   - **Plan** — tech breakdown (tables / endpoints / files, edge-cases, Definition of Done). Post as an issue comment.
   - **Questions** — if the right solution is non-obvious, ask via `AskUserQuestion` on real forks *before* writing the plan. Don't guess on architecture / UX / DB-schema.

   After posting the plan or questions → board status `Аппрув / Нужна информация`.

3. **Gate №1 — plan approval.** Do not write code until approved.

4. **Implement** (my zone) — branch `CH-<issue>-<short-desc-en>` → board status `В работе`, then `superpowers:test-driven-development` (test → code → refactor). Commits `CH-<issue>: <short desc en>`.

5. **Self-review** (my zone) — `/code-review` on the diff and fix findings, linters clean, then `superpowers:verification-before-completion` (tests green, checked by hand).

6. **PR** (my zone) — open the PR → board status `Ревью`. Title `CH-<issue>: <short desc en>`, description in Russian (1–5 sentences), `Closes #N`.

7. **Gate №2 — merge.** Only the user merges (squash, delete branch).

8. **Deploy** — manual for now.

## Board status

Set the project card's status from the checklist above (not from memory):

```
make set-status ISSUE=<N> STATUS='Проработка'
make set-status ISSUE=<N> STATUS='Аппрув / Нужна информация'
make set-status ISSUE=<N> STATUS='В работе'
make set-status ISSUE=<N> STATUS='Ревью'
```

`Открыт` (issue added) and `Закрыт` (issue closed / PR merged) are set automatically by the project's built-in workflows — never set them by hand. See `README.md` → «Доска проекта».

## Notes

- Non-obvious decision that changes architecture/UX/data → **questions first**, then plan.
- DB-schema changes (`Challenge / Participant / Day / Idea`) — flag explicitly in the plan.
- Спайк №0 (`message_reaction`) is out-of-process: a half-day experiment, no PR ceremony; record the A/B decision in `/docs/decisions/`.
