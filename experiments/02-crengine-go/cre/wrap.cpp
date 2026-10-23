// Each function follows its counterpart in koreader-base's cre.cpp (6e4bc81) without the Lua.
#include "crsetup-ko.h"
#include "crengine.h"
#include "lvdocview.h"

#include "wrap.h"

struct CreDoc {
    LVDocView *view;
    ldomDocument *dom;
};

void cre_init(const char *cache_dir, int cache_mb, const char *hyph_dir) {
    InitFontManager(lString8());
    if (cache_dir) {
        compressCachedData(true);
        setStorageMaxUncompressedSizeFactor(40);
        ldomDocCache::init(lString32(cache_dir), (lvsize_t)cache_mb * 1024 * 1024);
    }
    HyphMan::initDictionaries(lString32(hyph_dir));
}

int cre_register_font(const char *path) {
    return fontMan->RegisterFont(lString8(path)) ? 1 : 0;
}

CreDoc *cre_new(int width, int height) {
    CreDoc *d = new CreDoc();
    d->view = new LVDocView(-1, true); // bitsPerPixel=-1, noDefaultDocument=true
    d->view->setViewMode(DVM_PAGES, -1);
    d->view->Resize(width, height);
    d->dom = NULL;
    return d;
}

void cre_close(CreDoc *d) {
    d->view->close();
    delete d->view;
    delete d;
}

void cre_set_int(CreDoc *d, const char *name, int value) {
    CRPropRef props = LVCreatePropsContainer();
    props->setInt(name, value);
    d->view->propsApply(props);
}

void cre_set_str(CreDoc *d, const char *name, const char *value) {
    CRPropRef props = LVCreatePropsContainer();
    props->setString(name, value);
    d->view->propsApply(props);
}

void cre_set_font_size(CreDoc *d, int px) { d->view->setFontSize(px); }

void cre_set_interline(CreDoc *d, int percent) { d->view->setDefaultInterlineSpace(percent); }

int cre_set_stylesheet(CreDoc *d, const char *css_path) {
    lString8 css;
    if (!LVLoadStylesheetFile(lString32(css_path), css))
        return 0;
    d->view->setStyleSheet(css, false);
    return 1;
}

int cre_load(CreDoc *d, const char *path) {
    d->view->LoadDocument(path, false);
    d->dom = d->view->getDocument();
    return d->dom != NULL;
}

void cre_render(CreDoc *d) { d->view->Render(); }

int cre_page_count(CreDoc *d) { return d->view->getPageCount(); }

void cre_draw_page(CreDoc *d, int page, unsigned char *buf) {
    int w = d->view->GetWidth(), h = d->view->GetHeight();
    d->view->goToPage(page, false, true, false);
    LVGrayDrawBuf draw(w, h, 8, buf);
    d->view->Draw(draw, false);
}

static ldomXPointer idPointer(CreDoc *d, int spine_index, const char *id) {
    // epubfmt.cpp: ids in spine item i become "_doc_fragment_<i>_ <id>" (convertId, lvtinydom.cpp).
    lString32 s = cs32("#_doc_fragment_") + fmt::decimal(spine_index) + "_ " + Utf8ToUnicode(id);
    return d->dom->createXPointer(s);
}

int cre_id_page(CreDoc *d, int spine_index, const char *id) {
    ldomXPointer xp = idPointer(d, spine_index, id);
    if (xp.isNull())
        return -1;
    return d->view->getBookmarkPage(xp);
}

// docToWindowRect from cre.cpp: document to page coordinates, clipped to the page shown; false if not on it.
static bool docToWindowRect(LVDocView *tv, lvRect &rc) {
    lvPoint topLeft = rc.topLeft();
    lvPoint bottomRight = rc.bottomRight();
    bool topIn = tv->docToWindowPoint(topLeft);
    bool bottomIn = tv->docToWindowPoint(bottomRight, true);
    if (!topIn || !bottomIn)
        return false; // a word cut by the page edge: not needed for this trial
    rc.setTopLeft(topLeft);
    rc.setBottomRight(bottomRight);
    return true;
}

int cre_id_rects(CreDoc *d, int spine_index, const char *id, CreRect *out, int max) {
    ldomXPointer xp = idPointer(d, spine_index, id);
    if (xp.isNull())
        return 0;
    ldomXRange range(xp.getNode());
    LVArray<lvRect> rects;
    range.getSegmentRects(rects, false);
    int n = 0;
    for (int i = 0; i < rects.length() && n < max; i++) {
        lvRect r = rects[i];
        if (r.isEmpty() || !docToWindowRect(d->view, r))
            continue;
        out[n].left = r.left;
        out[n].top = r.top;
        out[n].right = r.right;
        out[n].bottom = r.bottom;
        n++;
    }
    return n;
}
