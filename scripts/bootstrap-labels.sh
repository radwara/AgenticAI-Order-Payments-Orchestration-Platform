#!/usr/bin/env bash
# Usage: scripts/bootstrap-labels.sh [owner/repo]
set -euo pipefail
repo="${1:-$(gh repo view --json nameWithOwner -q .nameWithOwner)}"
python3 - "$repo" <<'PY'
import sys,subprocess,re
repo=sys.argv[1]
for line in open(".github/labels.yml"):
    m=re.match(r'- \{name: "(.*?)", color: "(.*?)", description: "(.*?)"\}',line)
    if m:
        n,c,d=m.groups()
        subprocess.run(["gh","label","create",n,"--color",c,"--description",d,"--repo",repo,"--force"],check=True)
PY
