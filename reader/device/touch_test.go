package device

import (
	"encoding/binary"
	"os"
	"testing"
	"time"
)

// The touch screen as the Kobo Libra Colour reports it (captured 2026-11-20): the finger's tracking id again in every
// report, about 70 a second, and the lift's report with a position of its own (back where the finger came down).

// A finger's reports, as the kernel writes them: each point a report (the id, x, y, then a sync) 14 ms apart, then the
// lift (lift: its x and y, which may differ from the last point's, then id -1).
type kobo struct {
	w  *os.File
	at time.Duration
}

func (k *kobo) ev(typ, code uint16, val int32) {
	var b [16]byte
	binary.LittleEndian.PutUint32(b[0:], uint32(k.at/time.Second))
	binary.LittleEndian.PutUint32(b[4:], uint32(k.at%time.Second/time.Microsecond))
	binary.LittleEndian.PutUint16(b[8:], typ)
	binary.LittleEndian.PutUint16(b[10:], code)
	binary.LittleEndian.PutUint32(b[12:], uint32(val))
	k.w.Write(b[:])
}

func (k *kobo) report(id int32, x, y int) {
	k.ev(evAbs, absMTTrackingID, id)
	k.ev(evAbs, absMTPositionX, int32(x))
	k.ev(evAbs, absMTPositionY, int32(y))
	k.ev(evSyn, synReport, 0)
	k.at += 14 * time.Millisecond
}

func (k *kobo) lift(x, y int) {
	k.ev(evAbs, absMTPositionX, int32(x))
	k.ev(evAbs, absMTPositionY, int32(y))
	k.ev(evAbs, absMTTrackingID, -1)
	k.ev(evSyn, synReport, 0)
	k.at += 14 * time.Millisecond
}

func koboTouch(t *testing.T) (*kobo, <-chan Event) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { w.Close(); r.Close() })
	out := make(chan Event, 8)
	go readTouches(r, func(x, y int) (int, int) { return x, y }, out, 0, 0)
	return &kobo{w: w, at: 1000 * time.Second}, out
}

func next(t *testing.T, out <-chan Event) Event {
	select {
	case e := <-out:
		return e
	case <-time.After(3 * time.Second):
		t.Fatal("no gesture")
	}
	return Event{}
}

// A tap: the same id in each report is one finger, and its place is where it came down.
func TestTouchRepeatedIDTap(t *testing.T) {
	k, out := koboTouch(t)
	for i := 0; i < 10; i++ {
		k.report(650, 1020-i/3, 1041)
	}
	k.lift(1020, 1041)
	e := next(t, out)
	if e.Swipe || e.Hold != NotHeld || e.X != 1020 || e.Y != 1041 {
		t.Errorf("tap: %+v, want a tap at 1020,1041", e.Gesture)
	}
}

// A swipe: measured from where the finger came down to where it last was, not from its last report (which made a
// swipe look like a tap), nor to the lift's own position (which jumps back to the start).
func TestTouchRepeatedIDSwipe(t *testing.T) {
	k, out := koboTouch(t)
	for i := 0; i <= 30; i++ {
		k.report(652, 1152-23*i, 1013)
	}
	k.lift(1152, 1013) // the lift's report back at the start, as the Kobo's first touch did
	e := next(t, out)
	if !e.Swipe || e.DX != -690 || e.X != 1152 {
		t.Errorf("swipe: %+v, want a swipe from 1152 by -690", e.Gesture)
	}
}

// A finger that moves early and stays down past holdTime is a slow swipe, not a hold: each report used to start a hold's
// timer of its own, and the first, never stopped, made it a hold (a word chosen by accident).
func TestTouchRepeatedIDSlowSwipeNoHold(t *testing.T) {
	k, out := koboTouch(t)
	for i := 0; i < 10; i++ {
		k.report(653, 597+40*i, 924) // moved well past holdStill in the first reports
	}
	time.Sleep(holdTime + 300*time.Millisecond)
	k.report(653, 1000, 924)
	k.lift(1000, 924)
	e := next(t, out)
	if e.Hold != NotHeld || !e.Swipe {
		t.Errorf("slow swipe: %+v, want a swipe, no hold", e.Gesture)
	}
}

// A finger held still past holdTime is a hold, and its drop where it last was.
func TestTouchRepeatedIDHold(t *testing.T) {
	k, out := koboTouch(t)
	for i := 0; i < 10; i++ {
		k.report(651, 900, 1117)
	}
	if e := next(t, out); e.Hold != Held || e.X != 900 || e.Y != 1117 {
		t.Fatalf("hold: %+v, want held at 900,1117", e.Gesture)
	}
	k.report(651, 900, 1117) // a report while held: a drag, where the finger is
	k.lift(1020, 1041)
	e := next(t, out)
	for e.Hold == Dragged {
		e = next(t, out)
	}
	if e.Hold != Dropped || e.X != 900 || e.Y != 1117 {
		t.Errorf("drop: %+v, want dropped at 900,1117", e.Gesture)
	}
}
