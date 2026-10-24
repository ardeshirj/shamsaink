//! Glyphs drawn straight into the grey page from a cache of masks, as crengine and the Go engine do, instead of
//! vello_cpu turning every glyph into an outline and strips on every page (experiment 07). `Split` is the scene Blitz's
//! painter draws into: plain glyph runs (a solid colour, no skew, not huge) become placed masks; everything else is
//! recorded for vello_cpu, which then runs only on pages that have something else on them. Pictures still in their
//! files are decoded on the way, at the size drawn (`pictures`).

use anyrender::{Filter, Glyph, ImageRenderer as _, NormalizedCoord, Paint, PaintRef, PaintScene, recording::Scene};
use anyrender_vello_cpu::VelloCpuImageRenderer;
use peniko::kurbo::{Affine, Rect, Shape, Stroke, Vec2};
use peniko::{BlendMode, Color, Fill, FontData, ImageBrush, StyleRef};
use std::collections::HashMap;
use std::sync::Arc;

/// Horizontal positions a glyph is drawn at within a pixel (as vello_cpu's own atlas): 4 masks per glyph at most.
const SUBPIXEL: f32 = 4.0;
/// Larger text is left to vello_cpu.
const MAX_SIZE: f32 = 128.0;

#[derive(Clone, PartialEq, Eq, Hash)]
struct Key {
    font: u64,
    index: u32,
    glyph: u32,
    size: u32,
    hint: bool,
    coords: Vec<NormalizedCoord>,
    embolden: (u64, u64),
    subpixel: u8,
}

/// A glyph's coverage, its top-left corner relative to the glyph's origin on whole pixels.
struct Mask {
    left: i32,
    top: i32,
    width: usize,
    height: usize,
    alpha: Vec<u8>,
}

/// The masks drawn so far (None: the glyph has no ink, a space), and a renderer for each mask size.
#[derive(Default)]
pub struct Cache {
    masks: HashMap<Key, Option<Arc<Mask>>>,
    // a font file's key by its blob: every document loads the book's fonts again, as new blobs (experiment 08)
    fonts: HashMap<u64, u64>,
    /// masks drawn so far, and the time it took (ms)
    pub made: usize,
    pub made_ms: f64,
    renderers: HashMap<u16, (VelloCpuImageRenderer, Vec<u8>)>,
}

impl Cache {
    /// The masks drawn so far: glyph id, quarter-pixel position, width, height, coverage.
    pub fn masks(&self) -> Vec<(u32, u8, usize, usize, &[u8])> {
        self.masks
            .iter()
            .filter_map(|(k, m)| m.as_ref().map(|m| (k.glyph, k.subpixel, m.width, m.height, m.alpha.as_slice())))
            .collect()
    }

    /// A key for a font file that is the same in every document: its length and its first and last 16 KB, hashed once
    /// per blob.
    fn font_key(&mut self, font: &FontData) -> u64 {
        *self.fonts.entry(font.data.id()).or_insert_with(|| {
            use std::hash::{Hash, Hasher};
            let d = font.data.data();
            let mut h = std::collections::hash_map::DefaultHasher::new();
            d.len().hash(&mut h);
            d[..d.len().min(16384)].hash(&mut h);
            d[d.len().saturating_sub(16384)..].hash(&mut h);
            h.finish()
        })
    }

    /// The glyph drawn alone, black, by vello_cpu (the same code that drew it in the page before), on a square of
    /// three times the text size with the origin a quarter in and five eighths down, and cut to its ink.
    #[allow(clippy::too_many_arguments)]
    fn mask(&mut self, key: &Key, font: &FontData, size: f32, hint: bool, coords: &[NormalizedCoord], embolden: Vec2) -> Option<Arc<Mask>> {
        if let Some(m) = self.masks.get(key) {
            return m.clone();
        }
        let t = std::time::Instant::now();
        let s = (((3.0 * size).ceil() as u16).div_ceil(32) * 32).max(64);
        let (ox, oy) = (s as i32 / 4, s as i32 * 5 / 8);
        let (r, buf) = self.renderers.entry(s).or_insert_with(|| (VelloCpuImageRenderer::new(s as u32, s as u32), vec![]));
        r.reset();
        let glyph = Glyph { id: key.glyph, x: ox as f32 + key.subpixel as f32 / SUBPIXEL, y: oy as f32 };
        r.render_to_vec(
            |scene| {
                scene.draw_glyphs(font, size, hint, coords, embolden, Fill::NonZero, Color::BLACK, 1.0, Affine::IDENTITY, None, std::iter::once(glyph));
            },
            buf,
        );
        // the ink's bounds, in one pass row by row
        let s = s as usize;
        let (mut x0, mut x1, mut y0, mut y1) = (s, 0, s, 0);
        for (y, row) in buf.chunks_exact(4 * s).enumerate() {
            let mut inked = row.chunks_exact(4).enumerate().filter(|(_, p)| p[3] != 0).map(|(x, _)| x);
            if let Some(first) = inked.next() {
                let last = inked.last().unwrap_or(first);
                (x0, x1, y0, y1) = (x0.min(first), x1.max(last), y0.min(y), y);
            }
        }
        let m = match y0 < s {
            true => {
                let (width, height) = (x1 - x0 + 1, y1 - y0 + 1);
                let mut alpha = Vec::with_capacity(width * height);
                for y in y0..=y1 {
                    alpha.extend((x0..=x1).map(|x| buf[4 * (y * s + x) + 3]));
                }
                Some(Arc::new(Mask { left: x0 as i32 - ox, top: y0 as i32 - oy, width, height, alpha }))
            }
            false => None,
        };
        self.masks.insert(key.clone(), m.clone());
        self.made += 1;
        self.made_ms += t.elapsed().as_secs_f64() * 1e3;
        m
    }
}

/// A mask placed on the page: its glyph's origin on whole pixels, and the grey and opacity it is drawn in.
struct Placed {
    mask: Arc<Mask>,
    x: i32,
    y: i32,
    grey: u32,
    alpha: u32,
}

/// The scene Blitz's painter draws a page into.
pub struct Split<'c> {
    cache: &'c mut Cache,
    pictures: &'c mut crate::pictures::Cache,
    /// all text to vello_cpu too, as before the glyph layer
    pub text_to_rest: bool,
    pub rest: Scene,
    /// whether `rest` draws anything (layers alone do not)
    pub rest_draws: bool,
    placed: Vec<Placed>,
}

impl<'c> Split<'c> {
    pub fn new(cache: &'c mut Cache, pictures: &'c mut crate::pictures::Cache) -> Self {
        Split { cache, pictures, text_to_rest: false, rest: Scene::new(), rest_draws: false, placed: vec![] }
    }

    /// Each placed glyph's ink box on the page: x0, y0, x1, y1.
    pub fn boxes(&self) -> Vec<[i32; 4]> {
        self.placed
            .iter()
            .map(|p| {
                let (x, y) = (p.x + p.mask.left, p.y + p.mask.top);
                [x, y, x + p.mask.width as i32, y + p.mask.height as i32]
            })
            .collect()
    }

    /// Draws the placed glyphs over `out` (8-bit grey, `width` wide), rows [from, to) only.
    pub fn blit(&self, out: &mut [u8], width: usize, from: usize, to: usize) {
        for p in &self.placed {
            let m = &p.mask;
            let (x0, y0) = (p.x + m.left, p.y + m.top);
            for my in 0..m.height {
                let y = y0 + my as i32;
                if y < from as i32 || y >= to as i32 {
                    continue;
                }
                let row = &mut out[y as usize * width..(y as usize + 1) * width];
                let src = &m.alpha[my * m.width..(my + 1) * m.width];
                for (mx, &a) in src.iter().enumerate() {
                    let x = x0 + mx as i32;
                    if a == 0 || x < 0 || x >= width as i32 {
                        continue;
                    }
                    let a = a as u32 * p.alpha / 255;
                    let o = &mut row[x as usize];
                    *o = ((*o as u32 * (255 - a) + p.grey * a + 127) / 255) as u8;
                }
            }
        }
    }
}

impl anyrender::RenderContext for Split<'_> {}

impl PaintScene for Split<'_> {
    fn reset(&mut self) {
        self.rest.reset();
        self.rest_draws = false;
        self.placed.clear();
    }

    fn push_layer(
        &mut self,
        fill: Fill,
        blend: impl Into<BlendMode>,
        alpha: f32,
        transform: Affine,
        clip: &impl Shape,
        filter: Option<Arc<Filter>>,
        backdrop_filter: Option<Arc<Filter>>,
    ) {
        self.rest.push_layer(fill, blend, alpha, transform, clip, filter, backdrop_filter);
    }

    fn push_clip_layer(&mut self, fill: Fill, transform: Affine, clip: &impl Shape) {
        self.rest.push_clip_layer(fill, transform, clip);
    }

    fn pop_layer(&mut self) {
        self.rest.pop_layer();
    }

    fn stroke<'a>(&mut self, style: &Stroke, transform: Affine, brush: impl Into<PaintRef<'a>>, brush_transform: Option<Affine>, shape: &impl Shape) {
        self.rest_draws = true;
        self.rest.stroke(style, transform, brush, brush_transform, shape);
    }

    fn fill<'a>(&mut self, style: Fill, transform: Affine, brush: impl Into<PaintRef<'a>>, brush_transform: Option<Affine>, shape: &impl Shape) {
        let brush = brush.into();
        // Blitz fills the root's background even when it has none, in transparent: nothing to draw
        if let Paint::Solid(c) = brush
            && c.components[3] == 0.0
        {
            return;
        }
        self.rest_draws = true;
        if let Paint::Image(img) = brush
            && crate::pictures::is_file(img.image)
        {
            let to_brush = brush_transform.unwrap_or(Affine::IDENTITY);
            let Some((pic, scale)) = self.pictures.get(img.image, transform * to_brush) else { return };
            let brush = ImageBrush { image: &pic, sampler: img.sampler };
            self.rest.fill(style, transform, Paint::Image(brush), Some(to_brush * scale), shape);
            return;
        }
        self.rest.fill(style, transform, brush, brush_transform, shape);
    }

    fn draw_glyphs<'a, 's: 'a>(
        &'s mut self,
        font: &'a FontData,
        font_size: f32,
        hint: bool,
        normalized_coords: &'a [NormalizedCoord],
        embolden: Vec2,
        style: impl Into<StyleRef<'a>>,
        brush: impl Into<PaintRef<'a>>,
        brush_alpha: f32,
        transform: Affine,
        glyph_transform: Option<Affine>,
        glyphs: impl Iterator<Item = Glyph> + Clone,
    ) {
        let (style, brush) = (style.into(), brush.into());
        let [a, b, c, d, tx, ty] = transform.as_coeffs();
        let plain = matches!(style, StyleRef::Fill(_))
            && glyph_transform.is_none_or(|g| g == Affine::IDENTITY)
            && (a, b, c, d) == (1.0, 0.0, 0.0, 1.0)
            && font_size <= MAX_SIZE
            && !self.text_to_rest;
        let colour = match brush {
            Paint::Solid(colour) if plain => colour,
            _ => {
                self.rest_draws = true;
                let (s, f): (StyleRef<'a>, _) = (style, brush);
                return self.rest.draw_glyphs(font, font_size, hint, normalized_coords, embolden, s, f, brush_alpha, transform, glyph_transform, glyphs);
            }
        };
        let rgba = colour.to_rgba8();
        let grey = if rgba.r == rgba.g && rgba.g == rgba.b {
            rgba.r as u32
        } else {
            (rgba.r as u32 * 299 + rgba.g as u32 * 587 + rgba.b as u32 * 114) / 1000
        };
        let alpha = (rgba.a as f32 * brush_alpha.clamp(0.0, 1.0)).round() as u32;
        let mut key = Key {
            font: self.cache.font_key(font),
            index: font.index,
            glyph: 0,
            size: font_size.to_bits(),
            hint,
            coords: normalized_coords.to_vec(),
            embolden: (embolden.x.to_bits(), embolden.y.to_bits()),
            subpixel: 0,
        };
        for g in glyphs {
            let (x, y) = (g.x as f64 + tx, g.y as f64 + ty);
            // the nearest quarter pixel across, the nearest pixel down
            let q = (x * SUBPIXEL as f64).round() as i64;
            let (ix, sub) = (q.div_euclid(SUBPIXEL as i64) as i32, q.rem_euclid(SUBPIXEL as i64) as u8);
            key.glyph = g.id;
            key.subpixel = sub;
            if let Some(mask) = self.cache.mask(&key, font, font_size, hint, normalized_coords, embolden) {
                self.placed.push(Placed { mask, x: ix, y: y.round() as i32, grey, alpha });
            }
        }
    }

    fn draw_box_shadow(&mut self, transform: Affine, rect: Rect, brush: Color, radius: f64, std_dev: f64) {
        self.rest_draws = true;
        self.rest.draw_box_shadow(transform, rect, brush, radius, std_dev);
    }
}
