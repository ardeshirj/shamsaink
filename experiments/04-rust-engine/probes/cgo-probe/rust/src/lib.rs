//! Stage 0 probe for experiment 04: the smallest real Rust engine call from Go. Lays out UTF-8 text with Parley in a
//! font given as bytes, at a width, and returns the number of lines (or -1).

use parley::{Alignment, AlignmentOptions, BaseDirection, FontContext, LayoutContext, LineHeight, StyleProperty};

/// # Safety
/// `font` and `text` must point to `font_len` and `text_len` readable bytes; `text` must be UTF-8.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn probe_line_count(
    font: *const u8,
    font_len: usize,
    text: *const u8,
    text_len: usize,
    width: f32,
    size: f32,
) -> i32 {
    let font = unsafe { std::slice::from_raw_parts(font, font_len) }.to_vec();
    let Ok(text) = std::str::from_utf8(unsafe { std::slice::from_raw_parts(text, text_len) }) else {
        return -1;
    };
    let mut fcx = FontContext::new();
    let fams = fcx.collection.register_fonts(font.into(), None);
    let Some((family, _)) = fams.first() else { return -1 };
    let name = fcx.collection.family_name(*family).unwrap_or("").to_string();
    let mut lcx: LayoutContext<()> = LayoutContext::new();
    let mut b = lcx.ranged_builder(&mut fcx, text, 1.0, false);
    b.set_base_direction(BaseDirection::Rtl);
    b.push_default(StyleProperty::FontFamily(name.as_str().into()));
    b.push_default(StyleProperty::FontSize(size));
    b.push_default(StyleProperty::LineHeight(LineHeight::FontSizeRelative(1.9)));
    let mut layout = b.build(text);
    layout.break_all_lines(Some(width));
    layout.align(Alignment::Justify, AlignmentOptions::default());
    layout.len() as i32
}
