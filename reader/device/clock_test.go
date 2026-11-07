package device

import (
	"testing"
	"time"
)

// The wait for a new minute ends just past the next minute's start, whatever the second now.
func TestUntilMinute(t *testing.T) {
	for _, c := range []struct {
		now  string
		want time.Duration
	}{
		{"10:00:00", time.Minute + 5*time.Millisecond},
		{"10:00:59.5", 505 * time.Millisecond},
		{"10:59:30", 30*time.Second + 5*time.Millisecond},
	} {
		now, err := time.ParseInLocation("15:04:05.999", c.now, time.FixedZone("IRST", 3*3600+1800))
		if err != nil {
			t.Fatal(err)
		}
		if got := untilMinute(now); got != c.want {
			t.Errorf("untilMinute(%s) = %v, want %v", c.now, got, c.want)
		}
	}
}
