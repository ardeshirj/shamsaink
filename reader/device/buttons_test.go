package device

import "testing"

// The node is read from /proc/bus/input/devices's "H: Handlers=event0 perfmgr" line of the "gpio-keys" block.
func TestButtonsNode(t *testing.T) {
	block := "I: Bus=0019 Vendor=0001 Product=0001 Version=0100\nN: Name=\"gpio-keys\"\nP: Phys=gpio-keys/input0\nH: Handlers=event0 perfmgr \nB: EV=100013\n"
	if got := buttonsNodeIn(block + "\nN: Name=\"Elan Touchscreen\"\nH: Handlers=event1 perfmgr \n"); got != "/dev/input/event0" {
		t.Errorf("got %q", got)
	}
	if got := buttonsNodeIn("N: Name=\"Elan Touchscreen\"\nH: Handlers=event1\n"); got != "" {
		t.Errorf("a device without the keys: %q", got)
	}
}
