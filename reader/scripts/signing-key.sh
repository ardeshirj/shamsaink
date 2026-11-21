#!/bin/sh
# The release signing key for updates over Wi-Fi (reader/update, docs/DESIGN.md "Updates over Wi-Fi"), made once by the
# maintainer, on their Mac: `make signing-key` (sh reader/scripts/signing-key.sh).
#   - an ed25519 key pair made by reader/cmd/sign -keygen, in Docker (the eink-go image)
#   - the private key written straight into a backup file in the home folder, readable by its owner only
#     (~/shamsaink-signing-key-DATE.txt: keep it somewhere safe and offline, it is the only copy outside GitHub)
#   - and from that file into the repository's GitHub Actions secret SHAMSAINK_SIGNING_KEY (gh secret set, reading it
#     from the file: it is never printed nor passed on a command line), which the release job signs with
#   - only the public key printed: it goes into reader/update/keys.go (ReleaseKeys), committed
# Refuses to overwrite a backup that is there. A second key (to replace the first) is made the same way: the new public
# key added to ReleaseKeys beside the old one for a release, the old one removed after.
set -e
cd "$(dirname "$0")/../.."
REPO=${REPO:-ardeshirj/shamsaink}
backup="$HOME/shamsaink-signing-key-$(date +%Y-%m-%d).txt"
[ -e "$backup" ] && { echo "$backup exists already: not overwritten" >&2; exit 1; }
command -v gh > /dev/null || { echo "gh is needed (GitHub's command line), logged in" >&2; exit 1; }
gh auth status > /dev/null 2>&1 || { echo "gh is not logged in: gh auth login" >&2; exit 1; }
umask 077
name=$(basename "$backup")
pub=$(docker run --rm -v "$PWD":/r -v "$HOME":/out -v eink-gomod:/go/pkg/mod -v eink-gocache:/root/.cache/go-build \
  -w /r/reader eink-go go run ./cmd/sign -keygen "/out/$name")
[ -s "$backup" ] || { echo "no key was made" >&2; exit 1; }
chmod 600 "$backup"
gh secret set SHAMSAINK_SIGNING_KEY --repo "$REPO" < "$backup"
echo "The private key: in $backup (keep it safe, offline) and in $REPO's secret SHAMSAINK_SIGNING_KEY."
echo "The public key, for reader/update/keys.go (ReleaseKeys):"
echo "$pub"
