"""Builds a narrated EPUB 3 (media overlays, a word at a time) from a Wikisource chapter, its recording and what Whisper
heard in it (hear.py), as FarsiEPUB's narrated books are laid out: a span with an id per word, a SMIL par per word.

    python3 build.py book/intro.wiki book/intro.mp3 out/heard.json out/NAME.epub "TITLE" "AUTHOR" ar

The page's letters and Whisper's are matched as two strings (vowel marks, spaces and punctuation left out, letter
forms made the same, as FarsiEPUB's align.py does for Farsi); a word starts when the heard word holding its first
matched letter starts, and lasts until the next word starts. A word with no letter matched takes its share of the time
between its matched neighbours. Prints how much was matched.
"""
import difflib, html, json, re, sys, uuid, zipfile

wiki, mp3, heard_path, out, title, author, lang = sys.argv[1:8]

FORMS = str.maketrans({"أ": "ا", "إ": "ا", "آ": "ا", "ٱ": "ا", "ى": "ي", "ة": "ه", "ؤ": "و", "ئ": "ي", "ک": "ك", "ی": "ي"})
MARKS = re.compile("[ً-ٰٟـ‌‍ء]")  # vowel marks, superscript alef, tatweel, ZWNJ/ZWJ, lone hamza


def letters(s):
    return [c for c in MARKS.sub("", s).translate(FORMS) if c.isalpha()]


def strip_templates(s):
    while True:
        t = re.sub(r"\{\{[^{}]*\}\}", "", s)
        if t == s:
            return t
        s = t


# The chapter: its heading and paragraphs, wiki markup taken out.
text = strip_templates(open(wiki, encoding="utf-8").read())
text = re.sub(r"</?onlyinclude>|<ref[^>]*>.*?</ref>|<[^>]+>", "", text, flags=re.S)
text = re.sub(r"\[\[(?:[^|\]]*\|)?([^\]]*)\]\]", r"\1", text).replace("'''", "").replace("''", "")
heading, paras = title, []
for line in text.splitlines():
    line = line.strip()
    if not line:
        continue
    m = re.match(r"=+\s*(.*?)\s*=+$", line)
    if m:
        heading = m.group(1)
    else:
        paras.append(line)

# Every word of the page, in reading order: (block, its text); the heading is block 0.
blocks = [heading] + paras
words = []  # (block index, word)
for b, block in enumerate(blocks):
    for w in block.split():
        words.append((b, w))

heard = json.load(open(heard_path, encoding="utf-8"))
page, word_of_page = [], []
for i, (_, w) in enumerate(words):
    ls = letters(w)
    page += ls
    word_of_page += [i] * len(ls)
said, word_of_said = [], []
for j, (w, _, _) in enumerate(heard):
    ls = letters(w)
    said += ls
    word_of_said += [j] * len(ls)
match = {}
for a, b, n in difflib.SequenceMatcher(None, page, said, autojunk=False).get_matching_blocks():
    for x in range(n):
        match[a + x] = b + x

# Each page word's start: the heard word of its first matched letter.
starts = [None] * len(words)
first_letter = {}
for i, w in enumerate(word_of_page):
    first_letter.setdefault(w, []).append(i)
for k in range(len(words)):
    mine = [i for i in first_letter.get(k, []) if i in match]
    if mine:
        starts[k] = heard[word_of_said[match[mine[0]]]][1]
# keep them in order (a late match never makes time run backwards), then fill the gaps evenly
last = -1.0
for k in range(len(words)):
    if starts[k] is not None and starts[k] <= last:  # two words on one heard word: the second shares the gap after
        starts[k] = None
    elif starts[k] is not None:
        last = starts[k]
end_all = heard[-1][2] if heard else 0.0
k = 0
while k < len(words):
    if starts[k] is None:
        j = k
        while j < len(words) and starts[j] is None:
            j += 1
        a = starts[k - 1] if k > 0 else 0.0
        b = starts[j] if j < len(words) else end_all
        for n, m in enumerate(range(k, j)):
            starts[m] = round(a + (b - a) * (n + 1) / (j - k + 1), 3)
        k = j
    else:
        k += 1
ends = [starts[k + 1] if k + 1 < len(words) else end_all for k in range(len(words))]
matched_words = sum(1 for k in range(len(words)) if any(i in match for i in first_letter.get(k, [])))

# The EPUB.
ids = [f"w{k + 1}" for k in range(len(words))]
body = []
for b, block in enumerate(blocks):
    spans = " ".join(f'<span id="{ids[k]}">{html.escape(w)}</span>' for k, (bb, w) in enumerate(words) if bb == b)
    body.append(f"<h1>{spans}</h1>" if b == 0 else f"<p>{spans}</p>")
xhtml = f"""<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="{lang}" xml:lang="{lang}" dir="rtl">
<head><meta charset="UTF-8"/><title>{html.escape(heading)}</title><link rel="stylesheet" href="style.css"/></head>
<body>
{chr(10).join(body)}
</body>
</html>
"""
pars = "\n".join(f'<par id="p{k + 1}"><text src="chapter.xhtml#{ids[k]}"/><audio src="audio.mp3" clipBegin="{starts[k]:.3f}s" clipEnd="{ends[k]:.3f}s"/></par>'
                 for k in range(len(words)))
smil = f"""<?xml version="1.0" encoding="UTF-8"?>
<smil xmlns="http://www.w3.org/ns/SMIL" xmlns:epub="http://www.idpf.org/2007/ops" version="3.0">
<body>
<seq id="seq1" epub:textref="chapter.xhtml">
{pars}
</seq>
</body>
</smil>
"""
def clock(s):
    return f"{int(s // 3600)}:{int(s % 3600 // 60):02d}:{s % 60:06.3f}"
book_id = f"urn:uuid:{uuid.uuid5(uuid.NAMESPACE_URL, out)}"
opf = f"""<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id" xml:lang="{lang}">
<metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
<dc:identifier id="id">{book_id}</dc:identifier>
<dc:title>{html.escape(title)}</dc:title>
<dc:creator>{html.escape(author)}</dc:creator>
<dc:language>{lang}</dc:language>
<meta property="dcterms:modified">2026-11-12T00:00:00Z</meta>
<meta property="media:duration" refines="#chapter-mo">{clock(end_all)}</meta>
<meta property="media:duration">{clock(end_all)}</meta>
<meta property="media:active-class">-epub-media-overlay-active</meta>
</metadata>
<manifest>
<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
<item id="chapter" href="chapter.xhtml" media-type="application/xhtml+xml" media-overlay="chapter-mo"/>
<item id="chapter-mo" href="chapter.smil" media-type="application/smil+xml"/>
<item id="audio" href="audio.mp3" media-type="audio/mpeg"/>
<item id="css" href="style.css" media-type="text/css"/>
</manifest>
<spine page-progression-direction="rtl">
<itemref idref="chapter"/>
</spine>
</package>
"""
nav = f"""<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="{lang}" dir="rtl">
<head><meta charset="UTF-8"/><title>{html.escape(title)}</title></head>
<body><nav epub:type="toc"><ol><li><a href="chapter.xhtml">{html.escape(heading)}</a></li></ol></nav></body>
</html>
"""
css = "html { direction: rtl; } h1 { text-align: center; } p { text-align: justify; }\n"
container = """<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
<rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>
"""
with zipfile.ZipFile(out, "w") as z:
    z.writestr(zipfile.ZipInfo("mimetype"), "application/epub+zip", compress_type=zipfile.ZIP_STORED)
    z.writestr("META-INF/container.xml", container, compress_type=zipfile.ZIP_DEFLATED)
    for name, data in [("content.opf", opf), ("nav.xhtml", nav), ("chapter.xhtml", xhtml), ("chapter.smil", smil), ("style.css", css)]:
        z.writestr("OEBPS/" + name, data, compress_type=zipfile.ZIP_DEFLATED)
    z.write(mp3, "OEBPS/audio.mp3", compress_type=zipfile.ZIP_STORED)

print(f"{len(words)} words on the page, {len(heard)} heard; {matched_words} words matched "
      f"({100 * matched_words / max(len(words), 1):.0f}%), letters {len(match)} of {len(page)} "
      f"({100 * len(match) / max(len(page), 1):.0f}%); {end_all:.1f} s")
