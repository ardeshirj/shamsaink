"""Phosphor's regular and fill fonts cut to ICONS: Phosphor.ttf and Phosphor-Fill.ttf here, a few KB each, and
codepoints.txt (each icon's name and its character, for the templates). Run by subset.sh."""
import re
import subprocess
import urllib.request

VERSION = "2.1.2"  # @phosphor-icons/web
ICONS = """magnifying-glass list sun headphones arrow-left caret-left caret-right x list-bullets bookmark-simple text-aa
play pause skip-back skip-forward highlighter-circle note-pencil copy speaker-high moon arrows-clockwise
arrow-counter-clockwise""".split()
BASE = f"https://cdn.jsdelivr.net/npm/@phosphor-icons/web@{VERSION}/src"

fetch = lambda path: urllib.request.urlopen(f"{BASE}/{path}").read()
css = fetch("regular/style.css").decode()
codes = []
for name in ICONS:
    m = re.search(r"\.ph-" + re.escape(name) + r':before\s*\{\s*content:\s*"\\([0-9a-f]+)"', css)
    if not m:
        raise SystemExit(f"no icon {name}")
    codes.append((name, m.group(1)))
with open("codepoints.txt", "w") as f:
    f.writelines(f"{n} {c}\n" for n, c in codes)
unicodes = ",".join("U+" + c for _, c in codes)
for weight, font in (("regular", "Phosphor"), ("fill", "Phosphor-Fill")):
    with open(f"/tmp/{font}.ttf", "wb") as f:
        f.write(fetch(f"{weight}/{font}.ttf"))
    subprocess.run(["pyftsubset", f"/tmp/{font}.ttf", f"--unicodes={unicodes}", f"--output-file={font}.ttf",
                    "--no-hinting", "--layout-features=", "--name-IDs=*", "--name-legacy"], check=True)
