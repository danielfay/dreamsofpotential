# Do Issue Workflow

Plan and implement a GitHub issue on the current branch. Run `/setup-issue` first to create/checkout the branch.

## Arguments

`$ARGUMENTS` is the issue number to work on.

## Steps

### 1. Confirm branch

Verify that a branch for this issue is already checked out:

```bash
git branch --show-current
git log main..HEAD --oneline
git diff main...HEAD --stat
```

If you are on `main` or an unrelated branch, stop and tell the user to run `/setup-issue $ARGUMENTS` first.

### 2. Read the issue and its relationships, then find or generate a plan

First, read the issue and resolve its GitHub relationships:

```bash
gh issue view $ARGUMENTS --json title,body,comments
gh api repos/{owner}/{repo}/issues/$ARGUMENTS --jq '{parent: .parent_issue_url}'
gh api repos/{owner}/{repo}/issues/$ARGUMENTS/sub_issues --jq '[.[] | "#\(.number) \(.title) [\(.state)]"]'
```

If the issue has **sub-issues of its own**, read each one — they define the scope of work broken out under this issue and are useful context for planning.

If `parent_issue_url` is set, extract the parent issue number (last path segment of the URL) and read it along with all its sub-issues (your siblings):

```bash
gh issue view <parent_number> --json title,body,comments
gh api repos/{owner}/{repo}/issues/<parent_number>/sub_issues --jq '[.[] | "#\(.number) \(.title) [\(.state)]"]'
```

This gives you the full picture: what the parent is trying to accomplish, what work has already been done (closed siblings), and what's still in flight.

Now check for a plan:

```bash
ls plans/ | grep -iE "issue-$ARGUMENTS|$ARGUMENTS" || true
```

- **If a matching plan exists** (e.g. `plans/issue-$ARGUMENTS-*.md`): read it in full and follow it. Do not re-plan from scratch. If parts are stale or unclear, ask before deviating.
- **If no plan exists:** look for a plan for the parent issue (if there is one):

  ```bash
  ls plans/ | grep -iE "issue-<parent_number>|^<parent_number>" || true
  ```

  Read the parent plan if it exists — it contains the full technical context and design decisions that apply to this sub-issue. Use it to scope the plan you write to just the work described in the sub-issue body.

  For complex or ambiguous issues, consider stopping here and running `/plan` in Opus instead — it can ask clarifying questions, whereas the subagent below cannot. For straightforward issues, spawn a `claude` subagent with `model: "opus"` to research the codebase and write the plan. Use this briefing template:

  ```
  You are researching a bug/feature in a Go + Ebiten game codebase and writing an
  implementation plan. Do NOT write any code — only explore and produce the plan file.

  ## Issue <N>: <title>
  <full issue body>

  ## Parent issue context (if this is a sub-issue)
  <paste the parent issue title, body, and list of all its sub-issues with their open/closed state>

  ## Parent plan context (if this is a sub-issue)
  <paste the relevant sections of the parent plan here>

  ## Your task
  1. Explore the codebase to understand the relevant code paths and data flow.
     Key locations: `internal/game/` (world.go, sim.go, render.go, ui.go, input.go, game.go)
     and `main.go`. Read Serena memories (core, conventions) for project invariants.
  2. Identify the minimal change needed and any utilities or patterns already in
     the codebase to reuse.
  3. Write the plan to `plans/issue-<N>-<short-slug>.md` using this format:

  ## Context
  <what the issue is, root cause or motivation, why the fix/feature is needed>

  ## Files to modify
  - `path/to/file.go` — what to change and why
    - specific guidance on the change

  ## Utilities / patterns to reuse
  <existing helpers, patterns, or APIs the implementation should leverage>

  ## Verification
  - Tests: <what tests to write or update>
  - Manual: <make run steps to visually verify>

  The plan must be self-contained — a separate agent will execute it without
  this conversation's context.

  Working directory: /home/dan/workspace/personal/dreamsofpotential
  ```

  Wait for the agent to finish, then read the written plan file and follow it.

### 3. Implement

- Make the smallest change that resolves the issue.
- Write failing tests first, then fix, then verify tests pass (see [[feedback-regression-tests-first]]).
- Do not refactor surrounding code unless it directly blocks the fix.

### 4. Verify

```bash
make check   # go vet
make test    # go test ./...
```

Both must pass. Fix failures before proceeding.

If the change touches rendering or UI, run `make run` and exercise the golden path manually.

### 5. Commit

```bash
git add <specific files>
git commit -m "short imperative subject

Optional body if the why is non-obvious."
```

Rules:
- No `Co-Authored-By` trailers.
- Subject line: imperative mood, ≤72 chars.
- Body only when the why isn't obvious from the subject.
- Stage specific files, not `git add -A`.

### 6. Open the PR

Run `/create-pr closes #$ARGUMENTS`. That skill handles drafting the title and body, confirming with the user, creating the PR, and returning to `main`.
