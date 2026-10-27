// The Blitz engine's C interface (src/capi.rs), for Go through cgo.
#ifndef BLITZ_ENGINE_H
#define BLITZ_ENGINE_H

#include <stddef.h>
#include <stdint.h>

typedef struct Book Book;
typedef struct Fonts Fonts;

// The reader's file callback: call be_buf_put(dst, ...) with the file's bytes, or not at all if there is none.
typedef void (*be_read_fn)(uintptr_t ctx, const uint8_t *path, size_t path_len, void *dst);
void be_buf_put(void *dst, const uint8_t *data, size_t len);

Book *be_open(be_read_fn read, uintptr_t ctx, const uint8_t *const *spine, const size_t *spine_len, size_t n,
              uint32_t width, uint32_t height, uint32_t margin, float font_size);
// Opening as experiment 08 does: the reader lays out the document it opens at, then the rest, and keeps a cache file.
Fonts *be_fonts(const uint8_t *dir, size_t dir_len);  // fallback fonts in a folder, read once; Mono, Serif, Sans in the name
void be_fonts_free(Fonts *f);                         // books made with it keep their own copy
Book *be_new(be_read_fn read, uintptr_t ctx, const uint8_t *const *spine, const size_t *spine_len, size_t n,
             const Fonts *fonts /* or NULL */, uint32_t width, uint32_t height, uint32_t margin, float font_size,
             float line_height /* 0: the book's */);
void be_layout(Book *b, size_t doc, size_t *first, size_t *last);  // lays out document doc: its pages are [first, last)
int be_layout_next(Book *b);                          // paginates the next document not paginated yet: 1, or 0 if all are
uint8_t *be_save(Book *b, size_t *len);               // the cache (free with be_bytes_free); NULL until all are paginated
void be_bytes_free(uint8_t *ptr, size_t len);
int be_restore(Book *b, const uint8_t *data, size_t len);  // a cache on a book just made: 1 restored, 0 refused
size_t be_pages(Book *b);
void be_page_info(Book *b, size_t page, size_t *doc, size_t *lines);
void be_draw(Book *b, size_t page, uint8_t *out);  // width*height bytes of 8-bit grey
float be_line_spacing(Book *b, size_t doc);           // doc's most common line height over the text size (0: no text)
int64_t be_find(Book *b, size_t doc, const uint8_t *id, size_t id_len, int32_t *rects, size_t max, size_t *n);
void be_close(Book *b);

#endif
