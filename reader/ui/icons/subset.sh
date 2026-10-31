#!/bin/sh
# The icon fonts, cut from Phosphor's (phosphor-icons.com, MIT: LICENSE here) to the icons the panels use; subset.py
# does it, in a throwaway python image with fonttools. Run again after adding an icon's name to subset.py.
#
#     sh reader/ui/icons/subset.sh
set -e
cd "$(dirname "$0")"
docker run --rm -v "$PWD":/out -w /out python:3.12-slim sh -c \
  'pip install -q --root-user-action=ignore fonttools==4.55.0 2>/dev/null && python3 -I subset.py'
ls -l Phosphor.ttf Phosphor-Bold.ttf Phosphor-Fill.ttf
