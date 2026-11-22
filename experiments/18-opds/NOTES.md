# Experiment 18: OPDS, reading catalogues and downloading books on the device

For stage 7's OPDS (`docs/ROADMAP.md`): what real catalogues send, an OPDS 1.2 parser into go-toolkit's model, and
what a feed and a download cost on the Kobo and the Kindle. Done 2026-11-22.

## Why our own parser, on go-toolkit's model

go-toolkit (v0.16.2, and its main branch) only recognises OPDS media types; it does not parse feeds, and no issue or
PR plans it (asked on EDRLab's Discord 2026-11-22: whether it is planned, a PR welcome, or libopds2-go the route). **Their
answer (2026-11-22): Swift's OPDS support is to be vastly reworked very soon; wait for that before porting anything
to the Go toolkit.** So the parser stays ours (`reader/opds` when built) and is not offered upstream until then; the
rest of the reader sees only a small surface (a page of books and menus, the link to download), so a change of model
touches the parser alone. Asked back the same day: a timeline, or an issue to follow.
libopds2-go (OPDS 2, reads OPDS 1) is archived since 2021 and has a model of its own. Readium's Kotlin and Swift
toolkits turn every OPDS entry into a Readium `Publication`, so this does the same in Go: each book a
`manifest.Manifest`, the page around them a small `Feed` (links, facets, groups, navigation), the OPDS link properties
(count, price, indirect acquisition) in `Link.Properties` as Kotlin has them. Kotlin's `OPDS1Parser` is the reference,
read, not copied. Written so it can be offered upstream if Readium wants it, or stay ours.

- `opds/`: the parser (`ParseOPDS1(r, base)`), read as a stream, one entry at a time; links resolved against the
  feed's address, templated ones kept as templates; Atom's elements by namespace, Dublin Core's by name (feeds use
  both its namespaces); forgiving XML (HTML's entities, Latin-1). A book is an entry with an acquisition link, any
  other entry a menu. `go test ./opds` covers what no live feed showed (prices, indirect acquisition, facets, groups).
- `cmd/survey`: reads feeds (files or addresses, `-pages N` through next links), says what each uses, times the parse.
- `cmd/fetch`: downloads as the reader would (streamed to storage, the updater's timeouts and CA fallback), reports
  time, peak memory and CPU.
- `./go.sh` runs Go in `eink-go`; ARM builds `-tags readium_nocloud` (without it the survey is 32 MB, not 7).

## The catalogues (2026-11-22)

| Catalogue | Open? | Pages | Books have | Downloads |
|---|---|---|---|---|
| FarsiEPUB (`/opds/catalog.xml`) | yes | one page (1 book) | id, updated, language, summary | open-access EPUB; **answers 206** (below) |
| Standard Ebooks (`/feeds/opds`) | **no**: HTTP Basic, a Patrons Circle email | - | - | - |
| Gutenberg (`/ebooks.opds/`) | down for maintenance all day (503) | - | - | - |
| Ebooksgratuits (French, 3,278 books) | yes | 100 a page, next/prev, OpenSearch, author and category menus | id (a page's URL), language, summary; **no updated** | `acquisition` (not open-access) EPUB, PDF, MOBI, zip; a script that redirects to the file, its type `application/force-download`, its name in `Content-Disposition` |
| Wikisource export (English) | yes | **one page of 711 books, 1.1 MB** | id, updated, language | generic `acquisition`; EPUB made on request |

Feedbooks' and Internet Archive's old catalogues are gone (404, timeout); ManyBooks refuses (403).

- **Ids:** FarsiEPUB's entry id (`urn:farsiepub:library:buried-alive`) is not its EPUB's `dc:identifier`
  (`urn:uuid:993e…`). So new editions are matched by the catalogue's entry id (ROADMAP's option 1), not the book's own.
- **FarsiEPUB's bug:** `/books/{name}.epub` answers a plain GET with `206` and `content-range: bytes NaN-191586/191587`.
  R2 fills in `object.range` even when no range was asked for, and `deploy/src/books.ts` sends 206 whenever it is set;
  the tests' fake bucket returns none, so they pass. Our updater's check (`StatusCode != 200`) would refuse such a
  book. **Fixed in FarsiEPUB the same day** (deployed by the maintainer): 206 only with a `Range` header, and the range
  read by its values, since live ranged GETs got `bytes NaN-...` too (R2's range object has a `suffix` key even for an
  offset): a resumed download was broken as well.

## On the devices

Static ARMv7 builds, run from RAM. The Kobo with ShamsaInk running (its second core off, as in 17), the Kindle with it
not running (two cores). Parse: best of 5. Downloads over the home Wi-Fi, twice each.

| | Kobo Libra Colour | Kindle PW 11th gen |
|---|---|---|
| A page of 100 books (Ebooksgratuits, 242 KB) | 65 ms; 2.3 MB allocated, 0.3 MB kept | 163 ms |
| 711 books (Wikisource, 1.1 MB) | 385 ms; 9.7 MB allocated, 1.3 MB kept | 693 ms |
| FarsiEPUB's book (191 KB, HTTPS) | 0.58–0.63 s, mostly the first answer | 0.75–0.77 s |
| A release package (13.6 MB, GitHub, HTTPS) | 3.8–4.4 s, 3.1–3.6 MB/s, CPU 3.1 s | 6.0–6.6 s, 2.1–2.3 MB/s, CPU 6.0 s |
| Peak memory of a download | 9–10 MB, whatever its size | 10 MB |

- **Parsing is no concern:** a page of 100 books in under 0.2 s on either; even Wikisource's single page under 0.7 s.
  Read page by page as the feed pages, a list never holds more than a page.
- **A download's memory is flat** (streamed to storage). **Its time is the CPU's:** CPU time is most of the wall time,
  so HTTPS decryption on one core (the Kobo) or a slow one (the Kindle) is the limit, not Wi-Fi. A narrated book of
  98 MB: about 30 s on the Kobo, 45 on the Kindle. Not chased yet: which cipher is negotiated (Go has no AES
  instructions on 32-bit ARM), and the same download over plain HTTP to tell TLS from the rest.

## For the reader

- The parser is ready to move into `reader/opds`, ours until Readium's Swift rework settles the shape of a Go port;
  OPDS 2 later through go-toolkit's manifest JSON.
- A book's file named from `Content-Disposition`, else the final address; checked an EPUB by its contents (go-toolkit's
  sniffing), not the server's type; a 206 accepted only for a range we asked for.
- Which link: open-access or plain acquisition, EPUB; buy, borrow, sample and indirect ones left out for now.
- New editions: the catalogue's address and the entry's id recorded at download (ROADMAP's option 1).
- Standard Ebooks needs a login: not a catalogue to ship; a login is later work. Gutenberg to try again.
- A progress bar for downloads over a few seconds (the updater has one).

## Left

- Gutenberg's feed (down for maintenance all of 2026-11-22): its pages, ids, `<updated>` and download links.
- Why a download is CPU-bound: the cipher negotiated, and the same file over plain HTTP.
- After the deploy: a plain GET of a FarsiEPUB book answered 200, a ranged one 206 with real numbers, from the Kobo.
