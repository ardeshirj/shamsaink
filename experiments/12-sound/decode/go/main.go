// mp3-go: decodes an MP3 to 16-bit PCM in memory with hajimehoshi/go-mp3 and says how fast, against its length
// (experiment 12, step 6).
//
//	mp3-go FILE.mp3 [RUNS]
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/hajimehoshi/go-mp3"
)

func main() {
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	runs := 3
	if len(os.Args) > 2 {
		runs, _ = strconv.Atoi(os.Args[2])
	}
	buf := make([]byte, 64<<10)
	for i := 0; i < runs; i++ {
		t := time.Now()
		d, err := mp3.NewDecoder(bytes.NewReader(data))
		if err != nil {
			panic(err)
		}
		n := 0
		for {
			k, err := d.Read(buf)
			n += k
			if err == io.EOF {
				break
			}
			if err != nil {
				panic(err)
			}
		}
		secs := float64(n) / 4 / float64(d.SampleRate()) // go-mp3 gives 16-bit stereo
		el := time.Since(t).Seconds()
		fmt.Printf("go-mp3: %.1f s of sound decoded in %.2f s: %.0f× real time\n", secs, el, secs/el)
	}
}
