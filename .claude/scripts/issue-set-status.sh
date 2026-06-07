#!/usr/bin/env bash
# Usage: issue-set-status.sh <issue-number> <column-name>
set -e

ISSUE=$1
COLUMN=$2
PROJECT_ID="PVT_kwDOAolfr84A4TAD"
STATUS_FIELD_ID="PVTSSF_lADOAolfr84A4TADzgtSZyg"

# Look up the option ID for the given column name
OPTION_ID=$(gh api graphql -f query="{
  node(id: \"$PROJECT_ID\") {
    ... on ProjectV2 {
      field(name: \"Status\") {
        ... on ProjectV2SingleSelectField {
          options {
            id
            name
          }
        }
      }
    }
  }
}" --jq ".data.node.field.options[] | select(.name == \"$COLUMN\") | .id")

if [ -z "$OPTION_ID" ]; then
  echo "Column \"$COLUMN\" not found on the project board."
  exit 1
fi

# Get the item ID for this issue on the project board
ITEM_ID=$(gh api graphql -f query="{
  repository(owner: \"kiyanaw\", name: \"kiyanaw-backend\") {
    issue(number: $ISSUE) {
      projectItems(first: 10) {
        nodes {
          id
          project { id }
        }
      }
    }
  }
}" --jq ".data.repository.issue.projectItems.nodes[] | select(.project.id == \"$PROJECT_ID\") | .id")

if [ -z "$ITEM_ID" ]; then
  echo "Issue #$ISSUE is not on the project board, adding it..."
  ISSUE_NODE_ID=$(gh api graphql -f query="{
    repository(owner: \"kiyanaw\", name: \"kiyanaw-backend\") {
      issue(number: $ISSUE) { id }
    }
  }" --jq ".data.repository.issue.id")

  ITEM_ID=$(gh api graphql -f query="mutation {
    addProjectV2ItemById(input: {
      projectId: \"$PROJECT_ID\"
      contentId: \"$ISSUE_NODE_ID\"
    }) {
      item { id }
    }
  }" --jq ".data.addProjectV2ItemById.item.id")
fi

gh api graphql -f query="mutation {
  updateProjectV2ItemFieldValue(input: {
    projectId: \"$PROJECT_ID\"
    itemId: \"$ITEM_ID\"
    fieldId: \"$STATUS_FIELD_ID\"
    value: { singleSelectOptionId: \"$OPTION_ID\" }
  }) {
    projectV2Item { id }
  }
}" > /dev/null

echo "Issue #$ISSUE moved to $COLUMN."
