#!/bin/sh
# Mechanical guard-rail checks on a change (agents/guardrails.md). POSIX sh plus python3 stdlib.
#   check-deps.sh deps <diff-file> <issue-body-file> | deps --pr <N> --base <sha>
#       Fails when go.mod, go.sum or pom.xml gain a coordinate (Go module path; Maven
#       artifactId in <dependency>/<plugin>/<parent>/<extension>) that the linked
#       ticket's "Constraints" does not name as a whole token. The diff needs full
#       context (git diff --unified=100000): "gain" = in the new file, not the old one,
#       so version bumps do not count. New transitive go.sum modules must be named too.
#       --pr reuses check-scope.sh for the ticket link/approval checks via a gh wrapper.
#   check-deps.sh size <numstat-file>|--base <sha> [limit]   default limit 600; lockfiles
#       (go.sum, *.lock, package-lock.json...) and generated files are not counted.
#   check-deps.sh go-lcov <module-dir> <cover-profile>       lcov on stdout, for diff-cover
set -eu

die() { printf '%s\n' "$*" >&2; exit 2; }
HERE=$(cd "$(dirname "$0")" && pwd)
TMPD=$(mktemp -d)
trap 'rm -rf "$TMPD"' EXIT INT TERM

cmd_deps() {
  if [ "${1:-}" = "--pr" ]; then
    [ "${3:-}" = "--base" ] || die "usage: deps --pr N --base SHA"
    REAL_GH=${GH:-gh}
    # Records the issue check-scope.sh resolves; stubs the files API (scope is another job).
    cat >"$TMPD/gh" <<WRAP
#!/bin/sh
case "\$1 \$2" in
  "issue view") out=\$("$REAL_GH" "\$@") || exit \$?; printf '%s\n' "\$out" >"$TMPD/issue.json"; printf '%s\n' "\$out" ;;
  "api --paginate") echo '[]' ;;
  *) exec "$REAL_GH" "\$@" ;;
esac
WRAP
    chmod +x "$TMPD/gh"
    GH="$TMPD/gh" sh "$HERE/check-scope.sh" --pr "${2:?pr number}" >/dev/null || die "ticket link validation failed"
    jq -r '.body // ""' "$TMPD/issue.json" >"$TMPD/body.md"
    git diff --unified=100000 "${4:?base sha}...HEAD" -- '*go.mod' '*go.sum' '*pom.xml' >"$TMPD/deps.diff"
    set -- "$TMPD/deps.diff" "$TMPD/body.md"
  fi
  [ -r "${1:-}" ] && [ -r "${2:-}" ] || die "usage: deps <diff-file> <issue-body-file>"
  python3 - "$1" "$2" <<'PY'
import re, sys

def files(diff):  # (path, old_lines, new_lines) per file of a full-context diff
    cur = None
    for raw in open(diff, encoding="utf-8", errors="replace").read().split("\n"):
        if raw.startswith("diff --git "):
            if cur: yield cur
            cur = {"path": raw.rsplit(" b/", 1)[-1], "old": [], "new": [], "hunk": False}
        elif cur is None: continue
        elif not cur["hunk"]: cur["hunk"] = raw.startswith("@@")
        elif raw.startswith("+"): cur["new"].append(raw[1:])
        elif raw.startswith("-"): cur["old"].append(raw[1:])
        elif raw.startswith(" "): cur["old"].append(raw[1:]); cur["new"].append(raw[1:])
    if cur: yield cur

def go_coords(lines, is_sum):
    out = set()
    for line in lines:
        toks = line.split("//")[0].split()
        if toks[:1] and toks[0] in ("module", "go", "toolchain", "godebug", "retract"): continue
        if toks[:1] and toks[0] in ("require", "replace", "exclude"): toks = toks[1:]
        if not toks or toks[-1] == "(": continue
        found = toks[:1]
        if not is_sum and "=>" in toks:  # replace: the target counts unless it is a local path
            found += [t for t in toks[toks.index("=>") + 1:][:1] if not t.startswith((".", "/"))]
        out.update(t for t in found if "/" in t or "." in t)
    return out

def pom_coords(lines):
    out, depth = set(), 0
    for line in lines:
        for m in re.finditer(r"<(/?)(\w+)[^>]*?>(?:([^<]*)</\2>)?", line):
            closing, tag, text = m.groups()
            if tag in ("dependency", "plugin", "parent", "extension") and not text:
                depth = max(depth + (-1 if closing else 1), 0)
            elif tag == "artifactId" and depth and text:
                out.add(text.strip())
    return out

text, grab = [], False  # the Constraints section of the ticket
for line in open(sys.argv[2], encoding="utf-8", errors="replace").read().replace("\r", "").split("\n"):
    if re.match(r"#+\s", line): grab = bool(re.match(r"#+\s*Constraints", line, re.I)); continue
    if grab: text.append(line)
text = "\n".join(text)
mentioned = lambda c: re.search(r"(?<![A-Za-z0-9._/-])" + re.escape(c) + r"(?![A-Za-z0-9_/-])", text)

bad = []
for f in files(sys.argv[1]):
    base = f["path"].rsplit("/", 1)[-1]
    if base in ("go.mod", "go.sum"): gained = go_coords(f["new"], base == "go.sum") - go_coords(f["old"], base == "go.sum")
    elif base == "pom.xml": gained = pom_coords(f["new"]) - pom_coords(f["old"])
    else: continue
    bad += [(f["path"], c) for c in sorted(gained) if not mentioned(c)]
for path, c in bad:
    print(f"UNAPPROVED DEPENDENCY: {c} (added in {path}) is not named in the ticket's Constraints", file=sys.stderr)
if bad: sys.exit("deps-check failed: name it in the ticket's Constraints (human edit) or remove it")
print("deps-check ok")
PY
}

cmd_size() {
  if [ "${1:-}" = "--base" ]; then git diff --numstat "${2:?base sha}...HEAD" >"$TMPD/numstat"; shift 2
  else [ -r "${1:-}" ] || die "usage: size <numstat-file>|--base SHA [limit]"; cp "$1" "$TMPD/numstat"; shift; fi
  python3 - "$TMPD/numstat" "${1:-600}" <<'PY'
import re, sys
EXCLUDED = re.compile(r"(^|/)(go\.sum|package-lock\.json|yarn\.lock|pnpm-lock\.yaml)$|\.lock$|\.pb\.go$|_gen\.go$|\.gen\.go$|(^|/)(generated|gen)/|\.generated\.")
limit, total, skipped = int(sys.argv[2]), 0, 0
for line in open(sys.argv[1]):
    add, dele, path = (line.rstrip("\n").split("\t", 2) + ["", "", ""])[:3]
    n = (int(add) if add.isdigit() else 0) + (int(dele) if dele.isdigit() else 0)  # binary files show "-"
    if EXCLUDED.search(path): skipped += n
    else: total += n
print(f"changed lines: {total} (limit {limit}; {skipped} lockfile/generated lines excluded)")
if total > limit: sys.exit("size-check failed: diff too large; the ticket should be split (escalate)")
PY
}

cmd_golcov() {
  MODDIR=${1:?module dir (repo-relative)}; PROFILE=${2:?cover profile}
  [ -r "$PROFILE" ] && [ -r "$MODDIR/go.mod" ] || die "cannot read $PROFILE or $MODDIR/go.mod"
  MODPATH=$(sed -n 's/^module[[:space:]]\{1,\}//p' "$MODDIR/go.mod" | head -1 | tr -d '\r')
  python3 - "${MODDIR#./}" "$MODPATH" "$PROFILE" <<'PY'
import re, sys
moddir, modpath, profile = sys.argv[1:4]
hits = {}  # repo-relative path -> {line: count}
for line in open(profile):
    m = re.match(r"(.+):(\d+)\.\d+,(\d+)\.\d+ \d+ (\d+)$", line.strip())
    if not m or not m.group(1).startswith(modpath + "/"): continue  # also skips "mode:"
    prefix = "" if moddir in ("", ".") else moddir.rstrip("/") + "/"
    per = hits.setdefault(prefix + m.group(1)[len(modpath) + 1:], {})
    for n in range(int(m.group(2)), int(m.group(3)) + 1):
        per[n] = max(per.get(n, 0), int(m.group(4)))
for path in sorted(hits):
    print("TN:\nSF:" + path)
    for n in sorted(hits[path]): print(f"DA:{n},{hits[path][n]}")
    print("end_of_record")
PY
}

SUB=${1:-}; [ $# -eq 0 ] || shift
case "$SUB" in
  deps) cmd_deps "$@" ;;
  size) cmd_size "$@" ;;
  go-lcov) cmd_golcov "$@" ;;
  *) die "usage: check-deps.sh deps|size|go-lcov ... (see header)" ;;
esac
