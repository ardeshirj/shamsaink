"""perf/summarize.py: the benchmarks' results (perf/out/DEVICE/READER-MODE/times.tsv, from devtest's time and mem steps
and perf/koreader.sh) summed up: per device, reader, mode and step (start, open, turn, reopen; memory), the median of
every timing, each run's own median (so the runs' spread shows) and how many turns flashed, drawn as charts; beside
them the devices' own readers, measured once by hand (perf/reference/).

    python3 -I perf/summarize.py [--since YYYY-MM-DD] [--png FILE]   writes perf/out/summary.html (the charts, one file, nothing
                                                         fetched: open it in a browser) and summary.json (its data);
                                                         --png: turn, open and reopen as a picture, for the README
                                                         (docs/images/perf.png), by Google Chrome headless

A times.tsv line: run, name (start-1, open-2, turn-12, reopen-3, reading), kind (time or mem), then for time the last
full-page update's ms, the last update's ms, the full-page waveforms, whether one flashed and the reader's own line;
for mem the reader's resident memory and the available memory, in kB.
"""
import glob
import json
import os
import statistics
import sys

ROOT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "out")
STEPS = ["start", "open", "turn", "reopen"]
NAMES = {"start": "Start the app", "open": "First open", "turn": "Page turn", "reopen": "Reopen"}


def load(since):
    rows = []
    for path in sorted(glob.glob(os.path.join(ROOT, "*", "*", "times.tsv"))):
        device = os.path.basename(os.path.dirname(os.path.dirname(path)))
        reader, _, mode = os.path.basename(os.path.dirname(path)).rpartition("-")
        for line in open(path, encoding="utf-8"):
            f = line.rstrip("\n").split("\t")
            if len(f) < 5 or f[0] < since:
                continue
            rows.append({"device": device, "reader": reader, "mode": mode, "run": f[0], "name": f[1], "kind": f[2],
                         "v1": int(f[3]), "v2": int(f[4]), "modes": f[5] if len(f) > 5 else "",
                         "flash": len(f) > 6 and f[6] == "true"})
    return rows


def summarize(rows):
    out = {}
    for r in rows:
        key = (r["device"], r["reader"], r["mode"])
        d = out.setdefault(key, {"runs": set(), "steps": {}, "mem": []})
        d["runs"].add(r["run"])
        if r["kind"] == "mem":
            d["mem"].append((r["v1"], r["v2"]))
            continue
        step = r["name"].rsplit("-", 1)[0]
        s = d["steps"].setdefault(step, {"all": [], "by_run": {}, "flashed": 0, "modes": set()})
        s["all"].append(r["v1"])
        s["by_run"].setdefault(r["run"], []).append(r["v1"])
        s["flashed"] += r["flash"]
        s["modes"].update(m for m in r["modes"].split(",") if m)
    result = []
    for (device, reader, mode), d in sorted(out.items()):
        e = {"device": device, "reader": reader, "mode": mode, "runs": len(d["runs"]), "steps": {}}
        for step, s in d["steps"].items():
            run_medians = [statistics.median(v) for v in s["by_run"].values()]
            e["steps"][step] = {"median_ms": statistics.median(s["all"]), "n": len(s["all"]), "min_ms": min(s["all"]),
                                "max_ms": max(s["all"]), "run_medians_ms": run_medians, "flashed": s["flashed"],
                                "waveforms": sorted(s["modes"])}
        if d["mem"]:
            e["memory_mb"] = round(statistics.median(m[0] for m in d["mem"]) / 1024)
            e["available_mb"] = round(statistics.median(m[1] for m in d["mem"]) / 1024)
        result.append(e)
    return result


def reference():
    """The devices' own readers, measured once by hand in experiment 17 (perf/reference/*.json), drawn beside the runs."""
    out = []
    for path in sorted(glob.glob(os.path.join(os.path.dirname(os.path.abspath(__file__)), "reference", "*.json"))):
        out += json.load(open(path, encoding="utf-8"))
    return out


def lines(result):
    """A few lines for the terminal: each reader's medians, a device and mode at a time."""
    out = []
    for e in result:
        parts = [f"{NAMES[st].lower()} {e['steps'][st]['median_ms'] / 1000:.2f} s" if st != "turn" else
                 f"turn {e['steps'][st]['median_ms']:.0f} ms" for st in STEPS if st in e["steps"]]
        if "memory_mb" in e:
            parts.append(f"{e['memory_mb']} MB")
        out.append(f"{e['device']}, {e['mode']}, {e['reader']} ({e['runs']} runs): " + ", ".join(parts))
    return "\n".join(out)


PAGE = open(os.path.join(os.path.dirname(os.path.abspath(__file__)), "summary.template.html"), encoding="utf-8").read()


CHROME = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"


def png(page, out):
    """The page's #readme view (page turn, first open and reopen, light mode, the devices side by side: the README's
    table) as a picture at twice the size, as tall as it is (the view puts its height in the page's title)."""
    import re
    import subprocess
    url = "file://" + page + "#readme"
    dom = subprocess.run([CHROME, "--headless=new", "--disable-gpu", "--window-size=1240,3000", "--dump-dom", url],
                         capture_output=True, text=True).stdout
    m = re.search(r"<title>height (\d+)</title>", dom)
    height = int(m.group(1)) if m else 700
    subprocess.run([CHROME, "--headless=new", "--disable-gpu", "--hide-scrollbars", "--force-device-scale-factor=2",
                    f"--window-size=1240,{height}", "--screenshot=" + os.path.abspath(out), url],
                   capture_output=True)
    print("picture: " + out)


def main():
    since = ""
    if "--since" in sys.argv:
        since = sys.argv[sys.argv.index("--since") + 1]
    result = summarize(load(since)) + reference()
    if not result:
        print("no results in " + ROOT)
        return
    os.makedirs(ROOT, exist_ok=True)
    with open(os.path.join(ROOT, "summary.json"), "w", encoding="utf-8") as f:
        json.dump(result, f, indent=1)
    with open(os.path.join(ROOT, "summary.html"), "w", encoding="utf-8") as f:
        f.write(PAGE.replace("/*DATA*/[]", json.dumps(result)))
    print(lines(result))
    print("charts: " + os.path.join(ROOT, "summary.html"))
    if "--png" in sys.argv:
        png(os.path.join(ROOT, "summary.html"), sys.argv[sys.argv.index("--png") + 1])


if __name__ == "__main__":
    main()
