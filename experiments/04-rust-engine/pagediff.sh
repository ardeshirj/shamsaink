#!/bin/sh
# pagediff.sh A B: for each book, how many pages differ between out/draw-A and out/draw-B, and the most pixels and
# the largest grey difference on any page
for n in buried holmes; do
  same=0; diff=0; worst=0; peak=0
  for f in out/draw-$1/$n/page-*.png; do
    g=out/draw-$2/$n/${f##*/}
    if cmp -s "$f" "$g"; then same=$((same+1)); continue; fi
    diff=$((diff+1))
    px=$(compare -metric AE "$f" "$g" null: 2>&1 | cut -d' ' -f1)
    pk=$(compare -metric PAE "$f" "$g" null: 2>&1 | sed 's/.*(\(.*\))/\1/')
    [ "${px%.*}" -gt "$worst" ] && worst=${px%.*}
    peak=$(python3 -c "print(max($peak, round($pk*255)))")
  done
  echo "$n: $same pages identical, $diff differ (at most $worst px on a page, $peak grey levels)"
done
