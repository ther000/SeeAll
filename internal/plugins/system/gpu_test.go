package system

import "testing"

// TestParseLuidReadsTheAdapterFromACounterName is the whole reason the page
// knows which card a counter is about: the names come from Windows, and a slip
// in the offsets silently sums every card into one number.
func TestParseLuidReadsTheAdapterFromACounterName(t *testing.T) {
	cases := []struct {
		name string
		want uint64
		ok   bool
	}{
		{"pid_12772_luid_0x00000000_0x0000E3CD_phys_0_eng_0_engtype_3D", 0xE3CD, true},
		{"luid_0x00000000_0x000103C6_phys_0_part_0", 0x103C6, true},
		{"pid_7_luid_0x00000001_0xFFFFFFFF_phys_1_eng_12_engtype_Copy", 0x1FFFFFFFF, true},
		{"pid_7_luid_0x0000000_0x0000E3C_phys_0", 0, false},
		{"\\GPU Engine 0", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		got, ok := parseLuid(c.name)
		if ok != c.ok || got != c.want {
			t.Errorf("parseLuid(%q) = %#x, %v; want %#x, %v", c.name, got, ok, c.want, c.ok)
		}
	}
}

// TestShortGPUNameKeepsTheModel checks the settings row shows "GeForce RTX
// 3060 Laptop GPU" rather than the driver's full trademarked string.
func TestShortGPUNameKeepsTheModel(t *testing.T) {
	cases := map[string]string{
		"NVIDIA GeForce RTX 3060 Laptop GPU": "GeForce RTX 3060 Laptop GPU",
		"AMD Radeon(TM) Graphics":            "Radeon Graphics",
		"Intel(R) UHD Graphics 630":          "UHD Graphics 630",
		"Microsoft Basic Render Driver":      "Microsoft Basic Render Driver",
	}
	for in, want := range cases {
		if got := shortGPUName(in); got != want {
			t.Errorf("shortGPUName(%q) = %q, want %q", in, got, want)
		}
	}
}
