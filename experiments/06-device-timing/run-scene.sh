#!/bin/sh
# On the device: where Blitz's time building a page's scene goes. blitz-profile (NEON, with blitz-knobs.patch) on
# Buried Alive as it is, three ways: as drawn, glyphs unhinted (BLITZ_NO_HINT), glyphs not drawn (BLITZ_NO_GLYPHS).
# Appends to results-scene.txt.
#
#     sh run-scene.sh [BIN_DIR]
cd "$(dirname "$0")"
B=${1:-bin}
{
  echo "== $(date -u '+%Y-%m-%d %H:%M UTC')"
  echo "as drawn:";         $B/blitz-profile-knobs books/buried-alive | grep '^draw'
  echo "unhinted:";         BLITZ_NO_HINT=1 $B/blitz-profile-knobs books/buried-alive | grep '^draw'
  echo "glyphs not drawn:"; BLITZ_NO_GLYPHS=1 $B/blitz-profile-knobs books/buried-alive | grep '^draw'
} 2>&1 | tee -a results-scene.txt
