package panel

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"seeall/internal/plugin"
)

// TestChoiceOptionsFollowTheMachine checks a choice whose list only the running
// machine knows — the graphics cards — is filled from OptionsFor, and that a
// label maps back to the value it stands for. The label is what the row shows;
// the value is what is written down.
func TestChoiceOptionsFollowTheMachine(t *testing.T) {
	def := plugin.Setting{
		Key: "gpu",
		Options: []plugin.Option{
			{Value: "stale", Label: "一张早就拔掉的卡"},
		},
		OptionsFor: func() []plugin.Option {
			return []plugin.Option{
				{Value: "auto", Label: "自动"},
				{Value: "10DE:2520#0", Label: "GeForce RTX 3060 Laptop GPU"},
			}
		},
	}
	options := choiceOptions(def)
	labels, lookup := optionLabels(options)
	if len(labels) != 2 || labels[0] != "自动" || labels[1] != "GeForce RTX 3060 Laptop GPU" {
		t.Fatalf("the rows are %v, want the machine's list", labels)
	}
	if got := lookup["GeForce RTX 3060 Laptop GPU"]; got != "10DE:2520#0" {
		t.Errorf("the row maps to %q, want the card's key", got)
	}
	if _, ok := lookup["一张早就拔掉的卡"]; ok {
		t.Error("a card the machine no longer lists is still on offer")
	}

	// A setting with a fixed list keeps it: OptionsFor is the exception, not
	// the rule.
	fixed := plugin.Setting{Options: []plugin.Option{{Value: "a", Label: "A"}}}
	if labels, _ := optionLabels(choiceOptions(fixed)); len(labels) != 1 || labels[0] != "A" {
		t.Errorf("a fixed list became %v", labels)
	}
}

// TestSettingsChooseTheGPU is the settings page's GPU row end to end: it is
// offered on a machine with more than one card, and picking one writes that
// card's key into the instance's settings.
func TestSettingsChooseTheGPU(t *testing.T) {
	h, p := fixture(t)
	in := h.Instance("system")
	if in == nil {
		t.Fatal("the system monitor is not running")
	}
	var gpu plugin.Setting
	for _, def := range in.Manifest().Settings {
		if def.Key == "gpu" {
			gpu = def
		}
	}
	if gpu.Key == "" {
		t.Fatal("the system monitor has no GPU setting")
	}
	if gpu.OptionsFor == nil {
		t.Fatal("the GPU setting has no list to fill in")
	}
	cards := gpu.OptionsFor()
	if len(cards) < 3 {
		t.Skipf("this machine has %d rows to choose from; the page shows the row from two cards up", len(cards)-1)
	}

	p.SetSelected(SettingsPage)
	p.SetSize(Fit(1920, 1040, 1))
	tt := ui.NewTester(func(c *ui.Context) { p.View(c) }, DesignWidth, DesignHeight)
	row, ok := tt.Find("GPU")
	if !ok {
		t.Fatalf("the settings page offers no GPU row: %v", tt.Texts())
	}
	current, ok := tt.Find(cards[0].Label)
	if !ok {
		t.Fatalf("the GPU row does not show the card it is on: %v", tt.Texts())
	}
	if current.X <= row.X {
		t.Errorf("the choice at x=%v is not beside its label at x=%v", current.X, row.X)
	}

	// Open the row and pick the other card: the same window, the popup over
	// it.
	other := cards[1]
	if err := tt.Click(cards[0].Label); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(other.Label) {
		t.Fatalf("the popup offers no card: %v", tt.Texts())
	}
	if err := tt.Click(other.Label); err != nil {
		t.Fatalf("the popup does not offer %q: %v", other.Label, err)
	}
	if got := in.Settings().Get("gpu"); got != other.Value {
		t.Errorf("the setting is %q after choosing %q, want %q", got, other.Label, other.Value)
	}

	// And the page it drives is about that card.
	h.RefreshOne("system")
	var detail string
	for _, r := range in.Snapshot().Readings {
		if r.ID == "gpu" {
			detail = r.Detail
		}
	}
	if detail == "" {
		t.Fatalf("the system page shows no GPU reading: %v", in.Snapshot().Readings)
	}
	if detail != cardName(cards, other.Value) {
		t.Errorf("the GPU reading is about %q, want %q", detail, cardName(cards, other.Value))
	}
}

// TestAChoiceNeedsSomethingToChoose checks the settings page draws no row for
// a choice with one answer. A machine without a graphics card would otherwise
// show "GPU: 自动", a question nobody can answer any other way.
func TestAChoiceNeedsSomethingToChoose(t *testing.T) {
	h, p := fixture(t)
	in := h.Instance("system")
	if in == nil {
		t.Fatal("the system monitor is not running")
	}
	row := func(options []plugin.Option) *ui.Tester {
		def := plugin.Setting{
			Key: "gpu", Label: "显卡", Type: plugin.SettingChoice,
			OptionsFor: func() []plugin.Option { return options },
		}
		return ui.NewTester(func(c *ui.Context) {
			p.settingField(c, paletteFor(c), in, def, map[string]string{"gpu": options[0].Value})
		}, DesignWidth, 160)
	}

	if tt := row([]plugin.Option{{Value: "auto", Label: "自动"}}); tt.HasText("显卡") {
		t.Errorf("a one-answer choice is still a row: %v", tt.Texts())
	}
	two := []plugin.Option{{Value: "auto", Label: "自动"}, {Value: "10DE:2520#0", Label: "GeForce RTX 3060"}}
	tt := row(two)
	if !tt.HasText("显卡") {
		t.Fatalf("the choice with two answers is missing: %v", tt.Texts())
	}
	if !tt.HasText("自动") {
		t.Errorf("the row does not show the value it is on: %v", tt.Texts())
	}
}

// TestAMissingChoiceStaysVisible checks the row keeps a card the machine no
// longer lists: showing "自动" instead would contradict the page that says the
// card is gone, and leave nothing to switch away from.
func TestAMissingChoiceStaysVisible(t *testing.T) {
	h, p := fixture(t)
	in := h.Instance("system")
	if in == nil {
		t.Fatal("the system monitor is not running")
	}
	in.Settings().Set("gpu", "FFFF:0000#0")
	def := plugin.Setting{
		Key: "gpu", Label: "GPU", Type: plugin.SettingChoice, Default: "auto",
		OptionsFor: func() []plugin.Option {
			return []plugin.Option{{Value: "auto", Label: "自动"}}
		},
	}
	p.drafts = nil
	tt := ui.NewTester(func(c *ui.Context) {
		p.settingField(c, paletteFor(c), in, def, p.draftsFor(in))
	}, DesignWidth, 160)
	if !tt.HasText(missingChoice) {
		t.Fatalf("the row lost the card it was on: %v", tt.Texts())
	}
	// The row still offers a way out.
	if err := tt.Click(missingChoice); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("自动"); err != nil {
		t.Fatalf("the popup does not offer the way back: %v", err)
	}
	if got := in.Settings().Get("gpu"); got != "auto" {
		t.Errorf("the setting is %q after choosing 自动, want auto", got)
	}
}

func cardName(cards []plugin.Option, value string) string {
	for _, c := range cards {
		if c.Value == value {
			return c.Label
		}
	}
	return ""
}
