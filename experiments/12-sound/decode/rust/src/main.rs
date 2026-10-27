//! mp3-rust: decodes an MP3 to samples in memory with symphonia and says how fast, against its length (experiment 12,
//! step 6).
//!
//!     mp3-rust FILE.mp3 [RUNS]
use std::io::Cursor;
use std::time::Instant;
use symphonia::core::audio::SampleBuffer;
use symphonia::core::codecs::DecoderOptions;
use symphonia::core::formats::FormatOptions;
use symphonia::core::io::MediaSourceStream;
use symphonia::core::meta::MetadataOptions;
use symphonia::core::probe::Hint;

fn main() {
    let args: Vec<String> = std::env::args().collect();
    let data = std::fs::read(&args[1]).unwrap();
    let runs: usize = args.get(2).and_then(|r| r.parse().ok()).unwrap_or(3);
    for _ in 0..runs {
        let t = Instant::now();
        let mss = MediaSourceStream::new(Box::new(Cursor::new(data.clone())), Default::default());
        let mut hint = Hint::new();
        hint.with_extension("mp3");
        let mut format = symphonia::default::get_probe()
            .format(&hint, mss, &FormatOptions::default(), &MetadataOptions::default())
            .unwrap()
            .format;
        let track = format.default_track().unwrap().clone();
        let rate = track.codec_params.sample_rate.unwrap();
        let mut dec = symphonia::default::get_codecs().make(&track.codec_params, &DecoderOptions::default()).unwrap();
        let mut frames = 0usize;
        let mut out: Option<SampleBuffer<i16>> = None;
        while let Ok(packet) = format.next_packet() {
            let Ok(buf) = dec.decode(&packet) else { continue };
            frames += buf.frames();
            let o = out.get_or_insert_with(|| SampleBuffer::new(buf.capacity() as u64, *buf.spec()));
            o.copy_interleaved_ref(buf);
        }
        let secs = frames as f64 / rate as f64;
        let el = t.elapsed().as_secs_f64();
        println!("symphonia: {secs:.1} s of sound decoded in {el:.2} s: {:.0}× real time", secs / el);
    }
}
