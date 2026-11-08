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

// The Kobo's sound (experiment 13): Kobo's own player, one gst-launch-1.0 into mtkbtmwrpcaudiosink (Bluetooth headphones
// only, no speaker), kept open for the session because only one pipeline can hold the sink and starting one takes ~2 s.
// Raw sound goes in through its standard input, a pipe cut to one page so it holds ~20 ms: a write returns as the sink
// plays, which makes what has been written the narration's clock. While nothing is to be heard, silence is written, so
// the Bluetooth link stays awake.

const (
	sinkRate           = 48000 // the sink takes S16LE, 48 kHz, stereo
	sinkBytesPerSecond = sinkRate * 4
	fSetPipeSz         = 1031 // fcntl F_SETPIPE_SZ (Linux)
)

// silence is the chunk written when there is nothing to play: 20 ms.
var silence = make([]byte, sinkBytesPerSecond/50)

type sink struct {
	cmd  *exec.Cmd
	w    *os.File
	gain atomic.Int32 // the narration's gain, in 1/1024 (1024: as recorded; setVolume)
	buf  []byte       // the narration's chunk at its gain
	src  atomic.Pointer[narrator]
	dead atomic.Bool
	quit chan struct{}
	once sync.Once
}

// openSink starts the player; it fails if gst-launch-1.0 cannot be run. The player is up ~2 s later: what is written
// meanwhile waits in the pipe, so the narration's clock starts when it plays.
func openSink() (*sink, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	syscall.Syscall(syscall.SYS_FCNTL, r.Fd(), fSetPipeSz, 4096)
	if device.Sim() { // no player: the sound read at the sink's pace, 20 ms at a time, so the clock runs as on the Kobo
		k := &sink{w: w, quit: make(chan struct{})}
		k.gain.Store(1024)
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
	cmd := exec.Command("gst-launch-1.0", "-q", "fdsrc", "fd=0", "!", "rawaudioparse", "format=pcm", "pcm-format=s16le",
		fmt.Sprintf("sample-rate=%d", sinkRate), "num-channels=2", "!", "audioconvert", "!", "audioresample", "!",
		fmt.Sprintf("audio/x-raw,format=S16LE,rate=%d,channels=2", sinkRate), "!", "mtkbtmwrpcaudiosink")
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

// run writes the narration's chunks, else silence, as the sink takes them.
func (k *sink) run() {
	for !k.dead.Load() {
		select {
		case <-k.quit:
			return
		default:
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
