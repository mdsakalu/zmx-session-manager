#!/usr/bin/env bash
set -euo pipefail

# Check only PRs whose merge commits are included in this checkout.
pulls=$(gh api --paginate "repos/${GITHUB_REPOSITORY}/pulls?state=closed&per_page=100" \
  --jq '.[] | select(.merged_at and .user.type == "User") | [.merge_commit_sha, .user.login, .number] | @tsv')
missing=0
while IFS=$'\t' read -r sha login number; do
  [[ -n "$sha" ]] || continue
  git merge-base --is-ancestor "$sha" HEAD 2>/dev/null || continue
  if ! grep -Fiq "[@${login}](https://github.com/${login})" CONTRIBUTORS.md; then
    echo "Add @${login} (PR #${number}) to CONTRIBUTORS.md before releasing." >&2
    missing=1
  fi
done <<< "$pulls"
exit "$missing"
