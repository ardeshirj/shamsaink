//! Pictures decoded when drawn, at the size drawn (experiment 08), as crengine does. Blitz (with
//! `blitz-lazy-images.patch`) reads only a picture's size from its header for layout and keeps the file as it is; the
//! first page that draws it decodes it here, scaled to the pixels it covers, and keeps that for later pages, up to
//! `LIMIT` bytes. A 3625×5250 scan drawn 936 px wide is 5 MB instead of 76.

use peniko::kurbo::Affine;
use peniko::{Blob, ImageAlphaType, ImageData, ImageFormat};
use std::collections::{HashMap, VecDeque};
use std::sync::Arc;

/// The decoded pictures kept, in bytes.
const LIMIT: usize = 24 << 20;

#[derive(Default)]
pub struct Cache {
    // by the file's blob and the size decoded at; None: it could not be decoded
    decoded: HashMap<(u64, u32, u32), Option<ImageData>>,
    order: VecDeque<(u64, u32, u32)>,
    bytes: usize,
}

/// Whether an image's data is the file as it is (Blitz's own decoded pictures are RGBA, 4 bytes a pixel).
pub fn is_file(image: &ImageData) -> bool {
    image.data.len() != 4 * image.width as usize * image.height as usize
}

impl Cache {
    /// The file `image` decoded at the size it is drawn under `transform` (the brush's to the page's pixels), never
    /// larger than it is, and the transform that puts it where the whole picture was. None if it cannot be decoded.
    pub fn get(&mut self, image: &ImageData, transform: Affine) -> Option<(ImageData, Affine)> {
        let [a, b, c, d, _, _] = transform.as_coeffs();
        let (w, h) = (image.width, image.height);
        let size = |n: u32, s: f64| ((n as f64 * s).ceil() as u32).clamp(1, n);
        let (tw, th) = (size(w, a.hypot(b)), size(h, c.hypot(d)));
        let key = (image.data.id(), tw, th);
        if !self.decoded.contains_key(&key) {
            let pic = decode(image.data.data(), tw, th);
            self.bytes += pic.as_ref().map_or(0, |p| p.data.len());
            self.decoded.insert(key, pic);
            self.order.push_back(key);
            // the oldest let go, keeping the one just decoded
            while self.bytes > LIMIT && self.order.len() > 1 {
                let old = self.order.pop_front().unwrap();
                if let Some(Some(p)) = self.decoded.remove(&old) {
                    self.bytes -= p.data.len();
                }
            }
        }
        let pic = self.decoded[&key].clone()?;
        Some((pic, Affine::scale_non_uniform(w as f64 / tw as f64, h as f64 / th as f64)))
    }
}

/// Decodes a picture at width × height: a JPEG already scaled down while decoded (1/2, 1/4 or 1/8: the smallest at
/// least nine tenths of the size, as the time goes with the decoded size), anything else at full size, then each pixel
/// the average of those it covers (or, up to a tenth larger, the nearest).
fn decode(file: &[u8], width: u32, height: u32) -> Option<ImageData> {
    let (w, h, channels, px) = jpeg(file, width, height).or_else(|| {
        let img = image::load_from_memory(file).ok()?;
        let (w, h) = (img.width(), img.height());
        Some(if img.color().has_alpha() { (w, h, 4, img.into_rgba8().into_raw()) } else { (w, h, 3, img.into_rgb8().into_raw()) })
    })?;
    Some(ImageData {
        data: Blob::new(Arc::new(shrink(&px, w, h, channels, width, height))),
        format: ImageFormat::Rgba8,
        alpha_type: ImageAlphaType::Alpha,
        width,
        height,
    })
}

fn jpeg(file: &[u8], width: u32, height: u32) -> Option<(u32, u32, usize, Vec<u8>)> {
    if !file.starts_with(&[0xFF, 0xD8]) {
        return None;
    }
    let mut d = jpeg_decoder::Decoder::new(file);
    let near = |n: u32| (n * 9).div_ceil(10).min(u16::MAX as u32) as u16;
    let (w, h) = d.scale(near(width), near(height)).ok()?;
    let px = d.decode().ok()?;
    let channels = match d.info()?.pixel_format {
        jpeg_decoder::PixelFormat::L8 => 1,
        jpeg_decoder::PixelFormat::RGB24 => 3,
        _ => return None, // 16-bit grey, CMYK: left to the image crate
    };
    Some((w as u32, h as u32, channels, px))
}

/// RGBA at width × height (no more than a little larger than the source) from 1, 3 or 4 channels: each pixel the average of the source
/// pixels it covers, colours weighted by their opacity (a transparent pixel's colour counts for nothing), one row of
/// sums at a time.
fn shrink(px: &[u8], w: u32, h: u32, channels: usize, width: u32, height: u32) -> Vec<u8> {
    let (w, h, tw, th) = (w as usize, h as usize, width as usize, height as usize);
    let span = |i: usize, from: usize, to: usize| (i * from / to, ((i + 1) * from / to).max(i * from / to + 1));
    let cols: Vec<(usize, usize)> = (0..tw).map(|x| span(x, w, tw)).collect();
    let mut out = Vec::with_capacity(tw * th * 4);
    let mut sums = vec![0u64; tw * 4];
    for ty in 0..th {
        let (y0, y1) = span(ty, h, th);
        sums.fill(0);
        for y in y0..y1 {
            let row = &px[y * w * channels..(y + 1) * w * channels];
            for (tx, &(x0, x1)) in cols.iter().enumerate() {
                let s = &mut sums[4 * tx..4 * tx + 4];
                for p in row[x0 * channels..x1 * channels].chunks_exact(channels) {
                    let (r, g, b, a) = match channels {
                        1 => (p[0], p[0], p[0], 255),
                        3 => (p[0], p[1], p[2], 255),
                        _ => (p[0], p[1], p[2], p[3]),
                    };
                    let a64 = a as u64;
                    s[0] += r as u64 * a64;
                    s[1] += g as u64 * a64;
                    s[2] += b as u64 * a64;
                    s[3] += a64;
                }
            }
        }
        for (tx, &(x0, x1)) in cols.iter().enumerate() {
            let (n, s) = (((x1 - x0) * (y1 - y0)) as u64, &sums[4 * tx..4 * tx + 4]);
            let a = s[3];
            if a == 0 {
                out.extend([0, 0, 0, 0]);
            } else {
                out.extend(s[..3].iter().map(|&v| ((v + a / 2) / a) as u8));
                out.push(((a + n / 2) / n) as u8);
            }
        }
    }
    out
}
