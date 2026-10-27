//! MP3 narration for read-along, decoded in memory with symphonia (pure Rust). The reader (Go) hands over the file's
//! bytes and takes samples as it plays them; a seek goes to a time and says where it landed.

use std::io::Cursor;
use symphonia::core::audio::SampleBuffer;
use symphonia::core::codecs::{Decoder, DecoderOptions};
use symphonia::core::formats::{FormatOptions, FormatReader, SeekMode, SeekTo};
use symphonia::core::io::MediaSourceStream;
use symphonia::core::meta::MetadataOptions;
use symphonia::core::probe::Hint;
use symphonia::core::units::{Time, TimeBase};

/// A decoded-as-played MP3: its reader and decoder, and the samples decoded but not handed out yet.
pub struct Audio {
    format: Box<dyn FormatReader>,
    dec: Box<dyn Decoder>,
    track: u32,
    time_base: Option<TimeBase>,
    rate: u32,
    channels: u32,
    pending: Vec<i16>,
    next: usize,
    scratch: Option<SampleBuffer<i16>>,
}

impl Audio {
    /// Reads the file's format and its first track; None if it is not an MP3 this can decode.
    pub fn open(data: Vec<u8>) -> Option<Audio> {
        let mss = MediaSourceStream::new(Box::new(Cursor::new(data)), Default::default());
        let mut hint = Hint::new();
        hint.with_extension("mp3");
        let format = symphonia::default::get_probe()
            .format(&hint, mss, &FormatOptions::default(), &MetadataOptions::default())
            .ok()?
            .format;
        let track = format.default_track()?.clone();
        let rate = track.codec_params.sample_rate?;
        let channels = track.codec_params.channels.map_or(1, |c| c.count() as u32);
        let dec = symphonia::default::get_codecs().make(&track.codec_params, &DecoderOptions::default()).ok()?;
        Some(Audio {
            format,
            dec,
            track: track.id,
            time_base: track.codec_params.time_base,
            rate,
            channels,
            pending: Vec::new(),
            next: 0,
            scratch: None,
        })
    }

    /// Samples per second, per channel.
    pub fn rate(&self) -> u32 {
        self.rate
    }

    /// Channels; the samples come interleaved.
    pub fn channels(&self) -> u32 {
        self.channels
    }

    /// Seeks to `secs` (accurately, from the packet before it), dropping what was decoded. Returns the time reached in
    /// seconds, or -1 if the seek failed.
    pub fn seek(&mut self, secs: f64) -> f64 {
        let secs = secs.max(0.0);
        let to = SeekTo::Time { time: Time::new(secs.trunc() as u64, secs.fract()), track_id: Some(self.track) };
        let Ok(seeked) = self.format.seek(SeekMode::Accurate, to) else { return -1.0 };
        self.dec.reset();
        self.pending.clear();
        self.next = 0;
        match self.time_base {
            Some(tb) => {
                let t = tb.calc_time(seeked.actual_ts);
                t.seconds as f64 + t.frac
            }
            None => seeked.actual_ts as f64 / self.rate as f64,
        }
    }

    /// Fills `out` with interleaved samples; returns how many were written (0 only at the end of the stream).
    pub fn read(&mut self, out: &mut [i16]) -> usize {
        let mut n = 0;
        while n < out.len() {
            if self.next == self.pending.len() && !self.fill() {
                break;
            }
            let k = (out.len() - n).min(self.pending.len() - self.next);
            out[n..n + k].copy_from_slice(&self.pending[self.next..self.next + k]);
            self.next += k;
            n += k;
        }
        n
    }

    /// Decodes the next packet into `pending`; false at the end of the stream. Packets that fail to decode are skipped.
    fn fill(&mut self) -> bool {
        loop {
            let Ok(packet) = self.format.next_packet() else { return false };
            if packet.track_id() != self.track {
                continue;
            }
            let Ok(buf) = self.dec.decode(&packet) else { continue };
            if self.scratch.as_ref().is_none_or(|s| s.capacity() < buf.capacity()) {
                self.scratch = Some(SampleBuffer::new(buf.capacity() as u64, *buf.spec()));
            }
            let scratch = self.scratch.as_mut().unwrap();
            scratch.copy_interleaved_ref(buf);
            self.pending.clear();
            self.pending.extend_from_slice(scratch.samples());
            self.next = 0;
            return true;
        }
    }
}
