package main

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
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
			if !n.feed(k.w) {
				k.src.CompareAndSwap(n, nil)
			}
			continue
		}
		if _, err := k.w.Write(silence); err != nil {
			return
		}
	}
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
		time.AfterFunc(time.Second, func() { k.cmd.Process.Kill() })
	})
}
