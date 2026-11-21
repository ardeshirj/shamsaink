#!/bin/sh
# The release signing key for updates over Wi-Fi (reader/update, docs/DESIGN.md "Updates over Wi-Fi"), made by the
# maintainer only, on their Mac (never by an agent):
#
#   SIGNING_KEY_BACKUP='COMMAND' make signing-key
#
# An ed25519 pair made by reader/cmd/sign (in Docker); the private key piped into COMMAND, which keeps the backup (it
# reads the key on standard input, and should refuse to overwrite a key it already holds: that copy is the only one),
# then into this repository's GitHub Actions secret SHAMSAINK_SIGNING_KEY, which the release job signs with. Without
# SIGNING_KEY_BACKUP nothing is made: no key without its backup; a backup that fails stops it before GitHub. The private
# key is never a file, never printed, never on a command line: a variable and pipes only. Only the public key is
# printed: it goes into reader/update/keys.go (ReleaseKeys), committed. Where the backup lives is not this repository's
# business: the command says it.
#
# A new key replacing this one: its public half must reach the readers in a release still signed with the old key
# (keys.go holds both), so GitHub's secret is switched only after that release (reader/update/keys.go).
#
# Tests: GH and DOCKER name the programs (stand-ins in a test; the real ones otherwise).
set -e
cd "$(dirname "$0")/../.."
REPO=${REPO:-ardeshirj/shamsaink}
GH=${GH:-gh}
DOCKER=${DOCKER:-docker}
NAME=SHAMSAINK_SIGNING_KEY

fail() { echo "$*" >&2; exit 1; }
[ -n "$SIGNING_KEY_BACKUP" ] || fail "SIGNING_KEY_BACKUP is not set: the command that keeps the key's backup (it reads the key on standard input). No key is made without one."
command -v "$GH" > /dev/null || fail "gh is needed (GitHub's command line), signed in: gh auth login"
"$GH" auth status > /dev/null 2>&1 || fail "gh is not signed in: gh auth login"

pair=$("$DOCKER" run --rm -v "$PWD":/r -v eink-gomod:/go/pkg/mod -v eink-gocache:/root/.cache/go-build \
  -w /r/reader eink-go go run ./cmd/sign -keygen -) || fail "no key was made"
pub=$(printf '%s\n' "$pair" | sed -n 1p)
priv=$(printf '%s\n' "$pair" | sed -n 2p)
pair=
[ -n "$pub" ] && [ -n "$priv" ] || fail "no key was made"

# the backup first: a key GitHub signs with must never be without it
printf %s "$priv" | sh -c "$SIGNING_KEY_BACKUP" > /dev/null || fail "the backup command failed: nothing was changed (the new key was thrown away)"
printf %s "$priv" | "$GH" secret set "$NAME" --repo "$REPO" > /dev/null ||
  fail "GitHub did not take $NAME: the key is in the backup only; set GitHub's secret from it"
priv=
echo "The private key: in the backup and in $REPO's secret $NAME; no file."
echo "The public key, for reader/update/keys.go (ReleaseKeys):"
echo "$pub"
