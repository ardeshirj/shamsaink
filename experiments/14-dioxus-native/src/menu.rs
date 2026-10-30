//! The menu as Dioxus components, the same markup and classes as reader/ui/templates/components.html and menu.html.
use crate::common::*;
use blitz_dom::{Document as _, DocumentConfig};
use blitz_traits::shell::{ColorScheme, Viewport};
use dioxus::prelude::*;
use dioxus_core::ElementId;
use dioxus_html::PlatformEventData;
use dioxus_native_dom::{DioxusDocument, synthetic_click_event};
use std::any::Any;
use std::rc::Rc;
use std::time::Instant;

const UI_CSS: &str = include_str!("../../../reader/ui/templates/ui.css");

#[component]
fn Button(id: String, label: String, #[props(default)] on: bool, onclick: EventHandler<()>) -> Element {
    // components.html: "button" (and "choice", with a tick, when `on` is given as a tick)
    let tick = if on { "✓ " } else { "" };
    rsx! { span { class: "b", id: "{id}", onclick: move |_| onclick.call(()), "{tick}{label}" } }
}

/// The switch ("toggle" in components.html): a track with a knob.
#[component]
fn Switch(id: String, on: bool, onclick: EventHandler<()>) -> Element {
    let class = if on { "s on" } else { "s" };
    rsx! { span { class: "{class}", id: "{id}", onclick: move |_| onclick.call(()), span { class: "k" } } }
}

/// "stepper": a name, a minus, the value, a plus (grid cells of div.set).
#[component]
fn Stepper(name: String, id: String, value: String, dec: EventHandler<()>, inc: EventHandler<()>) -> Element {
    rsx! {
        span { class: "name", "{name}" }
        Button { id: "{id}-dec", label: "−", onclick: dec }
        span { class: "v", "{value}" }
        Button { id: "{id}-inc", label: "+", onclick: inc }
    }
}

/// "switch" in components.html: a name and a switch for its value.
#[component]
fn SwitchRow(label: String, id: String, on: bool, onclick: EventHandler<()>) -> Element {
    rsx! {
        span { class: "name", "{label}" }
        span { class: "v", Switch { id: "{id}", on, onclick } }
    }
}

/// "segments": one choice of a few as one bar, the chosen one filled.
#[component]
fn Segments(items: Vec<(String, String, bool)>, onpick: EventHandler<String>) -> Element {
    rsx! {
        div { class: "g",
            for (id, label, on) in items {
                span {
                    key: "{id}",
                    class: if on { "on" } else { "" },
                    id: "{id}",
                    onclick: {
                        let id = id.clone();
                        move |_| onpick.call(id.clone())
                    },
                    "{label}"
                }
            }
        }
    }
}

const BOOK_LINES: f32 = 1.16;

fn app() -> Element {
    let mut size = use_signal(|| 50.0f32);
    let mut lines = use_signal(|| None::<f32>); // None: the book's own
    let mut margin = use_signal(|| 60i32);
    let mut clock = use_signal(|| true);
    let mut dir = use_signal(String::new);
    let (lines_name, lines_v) = match lines() {
        Some(v) => ("Line spacing", v),
        None => ("Line spacing (the book's)", BOOK_LINES),
    };
    let mut step_lines = move |by: f32| {
        let v = lines().unwrap_or(BOOK_LINES) + by;
        lines.set(Some((v.clamp(0.8, 3.0) * 20.0).round() / 20.0));
    };
    let seg = |id: &str, label: &str| (id.to_string(), label.to_string(), dir() == id.trim_start_matches("dir-"));
    rsx! {
        style { "{UI_CSS} {PAGE_CSS}" }
        div { class: "row",
            Button { id: "leave", label: "Close the book", onclick: |_| {} }
            Button {
                id: "reset",
                label: "Reset settings",
                onclick: move |_| {
                    size.set(50.0);
                    lines.set(None);
                    margin.set(60);
                    clock.set(true);
                    dir.set(String::new());
                },
            }
            Button { id: "read-aloud", label: "Read aloud", onclick: |_| {} }
        }
        div { class: "set",
            Stepper {
                name: "Text size",
                id: "size",
                value: "{size} px",
                dec: move |_| size.set((size() - 2.0).clamp(30.0, 90.0)),
                inc: move |_| size.set((size() + 2.0).clamp(30.0, 90.0)),
            }
            Stepper {
                name: "{lines_name}",
                id: "lines",
                value: "{lines_v:.2}",
                dec: move |_| step_lines(-0.05),
                inc: move |_| step_lines(0.05),
            }
            Stepper {
                name: "Margins",
                id: "margin",
                value: "{margin} px",
                dec: move |_| margin.set((margin() - 10).clamp(0, 200)),
                inc: move |_| margin.set((margin() + 10).clamp(0, 200)),
            }
            SwitchRow { label: "Clock", id: "clock", on: clock(), onclick: move |_| clock.toggle() }
        }
        p { "Page turns (this book):" }
        Segments {
            items: vec![seg("dir-", "Book's (RTL)"), seg("dir-rtl", "Right to left"), seg("dir-ltr", "Left to right")],
            onpick: move |id: String| dir.set(id.trim_start_matches("dir-").to_string()),
        }
        p { "\u{a0}" }
    }
}

fn new_doc(fonts: &parley::FontContext) -> DioxusDocument {
    let cfg = DocumentConfig {
        viewport: Some(Viewport::new(W, H, 1.0, ColorScheme::Light)),
        font_ctx: Some(fonts.clone()),
        ..Default::default()
    };
    DioxusDocument::new(VirtualDom::new(app), cfg)
}

/// A click on the element with this html id, the way a touch would arrive (no hit test).
fn click(doc: &mut DioxusDocument, id: &str) {
    let (ev, eid) = {
        let b = doc.inner.borrow();
        let nid = b.get_element_by_id(id).expect("no such element");
        let node = b.get_node(nid).unwrap();
        let eid = node.attr(blitz_dom::LocalName::from("data-dioxus-id")).and_then(|v| v.parse::<usize>().ok());
        let ev = synthetic_click_event(node, Default::default());
        (ev, eid.expect("element has no click handler"))
    };
    let ev = Event::new(Rc::new(PlatformEventData::new(ev)) as Rc<dyn Any>, true);
    doc.vdom.runtime().handle_event("click", ev, ElementId(eid));
}

/// A short name for a node: <tag#id.class> and its text.
fn describe(doc: &blitz_dom::BaseDocument, id: blitz_dom::NodeId) -> String {
    let n = doc.get_node(id).unwrap();
    let e = n.element_data().unwrap();
    let text: String = n.text_content().chars().take(24).collect();
    format!(
        "<{}{}{}> \"{}\" box {:?}",
        e.name.local,
        e.attr(blitz_dom::local_name!("id")).map(|i| format!("#{i}")).unwrap_or_default(),
        e.attr(blitz_dom::local_name!("class")).filter(|c| !c.is_empty()).map(|c| format!(".{c}")).unwrap_or_default(),
        text,
        rect_of(doc, id)
    )
}

/// Other taps, each from a fresh menu: what the mutations marked, against what changed on the screen.
fn survey(fonts: &parley::FontContext, p: &mut Painter) {
    println!("other taps (second pass of each, warm): marked boxes vs pixel diff");
    for id in ["size-inc", "lines-inc", "margin-dec", "clock", "dir-rtl", "reset"] {
        let mut last = None;
        for _pass in 0..2 {
            let mut doc = new_doc(fonts);
            doc.initial_build();
            doc.inner.borrow_mut().resolve(0.0);
            let a = p.paint(&mut doc.inner.borrow_mut());
            if id == "reset" {
                click(&mut doc, "size-inc"); // so that there is something to reset
                doc.poll(None);
                doc.inner.borrow_mut().resolve(0.0);
            }
            let a = if id == "reset" { p.paint(&mut doc.inner.borrow_mut()) } else { a };
            let t = Instant::now();
            click(&mut doc, id);
            doc.poll(None);
            let t_event = ms(t);
            let mut touched = vec![];
            doc.inner.borrow().visit(|n, node| {
                if node.damage().is_some_and(|d| !d.is_empty()) {
                    touched.push(n);
                }
            });
            let t = Instant::now();
            doc.inner.borrow_mut().resolve(0.0);
            let t_res = ms(t);
            let t = Instant::now();
            let b = p.paint(&mut doc.inner.borrow_mut());
            let t_paint = ms(t);
            let d = doc.inner.borrow();
            let mut un = [i32::MAX, i32::MAX, i32::MIN, i32::MIN];
            let mut names = vec![];
            for &n in &touched {
                let r = rect_of(&d, n);
                un = [un[0].min(r[0]), un[1].min(r[1]), un[2].max(r[2]), un[3].max(r[3])];
                names.push(describe(&d, n).split(" box").next().unwrap().to_string());
            }
            last = Some((id, t_event, t_res, t_paint, touched.len(), names, un, diff_box(&a, &b)));
        }
        let (id, e, r, pa, k, names, un, df) = last.unwrap();
        println!("  {id}: event+mutations {e:.2} / resolve {r:.2} / paint {pa:.2} ms; {k} nodes marked {names:?}");
        println!("     union of marked boxes {un:?}; pixel diff {:?}", df.map(|(b, c)| (b, c)));
    }
}

pub fn run(n: usize, fonts: &str, out: &str) {
    let mut p = Painter::new();
    let fonts = font_context(fonts); // once, as the reader's fonts are
    let mut create = vec![];
    let (mut build, mut res, mut paint) = (vec![], vec![], vec![]);
    let (mut event, mut res2, mut paint2) = (vec![], vec![], vec![]);
    let mut imgs = vec![];
    let mut touched: Vec<blitz_dom::NodeId> = vec![];
    for i in 0..n {
        let t = Instant::now();
        let mut doc = new_doc(&fonts);
        create.push(ms(t));
        let t = Instant::now();
        doc.initial_build();
        build.push(ms(t));
        let t = Instant::now();
        doc.inner.borrow_mut().resolve(0.0);
        res.push(ms(t));
        let t = Instant::now();
        let a = p.paint(&mut doc.inner.borrow_mut());
        paint.push(ms(t));
        let ba = bottom(&doc.inner.borrow());

        // the tap
        let t = Instant::now();
        click(&mut doc, "size-inc");
        let changed = doc.poll(None);
        event.push(ms(t));
        assert!(changed, "the vdom made no mutations");
        if i == 0 {
            // which nodes the mutation writer marked, before resolve clears the marks
            let b = doc.inner.borrow();
            b.visit(|id, node| {
                if node.damage().is_some_and(|d| !d.is_empty()) {
                    touched.push(id);
                }
            });
        }
        let t = Instant::now();
        doc.inner.borrow_mut().resolve(0.0);
        res2.push(ms(t));
        let t = Instant::now();
        let b = p.paint(&mut doc.inner.borrow_mut());
        paint2.push(ms(t));
        let bb = bottom(&doc.inner.borrow());
        if i == 0 {
            println!("nodes the mutations marked damaged (before resolve), with their boxes after it:");
            let d = doc.inner.borrow();
            let mut un = [i32::MAX, i32::MAX, i32::MIN, i32::MIN];
            for &id in &touched {
                println!("  node {id}: {}", describe(&d, id));
                let r = rect_of(&d, id);
                un = [un[0].min(r[0]), un[1].min(r[1]), un[2].max(r[2]), un[3].max(r[3])];
            }
            println!("union of those boxes: {un:?} ({} x {})", un[2] - un[0], un[3] - un[1]);
            imgs = vec![(a, ba), (b, bb)];
        }
    }
    println!("first draw (new vdom + document)");
    let line = |l: &str, v: &mut Vec<f64>| println!("{l:<34} median {:7.2} ms   first {:7.2} ms", median(&mut v.clone()), v[0]);
    line("  VirtualDom + document created", &mut create);
    line("  initial build (vdom -> DOM)", &mut build);
    line("  resolve (style+layout)", &mut res);
    line("  paint", &mut paint);
    println!("after the tap on size-inc");
    line("  event + vdom + mutations applied", &mut event);
    line("  resolve (restyle+relayout)", &mut res2);
    line("  paint", &mut paint2);
    save(&format!("{out}/dioxus-before.png"), &imgs[0].0, imgs[0].1);
    save(&format!("{out}/dioxus-after.png"), &imgs[1].0, imgs[1].1);
    match diff_box(&imgs[0].0, &imgs[1].0) {
        Some((b, k)) => println!("pixel diff before/after: box {b:?} ({} x {}), {k} pixels", b[2] - b[0], b[3] - b[1]),
        None => println!("pixel diff: none"),
    }
    survey(&fonts, &mut p);
    println!("max RSS {:.1} MB", max_rss_mb());
}
