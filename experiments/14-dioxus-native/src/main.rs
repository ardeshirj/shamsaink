//! Experiment 14: the reader's menu drawn two ways by Blitz (plain vello_cpu), timed.
//!     spike templates|dioxus N FONTS HTMLDIR OUTDIR
mod common;
#[cfg(feature = "dioxus")]
mod menu;

use blitz_dom::DocumentConfig;
use blitz_html::HtmlDocument;
use blitz_traits::shell::{ColorScheme, Viewport};
use common::*;
use std::time::Instant;

fn config(fonts: &parley::FontContext) -> DocumentConfig {
    DocumentConfig {
        viewport: Some(Viewport::new(W, H, 1.0, ColorScheme::Light)),
        font_ctx: Some(fonts.clone()),
        ..Default::default()
    }
}

fn pct(label: &str, v: &mut Vec<f64>) {
    let first = v[0];
    println!("{label:<34} median {:7.2} ms   first {:7.2} ms", median(v), first);
}

/// What the Go reader does: a whole new document from the template's HTML, on the first draw and on every change.
fn templates(n: usize, fonts: &str, dir: &str, out: &str) {
    let html = |name: &str| {
        let h = std::fs::read_to_string(format!("{dir}/{name}.html")).unwrap();
        h.replacen("</head>", &format!("<style>{PAGE_CSS}</style></head>"), 1)
    };
    let (h50, h52) = (html("menu-50"), html("menu-52"));
    let mut p = Painter::new();
    let fonts = font_context(fonts); // once, as the reader's fonts are
    let (mut a_parse, mut a_res, mut a_paint) = (vec![], vec![], vec![]);
    let (mut b_parse, mut b_res, mut b_paint) = (vec![], vec![], vec![]);
    let mut imgs = vec![];
    for i in 0..n {
        let mut one = |h: &str, parse: &mut Vec<f64>, res: &mut Vec<f64>, paint: &mut Vec<f64>| {
            let cfg = config(&fonts);
            let t = Instant::now();
            let mut d = HtmlDocument::from_html(h, cfg);
            parse.push(ms(t));
            let t = Instant::now();
            d.as_mut().resolve(0.0);
            res.push(ms(t));
            let t = Instant::now();
            let g = p.paint(d.as_mut());
            paint.push(ms(t));
            (g, bottom(d.as_ref()))
        };
        let a = one(&h50, &mut a_parse, &mut a_res, &mut a_paint);
        let b = one(&h52, &mut b_parse, &mut b_res, &mut b_paint);
        if i == 0 {
            imgs = vec![a, b];
        }
    }
    println!("first draw (new document)");
    pct("  parse HTML", &mut a_parse);
    pct("  resolve (style+layout)", &mut a_res);
    pct("  paint", &mut a_paint);
    println!("after the tap (a whole new document, Go's template time not counted)");
    pct("  parse HTML", &mut b_parse);
    pct("  resolve (style+layout)", &mut b_res);
    pct("  paint", &mut b_paint);
    save(&format!("{out}/templates-before.png"), &imgs[0].0, imgs[0].1);
    save(&format!("{out}/templates-after.png"), &imgs[1].0, imgs[1].1);
    match diff_box(&imgs[0].0, &imgs[1].0) {
        Some((b, k)) => println!("pixel diff before/after: box {b:?} ({} x {}), {k} pixels", b[2] - b[0], b[3] - b[1]),
        None => println!("pixel diff: none"),
    }
    println!("max RSS {:.1} MB", max_rss_mb());
}

fn main() {
    let a: Vec<String> = std::env::args().collect();
    if a.len() < 6 {
        eprintln!("usage: spike templates|dioxus N FONTS HTMLDIR OUTDIR");
        std::process::exit(2);
    }
    let n: usize = a[2].parse().unwrap();
    match a[1].as_str() {
        "templates" => templates(n, &a[3], &a[4], &a[5]),
        #[cfg(feature = "dioxus")]
        "dioxus" => menu::run(n, &a[3], &a[5]),
        v => eprintln!("unknown variant {v} (dioxus needs --features dioxus)"),
    }
}
