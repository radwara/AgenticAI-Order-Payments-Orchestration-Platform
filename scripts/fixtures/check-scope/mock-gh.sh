#!/bin/sh
# Mock of the gh CLI for scripts/test-check-scope.sh. Serves canned JSON from
# $MOCK_DIR (the fixtures directory):
#   gh pr view N --json ...            -> prs.json[$MOCK_PR]
#   gh issue view N --json ...         -> issues.json[N]  (fails if absent)
#   gh api --paginate repos/.../files  -> files.json[$MOCK_FILES], each element
#                                         emitted as its own page (like --paginate)
case "$1 $2" in
  "pr view") jq -e --arg k "$MOCK_PR" '.[$k]' "$MOCK_DIR/prs.json" ;;
  "issue view") jq -e --arg k "$3" '.[$k]' "$MOCK_DIR/issues.json" || { echo "issue not found" >&2; exit 1; } ;;
  "api --paginate") jq -c --arg k "$MOCK_FILES" '.[$k][]' "$MOCK_DIR/files.json" ;;
  *) echo "mock-gh: unexpected: $*" >&2; exit 1 ;;
esac
