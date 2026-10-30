//! What both variants share: the fonts, the page size, painting a document with plain vello_cpu to grey, PNG output.
use anyrender::{ImageRenderer as _, PaintScene as _};
use anyrender_vello_cpu::VelloCpuImageRenderer;
use blitz_dom::BaseDocument;
use blitz_paint::paint_scene;
use std::sync::Arc;

pub const W: u32 = 1236;
pub const H: u32 = 900; // menuHeight in reader/menu.go
/// what drawPanel gives the engine: the page's text size and margin; html gets them from here in both variants
pub const PAGE_CSS: &str = "html { font-size: 40px; padding: 40px 40px 900px 40px; }";
pub const BUTTONS: [&str; 13] = [
    "leave", "reset", "read-aloud", "size-dec", "size-inc", "lines-dec", "lines-inc", "margin-dec", "margin-inc",
    "clock", "dir-", "dir-rtl", "dir-ltr",
];

pub fn ms(t: std::time::Instant) -> f64 {
    t.elapsed().as_secs_f64() * 1e3
}

pub fn median(v: &mut Vec<f64>) -> f64 {
    v.sort_by(|a, b| a.partial_cmp(b).unwrap());
    v[v.len() / 2]
}

/// The fallback fonts in a folder as generic families (engine/src/lib.rs `fallback_fonts` + `font_context`).
pub fn font_context(dir: &str) -> parley::FontContext {
    use parley::fontique::{Blob, GenericFamily as G};
    let mut fonts = vec![];
    for e in std::fs::read_dir(dir).unwrap().flatten() {
        let name = e.file_name().to_string_lossy().into_owned();
        let generic = if name.contains("Mono") {
            "monospace"
        } else if name.contains("Serif") {
            "serif"
        } else if name.contains("Sans") {
            "sans-serif"
        } else {
            continue;
        };
        fonts.push((generic, std::fs::read(e.path()).unwrap()));
    }
    let mut ctx = parley::FontContext::default();
    let mut families: Vec<(G, parley::fontique::FamilyId)> = vec![];
    for (generic, data) in &fonts {
        let gs: &[G] = match *generic {
            "serif" => &[G::Serif],
            "sans-serif" => &[G::SansSerif, G::SystemUi, G::UiSansSerif],
            "monospace" => &[G::Monospace, G::UiMonospace],
            _ => &[],
        };
        for (id, _) in ctx.collection.register_fonts(Blob::new(Arc::new(data.clone())), None) {
            for &g in gs {
                if !families.contains(&(g, id)) {
                    families.push((g, id));
                }
            }
        }
    }
    for g in [G::Serif, G::SansSerif, G::SystemUi, G::UiSansSerif, G::Monospace, G::UiMonospace] {
        ctx.collection.set_generic_families(g, families.iter().filter(|(h, _)| *h == g).map(|(_, id)| *id));
    }
    ctx
}

pub struct Painter {
    r: VelloCpuImageRenderer,
    rgba: Vec<u8>,
}

impl Painter {
    pub fn new() -> Painter {
        Painter { r: VelloCpuImageRenderer::new(W, H), rgba: vec![] }
    }

    /// The document painted on white, W x H, as grey (the green channel, as the engine does).
    pub fn paint(&mut self, doc: &mut BaseDocument) -> Vec<u8> {
        self.r.reset();
        self.r.render_to_vec(
            |scene| {
                let page = peniko::kurbo::Rect::new(0.0, 0.0, W as f64, H as f64);
                scene.fill(peniko::Fill::NonZero, Default::default(), peniko::Color::WHITE, None, &page);
                paint_scene(scene, doc, 1.0, W, H, 0, 0);
            },
            &mut self.rgba,
        );
        self.rgba.chunks_exact(4).map(|p| p[1]).collect()
    }
}

/// The box of an element by its id, in document pixels (left, top, right, bottom).
pub fn rect_of(doc: &BaseDocument, id: blitz_dom::NodeId) -> [i32; 4] {
    let n = doc.get_node(id).unwrap();
    let p = n.absolute_position(0.0, 0.0);
    let s = n.final_layout().size;
    [p.x.floor() as i32, p.y.floor() as i32, (p.x + s.width).ceil() as i32, (p.y + s.height).ceil() as i32]
}

/// How low the buttons reach (drawPanel's `bottom`).
pub fn bottom(doc: &BaseDocument) -> usize {
    BUTTONS.iter().filter_map(|b| doc.get_element_by_id(b)).map(|id| rect_of(doc, id)[3]).max().unwrap_or(0) as usize
}

/// Cropped as drawPanel crops (the buttons' bottom + 30 + 4, a black line at the bottom), written as a grey PNG.
pub fn save(path: &str, grey: &[u8], bottom: usize) {
    let h = (bottom + 34).min(H as usize);
    let mut px = grey[..h * W as usize].to_vec();
    for p in &mut px[(h - 4) * W as usize..] {
        *p = 0;
    }
    let f = std::fs::File::create(path).unwrap();
    let mut e = png::Encoder::new(std::io::BufWriter::new(f), W, h as u32);
    e.set_color(png::ColorType::Grayscale);
    e.set_depth(png::BitDepth::Eight);
    e.write_header().unwrap().write_image_data(&px).unwrap();
}

/// The box (left, top, right, bottom) of the pixels that differ, and how many.
pub fn diff_box(a: &[u8], b: &[u8]) -> Option<([i32; 4], usize)> {
    let (mut bx, mut n) = ([i32::MAX, i32::MAX, i32::MIN, i32::MIN], 0);
    for (i, (p, q)) in a.iter().zip(b).enumerate() {
        if p != q {
            let (x, y) = ((i % W as usize) as i32, (i / W as usize) as i32);
            bx = [bx[0].min(x), bx[1].min(y), bx[2].max(x + 1), bx[3].max(y + 1)];
            n += 1;
        }
    }
    (n > 0).then_some((bx, n))
}

/// Peak resident memory of this process so far, MB (Linux).
pub fn max_rss_mb() -> f64 {
    std::fs::read_to_string("/proc/self/status")
        .ok()
        .and_then(|s| s.lines().find(|l| l.starts_with("VmHWM")).map(|l| l.to_string()))
        .and_then(|l| l.split_whitespace().nth(1).and_then(|k| k.parse::<f64>().ok()))
        .map(|k| k / 1024.0)
        .unwrap_or(0.0)
}
