#!/bin/sh
# Tag a release: make tag V=0.0.1 NOTE="…" (sh reader/scripts/release.sh 0.0.1 ["note"]). Pushing the tag makes the
# release: CI runs its checks, builds the Kobo package and puts it on the repository's Releases page with its checksum
# (.github/workflows/ci.yml, the release job): git push origin main v0.0.1.
# FarsiEPUB's scripts/release, for ShamsaInk: the version is only its tag (the build writes it into the binary), so
# there is no bump. Checks first: on main, nothing uncommitted, no such tag yet, newer than the last one, something new
# since it, pushed as origin/main and passed by CI there. The tag's message is the note and the commits since the last
# tag. It doesn't push.
# Versions: X.Y.Z, or X.Y.Z-beta / X.Y.Z-beta.2 and the like. When to raise which number, and the history: CHANGELOG.md.
set -e
cd "$(dirname "$0")/../.."
usage() { echo "usage: reader/scripts/release.sh X.Y.Z[-beta[.N]] [note]" >&2; exit 2; }
[ $# -ge 1 ] && [ $# -le 2 ] || usage

v=${1#v}
t="v$v"
echo "$v" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z]+(\.[0-9]+)?)?$' || { echo "Version: X.Y.Z, e.g. 0.0.1, or 0.1.0-beta" >&2; exit 2; }
[ "$(git rev-parse --abbrev-ref HEAD)" = main ] || { echo "Tag on main." >&2; exit 1; }
[ -z "$(git status --porcelain --untracked-files=no)" ] || { echo "Commit or stash your changes first." >&2; exit 1; }
if git rev-parse -q --verify "refs/tags/$t" >/dev/null; then
  echo "$t exists already (on $(git rev-list -n 1 --abbrev-commit "$t")). Pick the next version." >&2
  exit 1
fi
last=$(git describe --tags --abbrev=0 --match 'v[0-9]*' 2>/dev/null || true)
# sort -V puts 0.1.0 before 0.1.0-beta; for us a pre-release comes first, so its suffix is set aside to compare
base() { echo "$1" | sed 's/-.*//'; }
if [ -n "$last" ]; then
  newer=$(printf '%s\n%s\n' "$(base "$last")" "$(base "$t")" | sort -V | tail -1)
  if [ "$(base "$last")" = "$(base "$t")" ]; then
    case "$last" in *-*) ;; *) echo "$t isn't newer than $last." >&2; exit 1 ;; esac
    case "$t" in *-*) [ "$(printf '%s\n%s\n' "$last" "$t" | sort -V | tail -1)" = "$t" ] || { echo "$t isn't newer than $last." >&2; exit 1; } ;; esac
  elif [ "$newer" != "$(base "$t")" ]; then
    echo "$t isn't newer than $last." >&2
    exit 1
  fi
  if [ "$(git rev-list -n 1 "$last")" = "$(git rev-parse HEAD)" ]; then
    echo "Nothing new since $last." >&2
    exit 1
  fi
fi
# Only a commit CI has passed on main is tagged (2026-11-08: v0.0.3 was tagged on a commit whose screen walk failed,
# so its release was never built): pushed first, its CI run green, then tagged and the tag pushed.
head=$(git rev-parse HEAD)
git fetch -q origin main
[ "$(git rev-parse origin/main)" = "$head" ] || { echo "Push main first (git push origin main), wait for CI to pass, then tag: this commit is not origin/main." >&2; exit 1; }
runs=$(gh run list --branch main --commit "$head" --json status,conclusion --jq '.[] | "\(.status) \(.conclusion)"' 2>/dev/null) || { echo "Could not ask GitHub for CI's runs (gh)." >&2; exit 1; }
case "$runs" in
  "") echo "CI has not run on $(git rev-parse --short HEAD) yet: wait for it, then tag." >&2; exit 1 ;;
  *"completed success"*) ;;
  *completed*) echo "CI failed on $(git rev-parse --short HEAD) (gh run list --branch main): fix it first." >&2; exit 1 ;;
  *) echo "CI is still running on $(git rev-parse --short HEAD) (gh run watch): tag once it passes." >&2; exit 1 ;;
esac
grep -q "^## $v " CHANGELOG.md || echo "Note: CHANGELOG.md has no entry for $v yet (the release's notes come from it)." >&2
{
  echo "ShamsaInk $v"
  echo
  if [ -n "$2" ]; then echo "$2"; echo; fi
  if [ -n "$last" ]; then echo "Since $last:"; git log --format='- %s' "$last..HEAD"; fi
} | git tag -a "$t" -F -
echo "Tagged $t on $(git rev-parse --short HEAD), which CI passed. Check it with git show $t, then release it: git push origin $t"
