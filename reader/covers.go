package main

import (
	"fmt"
	"time"

	"shamsainkreader/library"
)

// The books' small covers (library/covers.go), made in the background while the library is open: the library shows a
// book's words until its cover is kept, then its page is drawn again (libraryview.go). 2026-11-23.

// coversToMake are the books with a cover picture none is kept for: those in the list shown first, in its order.
func coversToMake(shown []library.Item, all []library.Item, made map[string]bool) []library.Book {
	var bs []library.Book
	seen := map[string]bool{}
	for _, list := range [][]library.Item{shown, all} {
		for _, it := range list {
			if it.Cover != "" && !made[it.Hash] && !seen[it.Hash] {
				seen[it.Hash] = true
				bs = append(bs, it.Book)
			}
		}
	}
	return bs
}

// makeCovers makes books' small covers in turn, for a screen w wide (a tile's width, a book's shape: 2 by 3), each
// one's hash sent on out once kept, until done or stop is closed (a book opened: the processor is the book's).
func makeCovers(dir string, books []library.Book, w int, out chan<- string, stop <-chan struct{}) {
	tw := (w - 2*40 - 3*36) / 4 // ui's tiles: four a row, the page's margins and the gaps between (a wide row's width;
	// a cover kept bigger from before 2026-11-24 is drawn smaller, as it is)
	for _, b := range books {
		select {
		case <-stop:
			return
		default:
		}
		t := time.Now()
		if err := library.MakeCover(dir, b, tw, tw*3/2); err != nil {
			fmt.Printf("covers: %v\n", err)
			continue
		}
		fmt.Printf("covers: %s made in %.0f ms\n", b.Path, ms(t))
		select {
		case out <- b.Hash:
		case <-stop:
			return
		}
	}
}
