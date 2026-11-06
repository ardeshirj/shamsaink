# Changelog

ShamsaInk's versions, newest first. A version is a git tag `vX.Y.Z`; pushing one builds the Kobo package and puts it
on GitHub's Releases page. A version's section here is its release's notes. How versions are numbered and released
is at the end, under "Versions and releases".

## Versions and releases

The same pattern as rtlbook's and FarsiEPUB's: a version is a git tag `vX.Y.Z`. Like FarsiEPUB's, the number lives only
in the tag; the build writes it into the binary (`shamsaink -version`, the first line of `reader.log`, the package's
`VERSION`), so there is nothing to bump.

### When the version changes

- **Before the beta, `v0.0.x`:** a build worth keeping or installing from a release (on the maintainer's devices).
  Every one is a pre-release on GitHub.
- **`v0.1.0-beta`** is reserved for the public beta (`docs/plan.md`, "Releases"); fixes to it `v0.1.0-beta.2`, ...
- **After it**, a new middle number (0.1.0 → 0.2.0) for something a reader notices: a feature, a screen, a setting, a
  device; a new last number (0.1.0 → 0.1.1) for a fix that changes none of that. **No release** for docs and tidying.
- **`v1.0.0`** is the stable release (the plan's "After the beta, toward stable").

### How to release

```bash
# add the version at the top of this file (its section is the release's notes), and commit
make tag V=0.0.1 NOTE="What changed"   # tag v0.0.1: the note, then the commits since the last tag
git show v0.0.1                        # check it
git push origin main v0.0.1            # GitHub runs the checks, then builds and publishes the release
```

`make tag` (`reader/scripts/release.sh`) only tags a clean `main` with a version newer than the last, and never pushes.
It reminds you when this file has no entry for the version.

The release (`.github/workflows/ci.yml`, the job `release`) has two files: `shamsaink-vX.Y.Z-kobo.tgz` and its
`.sha256`. The package's paths start at the Kobo's root, so it installs two ways (`docs/devices/kobo/install.md`):
over USB as `.kobo/KoboRoot.tgz`, or over SSH with `make kobo-release-install V=X.Y.Z`.
