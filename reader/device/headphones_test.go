package device

import (
	"strings"
	"testing"
)

// BlueZ answers GetManagedObjects, printed by dbus-send, in this shape (measured on the Libra Colour, 4.x's service
// 2026-11-08 and firmware 6's BlueZ 2026-11-11; addresses made up, most properties left out): a paired device, connected
// or not, and while sound flows its transport.
const (
	btAdapter = `method return time=1762636000.0 sender=:1.12 -> destination=:1.40 serial=50 reply_serial=2
   array [
      dict entry(
         object path "/org/bluez/hci0"
         array [
            dict entry(
               string "org.bluez.Device1"
`
	btPaired = `      dict entry(
         object path "/org/bluez/hci0/dev_00_11_22_33_44_55"
         array [
            dict entry(
               string "org.bluez.Device1"
               array [
                  dict entry(
                     string "Adapter"
                     variant                         object path "/org/bluez/hci0"
                  )
                  dict entry(
                     string "Icon"
                     variant                         string "audio-headphones"
                  )
                  dict entry(
                     string "Paired"
                     variant                         boolean true
                  )
`
	btConnected = `                  dict entry(
                     string "Connected"
                     variant                         boolean true
                  )
`
	btTransport = `      dict entry(
         object path "/org/bluez/hci0/dev_00_11_22_33_44_55/fd0"
         array [
            dict entry(
               string "org.bluez.MediaTransport1"
               array [
                  dict entry(
                     string "Device"
                     variant                         object path "/org/bluez/hci0/dev_00_11_22_33_44_55"
                  )
`
)

func TestSoundConnected(t *testing.T) {
	for _, c := range []struct {
		name string
		out  string
		want string
	}{
		{"no service's answer", "", ""},
		{"the adapter only", btAdapter, ""},
		{"headphones paired, not connected", btAdapter + btPaired, ""},
		{"headphones connected, no sound yet (firmware 6: no transport)", btAdapter + btPaired + btConnected, "00:11:22:33:44:55"},
		{"headphones connected, sound flowing", btAdapter + btPaired + btConnected + btTransport, "00:11:22:33:44:55"},
		{"a transport left, not connected", btAdapter + btPaired + btTransport, ""},
	} {
		if got := soundConnected([]byte(c.out)); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

// The pair to connect: a paired device that plays sound, not one seen in a scan nor a paired keyboard.
func TestPairedAudio(t *testing.T) {
	scanned := strings.ReplaceAll(strings.ReplaceAll(btPaired, "00_11_22_33_44_55", "66_77_88_99_AA_BB"), "boolean true", "boolean false")
	keyboard := strings.ReplaceAll(strings.ReplaceAll(btPaired, "00_11_22_33_44_55", "CC_DD_EE_FF_00_11"), "audio-headphones", "input-keyboard")
	for _, c := range []struct {
		name, out, want string
	}{
		{"nothing", btAdapter, ""},
		{"seen in a scan, not paired", btAdapter + scanned, ""},
		{"a paired keyboard", btAdapter + keyboard, ""},
		{"paired headphones after them", btAdapter + scanned + keyboard + btPaired, "00:11:22:33:44:55"},
		{"connected: its transport after it", btAdapter + btPaired + btTransport, "00:11:22:33:44:55"},
	} {
		if got := pairedAudio([]byte(c.out)); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}
