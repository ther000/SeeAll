package mouse

import (
	"testing"

	"seeall/internal/hid"
)

// TestBatteryChannelIsReadWhateverTheVendor: the receiver is recognised by the
// collection it broadcasts on, so a mouse whose vendor number nobody looked up
// is still found — and a collection that is not that one is not.
func TestBatteryChannelIsReadWhateverTheVendor(t *testing.T) {
	cases := []struct {
		name string
		intf hid.Interface
		want bool
	}{
		{"the receiver this package was written against",
			hid.Interface{VendorID: 0x24AE, ProductID: 0x185A, UsagePage: 0xFF00, Usage: 0x0002}, true},
		{"a receiver no one typed a number for",
			hid.Interface{VendorID: 0x3554, ProductID: 0x0001, UsagePage: 0xFF00, Usage: 0x0002}, true},
		{"another collection of the same receiver",
			hid.Interface{VendorID: 0x24AE, ProductID: 0x185A, UsagePage: 0xFF00, Usage: 0x0001}, false},
		{"an ordinary mouse collection",
			hid.Interface{VendorID: 0x24AE, ProductID: 0x185A, UsagePage: 0x0001, Usage: 0x0002}, false},
	}
	for _, c := range cases {
		if got := IsBatteryChannel(c.intf); got != c.want {
			t.Errorf("%s: IsBatteryChannel = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestTheMouseAsksForNothing pins what auto-detection bought: the settings page
// has no field for the mouse, because there is nothing about a receiver the
// user would have to know. A field here means the receiver is being found by
// hand again.
func TestTheMouseAsksForNothing(t *testing.T) {
	if settings := Manifest().Settings; len(settings) != 0 {
		t.Errorf("the mouse declares %v, want no settings", settings)
	}
}
