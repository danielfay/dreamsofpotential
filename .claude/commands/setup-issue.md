# Setup Issue

Prepare a GitHub issue for work: read it, assign it, move it to In Progress, and get the branch ready. Does not write any code.

## Arguments

`$ARGUMENTS` is the issue number to work on.

## Steps

### 1. Read the issue

```bash
gh issue view $ARGUMENTS --json title,body,comments
```

Read the full issue body and any linked comments. If the issue references other issues or PRs, read those too. Understand what done looks like before proceeding.

### 2. Assign

Assign the issue to `danielfay` if not already assigned.

```bash
gh issue edit $ARGUMENTS --add-assignee danielfay
```

### 3. Branch

Branch names start with the issue number, so check for an existing one first:

```bash
git fetch origin
git branch --list "$ARGUMENTS-*"         # local
git branch -r --list "origin/$ARGUMENTS-*"  # remote
```

- **If a matching branch exists locally**: check it out and pull any remote updates.
  ```bash
  git checkout <existing-branch>
  git pull origin <existing-branch> 2>/dev/null || true
  ```
- **If a matching branch exists only on remote**: check it out tracking the remote.
  ```bash
  git checkout -t origin/<existing-branch>
  ```
- **If no branch exists**: verify the working tree is clean first, then create from a fresh main.
  ```bash
  git status --porcelain   # must be empty — stop and warn the user if not
  git checkout main && git pull origin main
  git checkout -b $ARGUMENTS-<short-slug>
  ```
  The slug should be 2–4 words kebab-cased from the issue title. Branch names are `<number>-<short-slug>` — no `issue-` prefix.

### 4. Review existing work (existing branch only)

Skip this step if you just created a fresh branch.

If you checked out an existing branch, review what has already been done:

```bash
git log main..HEAD --oneline
git diff main...HEAD --stat
```

Read the commit messages and changed files. Then make a judgement call:

- **Looks complete** (commits address the issue, changed files match what the issue requires): stop and tell the user what you found — summarise the commits, the files changed, and why you think it's done. Ask whether to proceed to PR, continue work, or do nothing.
- **Partially done**: note what's been done and what's still missing, then continue.
- **Nothing meaningful committed** (empty or only boilerplate): note that the branch is fresh and ready.

### 5. Report status

Tell the user:
- The branch name and whether it was created or already existed.
- A one-sentence summary of the issue.
- The current state of the branch (fresh / partially done / looks complete).

If the branch looks complete, stop here and ask the user how to proceed. Otherwise, run `/do-issue $ARGUMENTS` to plan and implement.
