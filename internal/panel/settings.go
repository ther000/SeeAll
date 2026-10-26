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
	a.header(c, p, "设置", fmt.Sprintf("%d 个监控项 · %d 个插件",
		len(instances), len(a.Host.Plugins())), emptySnap(), false)

	ui.Scroll(c).Grow(1).Shrink(1).Basis(0).Padding(16, 2, 16, 14).Gap(10).Children(func() {
		card(c, p).Children(func() {
			label(c, p, "应用")
			ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
				ui.Column(c).Grow(1).Gap(1).Children(func() {
					ui.Text(c, "开机自启").FontSize(12)
					ui.Text(c, "登录后在托盘里待命").FontSize(11).TextColor(p.TextMuted)
				})
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
			ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
				ui.Column(c).Grow(1).Gap(1).Children(func() {
					ui.Text(c, "通知").FontSize(12)
					ui.Text(c, "电量低、状态异常时弹系统通知").FontSize(11).TextColor(p.TextMuted)
				})
				if ui.Button(c, "全部刷新").Clicked() {
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
			if ui.Button(c, "").Label(buttonLabel).Tooltip("移除这个监控项").Children(func() {
				ui.Text(c, buttonLabel).FontSize(11).SingleLine()
			}).Clicked() {
				if confirming {
					a.confirmRemove = ""
					a.Host.Remove(in.ID)
					a.drafts[in.ID] = ""
					a.ensureSelection(a.Host.Instances())
					a.SetSelected(a.Selected())
					c.Toast("已移除 " + in.Name)
				} else {
					a.confirmRemove = in.ID
				}
			}
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
		ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
			ui.Column(c).Grow(1).Gap(0).Children(func() {
				ui.Text(c, def.Label).FontSize(12)
				if def.Description != "" {
					ui.Text(c, def.Description).FontSize(11).TextColor(p.TextMuted).Wrap()
				}
			})
			on := in.Settings().Bool(def.Key, def.Default == "true")
			if ui.Switch(c, &on).Label(def.Label).Changed() {
				a.commit(in, def.Key, strconv.FormatBool(on))
			}
		})
	case plugin.SettingChoice:
		ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
			ui.Text(c, def.Label).FontSize(12).Grow(1)
			values := make([]string, 0, len(def.Options))
			lookup := map[string]string{}
			for _, o := range def.Options {
				values = append(values, o.Label)
				lookup[o.Label] = o.Value
			}
			current := drafts[def.Key]
			chosen := current
			for _, o := range def.Options {
				if o.Value == current {
					chosen = o.Label
				}
			}
			if ui.Select(c, &chosen, values).Label(def.Label).Changed() {
				a.commit(in, def.Key, lookup[chosen])
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

func parsesNumber(s string) bool {
	_, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return err == nil
}

// addCard adds another instance of a plugin that takes several, as a plugin
// with one monitor per host would.
func (a *App) addCard(c *ui.Context, p palette) {
	multi := make([]plugin.Plugin, 0, 2)
	for _, pl := range a.Host.Plugins() {
		if pl.Manifest().Multiple {
			multi = append(multi, pl)
		}
	}
	if len(multi) == 0 {
		return
	}

	card(c, p).Children(func() {
		label(c, p, "添加")
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			names := make([]string, 0, len(multi))
			for _, pl := range multi {
				names = append(names, pl.Manifest().Name)
			}
			chosen := names[0]
			ui.Select(c, &chosen, names).Grow(1).Label("插件")
			if ui.Button(c, "").Label("添加监控项").Children(func() {
				ui.Icon(c, iconPlus).FontSize(13)
				ui.Text(c, "添加").FontSize(12).SingleLine()
			}).Clicked() {
				for _, pl := range multi {
					if pl.Manifest().Name != chosen {
						continue
					}
					m := pl.Manifest()
					name := uniqueName(a.Host, m)
					added := a.Host.Add(m.ID, name, nil)
					if added == nil {
						c.Toast("加不上 " + name)
						return
					}
					a.SetSelected(added.ID)
					c.Toast("已添加 " + added.Name)
					return
				}
			}
		})
		ui.Text(c, "这类插件可以加多个，每个监控一个对象。").FontSize(11).TextColor(p.TextMuted).Wrap()
	})
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
