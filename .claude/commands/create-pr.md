# Create PR

Open a PR for an existing branch. Use this when work is already done and committed.

## Arguments

`$ARGUMENTS` is optional. Can be a branch name, an issue number to close, or both — e.g. `issue-249-lakota-composition closes #249`. If omitted, uses the current branch.

## Steps

### 1. Establish the branch

If a branch name was given, check it out. Otherwise use the current branch. Always check out the branch so the user can do manual testing while reviewing the draft.

```bash
git checkout <branch-name>   # or skip if already on the right branch
git status                   # confirm current branch
```

### 2. Read the diff

```bash
git log main..HEAD --oneline
git diff main --stat
```

Read the commits and changed files to understand the full scope of the work. If an issue number was provided, read it too:

```bash
gh issue view <number> --json title,body,comments,labels
```

Note the issue's labels — they will be applied to the PR.

### 3. Draft the PR

**Title**: short, plain English, describes the outcome. No ticket numbers.

**Body** (see [[feedback-pr-body]]):
- First line: `resolve #N` if this closes an issue.
- 1–2 plain prose sentences describing what was done and why.
- Inline "Note:" for known limitations — no extra headers.
- `### Testing` section: what automated tests cover it, and what was manually verified. If unknown, ask the user rather than inventing.
- No bold-labelled sections, no template placeholders, no padding.

Only add a "Note:" for a non-obvious finding if it is genuinely surprising — a hidden constraint, a subtle invariant, a decision that would confuse a future reader without context. Most implementations don't warrant one; do not add notes by default.

### 4. Confirm with user

Show the draft title and body. **Do not run `gh pr create` until the user approves.** This is the one step visible to others.

### 5. Create the PR

Push the branch, then create the PR **without** `--label` (labels are applied separately in the next step to avoid GitHub double-applying them):

```bash
git push -u origin <branch-name>
gh pr create --base main --assignee danielfay --title "<title>" --body "$(cat <<'EOF'
<body>
EOF
)"
```

After creation, apply any labels from the linked issue — but check first to avoid duplicates. If a label exists on the issue but not on PRs, skip it and tell the user so they can add it manually:

```bash
gh pr view <pr-number> --json labels --jq '.labels[].name' | grep -qx "<label>" \
  || gh pr edit <pr-number> --add-label "<label>"
```

### 6. Delete the plan file

If a plan file exists for this issue (e.g. `plans/issue-<number>-*.md`), delete it now:

```bash
rm plans/issue-<number>-*.md
```

The PR body and the code are the authoritative record. The plan served its purpose.

### 7. Return to main

```bash
git checkout main
```

Always leave the working directory on `main` after the PR is opened.
