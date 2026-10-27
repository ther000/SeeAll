package panel

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"seeall/internal/app"
	"seeall/internal/plugin"
)

// settings is the panel's settings page: which monitors run, what each one
// watches, and how the app itself behaves.
func (a *App) settings(c *ui.Context, p palette, instances []*app.Instance) {
	ui.Scroll(c).Fill().Padding(CardGap, Gutter).Gap(CardGap).Children(func() {
		ui.Text(c, "设置").FontSize(13).Bold()
		card(c, p).Children(func() {
			label(c, p, "应用")
			controlRow(c, p, "开机自启", "登录后在托盘里待命", func() {
				on := a.Host.StartAtLogin()
				if ui.Switch(c, &on).Label("开机自启").Changed() {
					if err := mygo.App.SetOpenAtLogin(on); err != nil {
						c.Toast("改不了开机自启：" + err.Error())
						return
					}
					a.Host.SetStartAtLogin(on)
				}
			})
			hline(c, p)
			controlRow(c, p, "通知", "电量低、状态异常时弹系统通知", func() {
				if ui.Button(c, "").Label("全部刷新").Children(func() {
					ui.Text(c, "全部刷新").FontSize(11).SingleLine()
				}).Clicked() {
					a.Host.Refresh()
					c.Toast("已刷新全部监控项")
				}
			})
		})

		for _, in := range a.sorted(instances) {
			a.instanceCard(c, p, in)
		}

		a.addCard(c, p)
	})
}

// instanceCard is one monitor's settings: the fields its plugin declares,
// plus removing it.
func (a *App) instanceCard(c *ui.Context, p palette, in *app.Instance) {
	m := in.Manifest()
	drafts := a.draftsFor(in)

	card(c, p).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconFor(m.Icon)).FontSize(13).TextColor(p.TextMuted)
			ui.Column(c).Grow(1).Gap(0).Children(func() {
				ui.Text(c, in.Name).FontSize(12).Bold().NoWrap()
				ui.Text(c, m.Summary).FontSize(11).TextColor(p.TextMuted).Wrap()
			})
			if m.Experimental {
				badge(c, "实验", p.Warn)
			}
			confirming := a.confirmRemove == in.ID
			buttonLabel := "移除"
			if confirming {
				buttonLabel = "确认移除"
			}
			ui.Row(c).Shrink(0).Children(func() {
				if ui.Button(c, "").Label(buttonLabel).Tooltip("移除这个监控项").Children(func() {
					ui.Text(c, buttonLabel).FontSize(11).SingleLine()
				}).Clicked() {
					if confirming {
						a.confirmRemove = ""
						a.Host.Remove(in.ID)
						for key := range a.drafts {
							if strings.HasPrefix(key, in.ID+"/") {
								delete(a.drafts, key)
							}
						}
						a.ensureSelection(a.Host.Instances())
						a.SetSelected(a.Selected())
						c.Toast("已移除 " + in.Name)
					} else {
						a.confirmRemove = in.ID
					}
				}
			})
		})

		for _, def := range m.Settings {
			a.settingField(c, p, in, def, drafts)
		}
	})
}

// draftsFor gives each instance a scratch copy of its values, which the
// text fields edit: a keystroke changes the draft, and the value is written
// when the field loses the focus or when it is a number that parses.
func (a *App) draftsFor(in *app.Instance) map[string]string {
	if a.drafts == nil {
		a.drafts = map[string]string{}
	}
	out := map[string]string{}
	for _, def := range in.Manifest().Settings {
		key := in.ID + "/" + def.Key
		v, ok := a.drafts[key]
		if !ok {
			v = in.Settings().Get(def.Key)
			if v == "" {
				v = def.Default
			}
			a.drafts[key] = v
		}
		out[def.Key] = v
	}
	return out
}

func (a *App) commit(in *app.Instance, key, value string) {
	a.drafts[in.ID+"/"+key] = value
	in.Settings().Set(key, value)
}

func (a *App) settingField(c *ui.Context, p palette, in *app.Instance, def plugin.Setting, drafts map[string]string) {
	switch def.Type {
	case plugin.SettingBool:
		on := in.Settings().Bool(def.Key, def.Default == "true")
		controlRow(c, p, def.Label, def.Description, func() {
			if ui.Switch(c, &on).Label(def.Label).Changed() {
				a.commit(in, def.Key, strconv.FormatBool(on))
			}
		})
	case plugin.SettingChoice:
		options := choiceOptions(def)
		// A value the machine no longer offers — a graphics card that was
		// taken out — stays in the row, with nothing to call it. Quietly
		// showing another value would hide the reason the page beside it says
		// the card is gone, and leave the user nothing to change.
		current := drafts[def.Key]
		if _, ok := labelOf(options, current); !ok && current != "" {
			options = append(options, plugin.Option{Value: current, Label: missingChoice})
		}
		// A choice with nothing to choose — a machine with no graphics card
		// to pick — is not a row: the page would be asking a question with one
		// answer.
		if len(options) < 2 {
			return
		}
		labels, lookup := optionLabels(options)
		shown, _ := labelOf(options, current)
		controlRow(c, p, def.Label, def.Description, func() {
			if ui.Select(c, &shown, labels).Label(def.Label).Changed() {
				a.commit(in, def.Key, lookup[shown])
			}
		})
	default:
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, def.Label).FontSize(12)
			value := drafts[def.Key]
			field := ui.TextInput(c, &value).Placeholder(def.Placeholder).Label(def.Label)
			if def.Secret {
				field.Password()
			}
			field.FillWidth()
			if value != drafts[def.Key] {
				a.drafts[in.ID+"/"+def.Key] = value
				// Numbers are written as they parse; text is written
				// when the user presses Enter, so that a half-typed host
				// name does not restart a poller.
				if def.Type == plugin.SettingNumber && (value == "" || parsesNumber(value)) {
					a.commit(in, def.Key, value)
				}
			}
			if field.Submitted() {
				a.commit(in, def.Key, value)
			}
			if def.Description != "" {
				ui.Text(c, def.Description).FontSize(11).TextColor(p.TextMuted).Wrap()
			}
		})
	}
}

// controlRow is one line of a settings card: what it is and what it does on
// the left, its control on the right. The text grows and the control keeps its
// own width, so every control on the page ends on the card's inner edge.
func controlRow(c *ui.Context, p palette, title, detail string, control func()) {
	ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
		ui.Column(c).Grow(1).Gap(1).Children(func() {
			ui.Text(c, title).FontSize(12)
			if detail != "" {
				// The detail wraps rather than running under the control on
				// its right: a long one would be read as part of the choice.
				ui.Text(c, detail).FontSize(11).TextColor(p.TextMuted).Wrap()
			}
		})
		ui.Row(c).Shrink(0).AlignItems(ui.Center).Children(control)
	})
}

// choiceOptions is what a choice offers right now. A setting whose list only
// the running machine knows — the cards it has — fills it in here, once per
// frame; the rest keep the list their manifest declared.
func choiceOptions(def plugin.Setting) []plugin.Option {
	if def.OptionsFor != nil {
		return def.OptionsFor()
	}
	return def.Options
}

// optionLabels is the list of labels the drop-down shows, and the way back
// from a label to the value it stands for.
func optionLabels(options []plugin.Option) ([]string, map[string]string) {
	labels := make([]string, 0, len(options))
	lookup := make(map[string]string, len(options))
	for _, o := range options {
		labels = append(labels, o.Label)
		lookup[o.Label] = o.Value
	}
	return labels, lookup
}

// labelOf finds the label a value is shown under.
func labelOf(options []plugin.Option, value string) (string, bool) {
	for _, o := range options {
		if o.Value == value {
			return o.Label, true
		}
	}
	return "", false
}

// missingChoice is what a choice calls a value the machine no longer offers.
const missingChoice = "未找到"

func parsesNumber(s string) bool {
	_, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return err == nil
}

// addCard offers every plugin the page could run that it does not run now: a
// monitor the user removed comes back from here, with its settings as they
// were.
func (a *App) addCard(c *ui.Context, p palette) {
	missing := a.missingPlugins()
	if len(missing) == 0 {
		return
	}

	card(c, p).Children(func() {
		label(c, p, "添加")
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			names := make([]string, len(missing))
			for i, pl := range missing {
				names[i] = pl.Manifest().Name
			}
			chosen := names[0]
			for i := range missing {
				if a.addChoice == missing[i].Manifest().ID {
					chosen = names[i]
				}
			}
			if ui.Select(c, &chosen, names).Grow(1).Label("插件").Changed() {
				a.addChoice = pluginIDNamed(missing, chosen)
			}
			if ui.Button(c, "").Label("添加监控项").Children(func() {
				ui.Icon(c, iconPlus).FontSize(13)
				ui.Text(c, "添加").FontSize(12).SingleLine()
			}).Clicked() {
				a.add(c, p, pluginIDNamed(missing, chosen))
			}
		})
	})
}

// missingPlugins lists the plugins without an instance, in the order the tabs
// would show them.
func (a *App) missingPlugins() []plugin.Plugin {
	running := map[string]bool{}
	for _, in := range a.Host.Instances() {
		running[in.PluginID] = true
	}
	var out []plugin.Plugin
	for _, pl := range a.Host.Plugins() {
		m := pl.Manifest()
		if running[m.ID] {
			continue
		}
		out = append(out, pl)
	}
	return out
}

// add starts a monitor again from the settings page. It comes back with the
// settings it had: the host keeps those of a monitor the user removed.
func (a *App) add(c *ui.Context, p palette, pluginID string) {
	pl := a.Host.Plugin(pluginID)
	if pl == nil {
		return
	}
	m := pl.Manifest()
	name := uniqueName(a.Host, m)
	added := a.Host.Add(m.ID, name, nil)
	if added == nil {
		c.Toast("加不上 " + name)
		return
	}
	a.addChoice = ""
	a.SetSelected(added.ID)
	c.Toast("已添加 " + added.Name)
}

// pluginIDNamed maps the label the select shows back to a plugin's ID.
func pluginIDNamed(plugins []plugin.Plugin, name string) string {
	for _, pl := range plugins {
		if pl.Manifest().Name == name {
			return pl.Manifest().ID
		}
	}
	return ""
}

// uniqueName makes a name that no instance of the plugin uses yet.
func uniqueName(h *app.Host, m plugin.Manifest) string {
	base := m.DefaultInstance
	used := map[string]bool{}
	for _, in := range h.Instances() {
		if in.PluginID == m.ID {
			used[in.Name] = true
		}
	}
	if !used[base] {
		return base
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s %d", base, i)
		if !used[candidate] {
			return candidate
		}
	}
}
