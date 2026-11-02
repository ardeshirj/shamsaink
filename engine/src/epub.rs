//! An EPUB file read as it is, as a reader opens one (stage 0 of docs/plan.md): the zip's central directory read
//! once at open, then each file read and inflated when Blitz asks for it. One open file instead of one open per file
//! of the book, which matters on the Kindle's FUSE storage (09). Stored and deflated entries, the two methods EPUBs
//! use; no zip64 (an EPUB over 4 GB).

use crate::Source;
use std::collections::HashMap;
use std::fs::File;
use std::io::{Error, ErrorKind, Result};
use std::os::unix::fs::FileExt;

pub struct Epub {
    file: File,
    /// A file's path in the book: its local header's offset, its compressed size, its method.
    entries: HashMap<String, (u64, usize, u16)>,
}

fn u16_at(b: &[u8], i: usize) -> usize {
    u16::from_le_bytes([b[i], b[i + 1]]) as usize
}

fn u32_at(b: &[u8], i: usize) -> usize {
    u32::from_le_bytes([b[i], b[i + 1], b[i + 2], b[i + 3]]) as usize
}

impl Epub {
    pub fn open(path: &std::path::Path) -> Result<Epub> {
        let bad = |what: &str| Error::new(ErrorKind::InvalidData, format!("{}: {what}", path.display()));
        let file = File::open(path)?;
        let len = file.metadata()?.len();
        // The end of central directory record: 22 bytes, then a comment of up to 64 KB.
        let tail_len = len.min(22 + 65535) as usize;
        let mut tail = vec![0; tail_len];
        file.read_exact_at(&mut tail, len - tail_len as u64)?;
        let end = (0..=tail_len.saturating_sub(22))
            .rev()
            .find(|&i| tail[i..i + 4] == *b"PK\x05\x06")
            .ok_or_else(|| bad("not a zip"))?;
        let (count, size, offset) = (u16_at(&tail, end + 10), u32_at(&tail, end + 12), u32_at(&tail, end + 16));
        let mut dir = vec![0; size];
        file.read_exact_at(&mut dir, offset as u64)?;
        let mut entries = HashMap::with_capacity(count);
        let mut p = 0;
        for _ in 0..count {
            if p + 46 > dir.len() || dir[p..p + 4] != *b"PK\x01\x02" {
                return Err(bad("broken central directory"));
            }
            let (method, csize) = (u16_at(&dir, p + 10) as u16, u32_at(&dir, p + 20));
            let (name_len, extra_len, comment_len) = (u16_at(&dir, p + 28), u16_at(&dir, p + 30), u16_at(&dir, p + 32));
            let name = String::from_utf8_lossy(&dir[p + 46..p + 46 + name_len]).into_owned();
            entries.insert(name, (u32_at(&dir, p + 42) as u64, csize, method));
            p += 46 + name_len + extra_len + comment_len;
        }
        Ok(Epub { file, entries })
    }
}

impl Source for Epub {
    fn read(&self, path: &str) -> Option<Vec<u8>> {
        let &(local, csize, method) = self.entries.get(path)?;
        // The local header's name and extra field may differ in length from the central directory's.
        let mut head = [0u8; 30];
        self.file.read_exact_at(&mut head, local).ok()?;
        let start = local + 30 + (u16_at(&head, 26) + u16_at(&head, 28)) as u64;
        let mut data = vec![0; csize];
        self.file.read_exact_at(&mut data, start).ok()?;
        match method {
            0 => Some(data),
            8 => miniz_oxide::inflate::decompress_to_vec(&data).ok(),
            _ => None,
        }
    }
}
