# Decompose Issue

Break a large GitHub issue into focused sub-issues. Reads the parent issue and any existing plan, identifies natural seams, drafts sub-issues for review, then creates them and registers them as proper GitHub sub-issues using the parent/child relationship API.

Does not investigate the codebase beyond reading plans and issue content.

## Arguments

`$ARGUMENTS` is the parent issue number to decompose.

## Steps

### 1. Read the parent issue, plan, and existing sub-issues

```bash
gh issue view $ARGUMENTS --json title,body,comments
gh api repos/{owner}/{repo}/issues/$ARGUMENTS/sub_issues --jq '[.[] | "#\(.number) \(.title) [\(.state)]"]'
ls plans/ | grep -iE "issue-$ARGUMENTS|^$ARGUMENTS" || true
```

If a matching plan file exists, read it in full. The plan is the primary input for finding seams — the issue body alone is usually too high-level.

Note the existing sub-issues (if any) so you don't duplicate them.

### 2. Identify natural seams

**If the plan already defines phases or a numbered roadmap, use those phases directly as sub-issues — do not re-analyse or consolidate them.** Trust the plan; it was written with intentional boundaries. If the plan also calls out explicit spin-out sub-issues (labelled as such), include those too.

When there is no pre-defined phasing, find splits that satisfy as many of these as possible:

- **Deploy boundary**: can one piece be shipped and verified before the next begins?
- **Independent testability**: can it be unit- or smoke-tested without the other pieces?
- **Skill boundary**: backend-only vs frontend-only vs infra — single-discipline issues are easier to hand off
- **Risk isolation**: separate high-risk or out-of-band steps (e.g. console config, manual AWS setup) from routine code work
- **Size**: aim for 2–5 sub-issues; fewer than 2 means the split isn't worth it, more than 5 usually means over-engineering

Avoid splits that require two sub-issues to be deployed simultaneously to function — one should be able to land first without breaking anything.

### 3. Draft the sub-issues

For each proposed sub-issue:
- **Title**: short, imperative, specific enough that it could stand alone on the board
- **Body**: one or two sentences of plain prose — no headers, no technical detail, no acceptance criteria. Describe *what* the piece of work achieves and *why* it matters to the user, not how to implement it. Do not reference sibling sub-issue numbers in the body (use the blocking relationship instead). A developer reading it should understand the goal without needing implementation knowledge.

Also identify the **blocking relationships** between sub-issues: which issues must be completed before others can begin. Express these as a simple list: `#A blocks #B`. Use the draft numbers (1, 2, 3…) since real issue numbers aren't known yet.

### 4. Present for review

Show all proposed sub-issues together in a numbered list, followed by the dependency graph:

```
**1. <title>**
<body>

**2. <title>**
<body>
...

**Dependencies:**
- #1 blocks #3
- #2 blocks #3
- #3 blocks #4, #5
...
```

Ask: "Do these look right, or would you like to adjust any titles, bodies, or the split itself?"

Iterate until the user approves. Do not create anything until the user explicitly signs off.

### 5. Create the sub-issues and register them as children

For each approved sub-issue, create it:

```bash
gh issue create --title "<title>" --body "<body>"
```

Capture the issue number from the URL in the output. Then fetch its numeric ID and register it as a sub-issue of the parent:

```bash
ISSUE_ID=$(gh api repos/{owner}/{repo}/issues/<number> --jq '.id')
gh api repos/{owner}/{repo}/issues/$ARGUMENTS/sub_issues --method POST -F sub_issue_id=$ISSUE_ID
```

Repeat for each sub-issue. The parent/child relationship is stored by GitHub — do not edit the parent issue body to add a sub-issues list.

### 5b. Wire blocking relationships

After **all** sub-issues have been created, set the blocking relationships identified in Step 3. For each "X blocks Y" pair, fetch both node IDs and call `addBlockedBy` once — this automatically populates `blocking` on X and `blockedBy` on Y from a single call:

```bash
BLOCKED_NODE=$(gh api repos/{owner}/{repo}/issues/<blocked-number> --jq '.node_id')
BLOCKING_NODE=$(gh api repos/{owner}/{repo}/issues/<blocking-number> --jq '.node_id')

gh api graphql -f query='
mutation($issueId: ID!, $blockingId: ID!) {
  addBlockedBy(input: { issueId: $issueId, blockingIssueId: $blockingId }) {
    issue { number }
    blockingIssue { number }
  }
}' -F issueId="$BLOCKED_NODE" -F blockingId="$BLOCKING_NODE"
```

Repeat for each dependency pair.

### 6. Report

List each created sub-issue as `#<number> — <title>` with its URL. Confirm the parent relationship was set by noting the `parent_issue_url` field returned from the sub_issues POST. Then list the blocking relationships that were wired, e.g. `#277 blocks #279`.
