package device

import "testing"

// wifid's diagnostic text, its shape as the Kindle's firmware 5.19 gives it (the names made up)
func TestKindleNetworks(t *testing.T) {
	text := `********* 2- Connection *********
2.1 MAC: 00:00:00:00:00:00
2.3 AP: Home (00:00:00:00:00:01)
2.3.1   Signal: 5/5

********* 3- Networks *********
3.1   1	Home	0	[WPA2-PSK]	C: [CCMP]
3.2   2	Office	1	[WPA2-PSK]	C: [CCMP]
********* 4- Interface *********
4.1  IP        : 192.0.2.10
`
	if n := kindleNetworks(text); n != 2 {
		t.Errorf("%d networks, want 2", n)
	}
	if n := kindleNetworks("********* 3- Networks *********\n********* 4- Interface *********\n"); n != 0 {
		t.Errorf("%d networks, want 0", n)
	}
}
