# Contributing to Velo

---

## Branch Names

```
<type>/<short-description>
```

| Type | When to use |
|------|-------------|
| `feat` | New feature |
| `fix` | Bug fix |
| `chore` | Tooling, config, dependencies |
| `refactor` | Code change with no behavior change |
| `test` | Adding or fixing tests |
| `docs` | Documentation only |

**Examples:**
```
feat/clip-alignment-algorithm
fix/apns-token-refresh
chore/docker-compose-setup
refactor/session-repository
docs/api-endpoints
```

Rules:
- Lowercase, hyphens only (no underscores, no spaces)
- Keep it short and readable — describe the work, not the ticket
- Branch off `main`; delete after merge

---

## Commits

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <short description>
```

**Examples:**
```
feat(reel): implement clip alignment algorithm
fix(auth): handle expired refresh token on 401
chore(deps): upgrade pgx to v5.7
test(session): add fixture for late joiner alignment
```

- Present tense, lowercase, no period at the end
- Keep the subject line under 72 characters
- Add a body if the why isn't obvious from the subject

---

## Pull Requests

### Title

Same format as commits:
```
feat(camera): hold-to-record with duration enforcement
fix(worker): retry reel job with exponential backoff
```

### Body Template

```markdown
## What
Brief description of what this PR does and what the end state is.

## Why
Why this change is needed. What problem does it solve?

## How
Non-obvious implementation details, tradeoffs, or approach decisions.
Delete this section if the change is straightforward.

## Testing
How you verified this works. Be specific enough that a reviewer can judge
whether the coverage is adequate.

- [ ] Unit tests added or updated
- [ ] Tested manually — describe steps and device/environment
- [ ] Edge cases considered (list any intentionally not covered)

## Screenshots
Before/after if this touches UI. Delete if backend-only.

## Checklist
- [ ] Self-reviewed the diff before requesting review
- [ ] No debug logs, dead code, or commented-out blocks left in
- [ ] Migration has a working down file
- [ ] No secrets or hardcoded credentials in the diff
```

### Rules

- **1 approval required** before merge (see GitHub setup below)
- **No direct pushes to `main`**
- **Merge strategy: merge commit** — use "Merge pull request" (not squash, not rebase) to preserve individual commit history
- Keep PRs focused — one logical change per PR
- All CI checks must pass before merge
- Resolve all review comments before merging
- Delete your branch after merge

---

## Code Review

**As an author:**
- Review your own diff before requesting review
- Provide context in the PR body — don't make reviewers guess
- Respond to all comments, even if just acknowledging

**As a reviewer:**
- Approve if the code is correct and safe to ship, even if you'd do it differently
- Block (`Request changes`) only for correctness issues, security concerns, or clear architectural problems
- Distinguish blocking feedback from suggestions — prefix non-blocking comments with `nit:` or `optional:`
