## Working Style
- **Plan first**: For any task with 3+ steps or architectural decisions, plan before coding. If something goes sideways, stop and re-plan — don't push through.
- **Verify before done**: Never mark complete without proving it works. Run tests, check logs, show correctness.
- **Minimal change**: Fix bugs with the smallest change required unless a deeper architectural issue is clearly the root cause.
- **Elegance check**: For non-trivial changes, ask "is there a more elegant solution?" Skip for simple fixes.
- **When stuck**: If blocked, uncertain about direction, or facing an architectural decision → move to conflict resolution.

## Before Touching Code
Read the full file. Find related functions, tests, and references. Understand the existing pattern before introducing a new one. Don't assume APIs exist — search first.

## Conflict Resolution
If code and PRD conflict, an architectural decision arises, or you're stuck:
1. State the conflict or blocker clearly (one line each: what code implies, what PRD says).
2. State the trade-offs.
3. If the better path is obvious and low-risk: take it, then update the PRD.
4. If it touches scope, API contracts, or v1/v2 boundaries: stop and ask.

## Commits
Small, logical commits. Conventional prefixes: `feat:`, `fix:`, `refactor:`, `docs:`, `test:`, `chore:`. Only stage files related to the change. Use `git add <files>` and `git commit -m 'msg'`.

###  Conflict resolution protocol
If you encounter a conflict between what the code implies and what the PRD says, come across architectural decision or question, or if you need to make a design decision the PRD doesn't cover:
    a. Identify the conflict clearly (one sentence each: what code implies, what PRD says).
    b. State the trade-offs concisely.
    c. If the better path is obvious and low-risk: take it, implement it, then update the
       PRD to reflect the decision.
    d. If the trade-off is non-trivial or involves scope, API contract, or v1/v2 boundary:
       STOP and ask for approval before writing any code.

## Core Principles
- **Simplicity First**: Make every change as simple as possible. Impact minimal code.
- **No Laziness**: Find root causes. No temporary fixes. Principal Engineer, Senior Developer standards.
- **Minimal Impact**: Changes should only touch what's necessary. Avoid introducing bugs.
