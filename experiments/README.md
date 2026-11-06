# Experiments

Each folder is one experiment, numbered in the order they were done; its `NOTES.md` has what was tried, measured and
decided. The decisions are summed up in `CLAUDE.md` and `docs/PLAN.md`.

## The code behind the notes: the `experiments-archive` tag

The experiments' code and scripts are heavy and, once decided, rarely needed. The plan is to trim main to the notes
(results and decisions) and keep the full code behind a git tag, so nothing is lost and main stays light.

- **`experiments-archive`** (annotated, pushed): the whole repository on 2026-11-02 (`d6ec769`), every experiment's code
  and scripts, before any trim. To get it back: `git checkout experiments-archive` (or
  `git show experiments-archive:experiments/08-opening/pack.sh` for one file).
- **What it is for:** a name to find the code by (deleted files stay in history, but nobody remembers the commit), and
  the engine, scripts and fonts frozen as the experiments were measured. Rerun an old experiment from the tag, **not
  from main**: main's engine moves on (for example, `fallback_fonts` in `engine/src/lib.rs` now loads `Parastoo*` and
  `Vazirmatn*` by name, so 08 and 09's Haji Agha fonts folder loads differently than when they were measured).
- **What it is not:** a maintained copy. Docker images and fetched dependencies may have moved on; rebuilding it takes
  the toolchain of its time.

### Rules

1. **Never move or delete a pushed tag.** It is a published ref, and moving it changes what the old results were
   measured on.
2. **A new experiment does not move the tag.** It is a later commit, so it is not in the snapshot, and that is fine.
3. **Tag each later experiment on its own, just before trimming it:** `experiment/NN-name` (for example
   `experiment/15-name`), on the last commit that still has its full code. List them with `git tag -l 'experiment/*'`.
   `experiments-archive` stays the one snapshot of 01–14; no `-2`, `-3` numbered snapshots (their names say nothing).
4. **A trimmed experiment's `NOTES.md` says where its code is** ("full code: tag `experiments-archive`, folder
   `experiments/NN-name`", or for a later one its own tag, `experiment/NN-name`).
5. Changes to the engine, the reader or the fonts do not need a new tag; only changes to the experiments' own files do.
