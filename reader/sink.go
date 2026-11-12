package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"shamsainkreader/device"
)

// The Kobo's sound (experiment 13): Kobo's own player, one gst-launch-1.0 into the device's sink (SoundSink: PulseAudio's
// on firmware 6, MediaTek's mtkbtmwrpcaudiosink on 4.x; Bluetooth headphones only, no speaker), kept open for the session because only one pipeline can hold the sink and starting one takes ~2 s.
// Raw sound goes in through its standard input, a pipe cut to one page so it holds ~20 ms: a write returns as the sink
// plays, which makes what has been written the narration's clock. While nothing is to be heard, silence is written, so
// the Bluetooth link stays awake.

const (
	sinkRate           = 48000 // the sink takes S16LE, 48 kHz, stereo
	sinkBytesPerSecond = sinkRate * 4
	fSetPipeSz         = 1031 // fcntl F_SETPIPE_SZ (Linux)
	// sinkBuffer is the player's buffer (its buffer-time). GStreamer's default for audio sinks, 200 ms, which Kobo's
	// Bluetooth sink and its own interface keep, is less than the player's stalls on the Libra Colour: 250-320 ms, every
	// 4 and 10 s in turn, the writer ready all along (2026-11-08); a stall longer than the buffer was heard as a glitch.
	sinkBuffer = 500 * time.Millisecond
	// sinkLater is how much later than with the default buffer the sound written is heard: the underline is moved by it
	// (the lead set by ear, lead_ms, was set with the default).
	sinkLater = sinkBuffer - 200*time.Millisecond
)

// silence is the chunk written when there is nothing to play: 20 ms.
var silence = make([]byte, sinkBytesPerSecond/50)

type sink struct {
	cmd     *exec.Cmd
	w       *os.File
	gain    atomic.Int32 // the narration's gain, in 1/1024 (1024: as recorded; setVolume)
	settled atomic.Bool  // the player is taking sound at its pace (run): the narration may start
	buf     []byte       // the narration's chunk at its gain
	src     atomic.Pointer[narrator]
	dead    atomic.Bool
	quit    chan struct{}
	once    sync.Once
}

// openSink starts the player; it fails if gst-launch-1.0 cannot be run. The player is up ~2 s later: what is written
// meanwhile waits in the pipe, so the narration's clock starts when it plays.
func openSink(dev *device.Device) (*sink, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	syscall.Syscall(syscall.SYS_FCNTL, r.Fd(), fSetPipeSz, 4096)
	if device.Sim() { // no player: the sound read at the sink's pace, 20 ms at a time, so the clock runs as on the Kobo
		k := &sink{w: w, quit: make(chan struct{})}
		k.gain.Store(1024)
		k.settled.Store(true)                                       // no player to wait for: the scenarios' timing as before
		name := fmt.Sprintf("sound-%d.raw", time.Now().UnixMilli()) // a video of the simulator: what the Kobo would play
		rec := device.Record(name)
		if rec != nil {
			device.RecordNote("sound s16le %d 2 %s", sinkRate, name)
		}
		go func() {
			defer r.Close()
			if rec != nil {
				defer rec.Close()
			}
			t := time.NewTicker(20 * time.Millisecond)
			defer t.Stop()
			buf := make([]byte, len(silence))
			for {
				select {
				case <-k.quit:
					return
				case <-t.C:
				}
				if _, err := io.ReadFull(r, buf); err != nil {
					return
				}
				if rec != nil {
					rec.Write(buf)
				}
			}
		}()
		go k.run()
		return k, nil
	}
	element, env := dev.SoundSink()
	cmd := exec.Command("gst-launch-1.0", "-q", "fdsrc", "fd=0", "!", "rawaudioparse", "format=pcm", "pcm-format=s16le",
		fmt.Sprintf("sample-rate=%d", sinkRate), "num-channels=2", "!", "audioconvert", "!", "audioresample", "!",
		fmt.Sprintf("audio/x-raw,format=S16LE,rate=%d,channels=2", sinkRate), "!", element, fmt.Sprintf("buffer-time=%d", sinkBuffer.Microseconds()))
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = r
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Start(); err != nil {
		r.Close()
		w.Close()
		return nil, err
	}
	r.Close()
	k := &sink{cmd: cmd, w: w, quit: make(chan struct{})}
	k.gain.Store(1024)
	go func() { // the player ended (headphones gone, an error): the writes stop
		err := cmd.Wait()
		k.dead.Store(true)
		select {
		case <-k.quit: // ended by us
		default:
			fmt.Fprintln(os.Stderr, "shamsaink: sound: the player ended:", err)
		}
		w.Close()
	}()
	go k.run()
	return k, nil
}

// A player just opened takes ~2 s to wake the headphones (experiment 13): it takes its buffer's worth at once, then
// stalls (1.4 s, then 0.6 s, seen 2026-11-08), so a voice written meanwhile was cut near its start. It is given silence
// until it has settled: open settleAfter at least and settleRun of chunks taken each within settleChunk, or settleMax
// whatever it does. The narration's clock starts with its first chunk taken after that.
const (
	settleAfter = time.Second
	settleRun   = 500 * time.Millisecond
	settleChunk = 100 * time.Millisecond
	settleMax   = 5 * time.Second
)

// run writes the narration's chunks, else silence, as the sink takes them; silence alone until the player has settled.
func (k *sink) run() {
	opened, run := time.Now(), time.Duration(0)
	for !k.dead.Load() {
		select {
		case <-k.quit:
			return
		default:
		}
		if !k.settled.Load() {
			t := time.Now()
			if _, err := k.w.Write(silence); err != nil {
				return
			}
			if d := time.Since(t); d > settleChunk {
				run = 0
			} else {
				run += 20 * time.Millisecond
			}
			if since := time.Since(opened); since >= settleAfter && run >= settleRun || since >= settleMax {
				fmt.Printf("sound: the player settled after %d ms\n", since.Milliseconds())
				k.settled.Store(true)
			}
			continue
		}
		if n := k.src.Load(); n != nil {
			if !n.feed(k) {
				k.src.CompareAndSwap(n, nil)
			}
			continue
		}
		if _, err := k.w.Write(silence); err != nil {
			return
		}
	}
}

// setVolume sets the narration's volume, per cent of the sound as recorded: a gain on our sound, squared so the
// slider's steps sound even (half way, about a quarter of the amplitude: 12 dB quieter). The headphones' own volume
// cannot be set from the Kobo (its Bluetooth service's Volume is read only, docs/devices/kobo/README.md).
func (k *sink) setVolume(percent int) {
	v := min(max(percent, 0), 100)
	k.gain.Store(int32(v * v * 1024 / 10000))
}

// Write writes the narration's chunk p to the player at its volume (each 16-bit sample scaled), as it is at 100.
func (k *sink) Write(p []byte) (int, error) {
	g := k.gain.Load()
	if g >= 1024 {
		return k.w.Write(p)
	}
	k.buf = scale(k.buf[:0], p, g)
	if _, err := k.w.Write(k.buf); err != nil {
		return 0, err
	}
	return len(p), nil
}

// scale appends p's 16-bit little-endian samples to out, each times gain/1024.
func scale(out, p []byte, gain int32) []byte {
	for i := 0; i+1 < len(p); i += 2 {
		v := int32(int16(uint16(p[i])|uint16(p[i+1])<<8)) * gain / 1024
		out = append(out, byte(v), byte(v>>8))
	}
	return out
}

// play makes n the sound heard; the one before it is dropped.
func (k *sink) play(n *narrator) {
	if old := k.src.Swap(n); old != nil {
		old.stop()
	}
}

// close ends the player.
func (k *sink) close() {
	k.once.Do(func() {
		close(k.quit)
		k.w.Close()
		if k.cmd != nil { // nil on the simulated device
			time.AfterFunc(time.Second, func() { k.cmd.Process.Kill() })
		}
	})
}
