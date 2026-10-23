// A small C interface to crengine for Go (cgo), modelled on koreader-base's cre.cpp (KOReader's Lua glue).
#ifndef CRE_WRAP_H
#define CRE_WRAP_H

#ifdef __cplusplus
extern "C" {
#endif

typedef struct CreDoc CreDoc;
typedef struct { int left, top, right, bottom; } CreRect;

// Once per process: the font manager, the cache of laid-out books (cache_dir NULL: none) and hyphenation patterns.
void cre_init(const char *cache_dir, int cache_mb, const char *hyph_dir);
int cre_register_font(const char *path);

CreDoc *cre_new(int width, int height);
void cre_close(CreDoc *d);
void cre_set_int(CreDoc *d, const char *name, int value);
void cre_set_str(CreDoc *d, const char *name, const char *value);
void cre_set_font_size(CreDoc *d, int px);
void cre_set_interline(CreDoc *d, int percent);
int cre_set_stylesheet(CreDoc *d, const char *css_path);

int cre_load(CreDoc *d, const char *path);
void cre_render(CreDoc *d);
int cre_page_count(CreDoc *d);
// Draws page (from 0) into buf, width*height bytes of 8-bit grey; it becomes the page shown.
void cre_draw_page(CreDoc *d, int page, unsigned char *buf);

// The element with the given id in the spine item at spine_index (as the EPUB names them; crengine renames ids
// when it joins a book's files into one document). cre_id_page: the page it starts on, -1 if not found.
int cre_id_page(CreDoc *d, int spine_index, const char *id);
// The boxes of its text on the page shown, in pixels (one per line it spans); returns how many, at most max.
int cre_id_rects(CreDoc *d, int spine_index, const char *id, CreRect *out, int max);

#ifdef __cplusplus
}
#endif

#endif
