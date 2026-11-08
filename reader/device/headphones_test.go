package device

import "testing"

// The Kobo's Bluetooth service answers GetManagedObjects, printed by dbus-send, in this shape (measured 2026-11-08 on
// the Libra Colour; addresses made up, most properties left out): a paired device, and once connected its transport.
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
		{"headphones connected", btAdapter + btPaired + btTransport, "00:11:22:33:44:55"},
	} {
		if got := soundConnected([]byte(c.out)); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}
