// fetch downloads URL into FILE as the reader would: streamed to storage, a 20 s limit on connecting and on the
// answer's start, then reports the status, size, time and the process's peak memory.
//
//	fetch URL FILE
package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"syscall"
	"time"

	_ "golang.org/x/crypto/x509roots/fallback" // Go's own CA roots when the device has none, as the updater
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: fetch URL FILE")
		os.Exit(2)
	}
	client := &http.Client{Transport: &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 20 * time.Second}).DialContext,
		TLSHandshakeTimeout:   20 * time.Second,
		ResponseHeaderTimeout: 20 * time.Second,
	}}
	t0 := time.Now()
	res, err := client.Get(os.Args[1])
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
	head := time.Since(t0)
	out, err := os.Create(os.Args[2])
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
	n, err := io.Copy(out, res.Body)
	res.Body.Close()
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	all := time.Since(t0)
	var ru syscall.Rusage
	syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	fmt.Printf("%s %d bytes (Content-Length %d), headers %d ms, all %d ms, %.2f MB/s, peak RSS %.1f MB, CPU %d ms",
		res.Status, n, res.ContentLength, head.Milliseconds(), all.Milliseconds(),
		float64(n)/1e6/all.Seconds(), float64(ru.Maxrss)/1024,
		(time.Duration(ru.Utime.Nano())+time.Duration(ru.Stime.Nano())).Milliseconds())
	if err != nil {
		fmt.Printf(", error: %v", err)
	}
	fmt.Println()
}
