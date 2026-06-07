# Create Issue

Intake information from the user, draft a GitHub issue, create it, and optionally hand off to setup-issue.

Does not investigate the codebase or do any implementation work.

## Steps

### 1. Gather information

Ask the user for all of the following in a single message:

- **Title**: A short, descriptive title
- **Context / Motivation**: Why is this needed? What problem does it solve?
- **Labels**: Any labels to apply — e.g. `bug`, `enhancement`, `question` (optional)
- **Parent issue**: Is this a sub-issue of an existing issue? If so, which number? (optional)
- **Blocked by**: Does this issue depend on another issue being completed first? If so, which number(s)? (optional)

Wait for the user's response before proceeding.

### 2. Ask clarifying questions (if needed)

If the motivation is unclear, ask one or two targeted follow-up questions. Keep it brief — this is not a requirements session.

### 3. Draft the issue

The issue body is plain prose only — no headers, no acceptance criteria, no notes section. Do not prescribe how the work should be done or define what "done" looks like; that is left to the developer. Write a concise paragraph describing the problem and its likely cause.

### 4. Show draft for review

Present the full draft — title and body — to the user and ask: "Does this look right, or would you like any changes?"

Iterate until the user approves.

### 5. Create the issue

Once approved, create the issue:

```bash
gh issue create --title "<title>" --body "<body>" [--label "<label>"]
```

Capture the issue URL from the output and extract the issue number (the trailing integer in the URL).

If the user specified a parent issue, register the new issue as a sub-issue of it:

```bash
CHILD_ID=$(gh api repos/{owner}/{repo}/issues/<new-issue-number> --jq '.id')
gh api repos/{owner}/{repo}/issues/<parent-number>/sub_issues --method POST -F sub_issue_id=$CHILD_ID
```

If the user specified one or more "blocked by" issues, wire each blocking relationship. One call covers both sides — it populates `blocking` on the blocker and `blockedBy` on the new issue simultaneously:

```bash
NEW_NODE=$(gh api repos/{owner}/{repo}/issues/<new-issue-number> --jq '.node_id')
BLOCKING_NODE=$(gh api repos/{owner}/{repo}/issues/<blocking-number> --jq '.node_id')

gh api graphql -f query='
mutation($issueId: ID!, $blockingId: ID!) {
  addBlockedBy(input: { issueId: $issueId, blockingIssueId: $blockingId }) {
    issue { number }
    blockingIssue { number }
  }
}' -F issueId="$NEW_NODE" -F blockingId="$BLOCKING_NODE"
```

Repeat for each blocker.

### 6. Offer to set up for work

Ask the user: "Would you like to start working on this issue now?"

If yes, run `/setup-issue <issue-number>`.
