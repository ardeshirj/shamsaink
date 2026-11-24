# Experiment 19: the library's covers in colour (Kobo Libra Colour), 2026-11-24

The library keeps each book's cover as a small grey PNG (`reader/library/covers.go`), and draws every page grey; only
the opening and sleep screens are shown in colour (`Screen.ShowRGBA`). The question: what colour covers would cost on
the Kobo, and how they look on the library's own refreshes.

`main.go` (a static ARMv7 binary, `colourcovers -dir DIR [-formats] [-show]`): for each book with a cover picture under
DIR/books, its small cover made grey (the library's `MakeCover`) and in colour (the same scaling, colour kept), timed;
each read back, timed; `-formats` keeps the colour covers as raw RGBA, raw RGB and JPEG too and reads each cold (the
file cache emptied) and warm; `-show` draws a page like the library's (four a row, twelve) grey then in colour, and a
panel opened and closed over the covers, each step timed to the ink.

Books: twelve of Standard Ebooks' (their covers full-colour paintings; downloaded for the test, not kept in the
repository): Dracula, Jane Eyre, The Great Gatsby, The War of the Worlds, Moby-Dick, Sherlock Holmes, Alice's Adventures in
Wonderland (Tenniel), Frankenstein, The Picture of Dorian Gray, Pride and Prejudice, Treasure Island, The Time Machine.
Tiles 269×403, the library's size on the Kobo.

## Making and reading (mean of 12)

| | grey PNG (today) | colour PNG |
|---|---|---|
| made (once, in the background) | 309 ms | 624 ms |
| kept file | 67 KB | 178 KB |
| read (warm) | 8.9 ms | 26.2 ms |

## How a colour cover is kept (mean of 12, read as the library needs it: an RGBA picture)

| | file | read cold | read warm |
|---|---|---|---|
| PNG | 178 KB | 34.6 ms | 29.0 ms |
| raw RGBA (8 bytes of size, then the pixels) | 425 KB | 5.7 ms | 1.6 ms |
| raw RGB (three bytes a pixel, made RGBA as read) | 318 KB | 8.0 ms | 3.2 ms |
| JPEG, quality 90 | 34 KB | 22.5 ms | 16.8 ms |

Raw pixels read faster than today's grey PNG, even cold: twelve in 70–100 ms. The cost is the storage: 500 books
~160 MB (RGB) or ~210 MB (RGBA), on a 32 GB Kobo.

## On the screen (to the ink)

| step | ms |
|---|---|
| grey page, GC16 with a flash | 499 |
| colour page, GC16 with a flash | 539 |
| colour page, REAGL (a page turn) | 470 |
| a panel opened over the covers, REAGL | 388 |
| the panel closed, the covers back, REAGL | 396 |
| the same, GL16: opened / closed | 387 / 408 |

(The grey page's panel step, 1104 ms, timed a full GC16 refresh with it: not comparable.)

Seen by the maintainer: the covers in colour "look nice and clean", and come back clean when a panel closes over them.

## Conclusion

Colour covers cost the Kobo almost nothing on the screen (~40 ms more for a whole page) and nothing to read, kept as
raw pixels; making them takes twice as long, in the background, once a book. The Kindle has no colour: its covers stay
grey (and could be kept raw too, for the same quicker reads).
