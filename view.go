package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

const (
	titleBarHeight = 52 // the height of an inset title bar
	sidebarWidth   = 220
)

func icon(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` + shapes + `</svg>`))
}

var (
	allIcon     = icon(`<path d="M12 2 2 7l10 5 10-5-10-5Z"/><path d="m2 17 10 5 10-5"/><path d="m2 12 10 5 10-5"/>`)
	serverIcon  = icon(`<rect x="2" y="3" width="20" height="8" rx="2"/><rect x="2" y="13" width="20" height="8" rx="2"/><path d="M6 7h.01M6 17h.01"/>`)
	plusIcon    = icon(`<path d="M12 5v14M5 12h14"/>`)
	refreshIcon = icon(`<path d="M21 12a9 9 0 1 1-3-6.7L21 8"/><path d="M21 3v5h-5"/>`)
	tunnelIcon  = icon(`<path d="M8 3 4 7l4 4"/><path d="M4 7h16"/><path d="m16 21 4-4-4-4"/><path d="M20 17H4"/>`)
)

// appleTheme returns the frame's default theme, which follows the system's
// appearance and accent color, with Apple's system colors and SwiftUI's
// rounder corners.
func appleTheme(c *ui.Context) *ui.Theme {
	t := *c.Theme()
	pick := func(light, dark string) ui.Color {
		if t.Dark {
			return ui.Hex(dark)
		}
		return ui.Hex(light)
	}
	t.Background = pick("#f2f2f7", "#1c1c1e") // systemGroupedBackground
	t.Surface = pick("#e5e5ea", "#3a3a3c")    // systemGray5
	t.SurfaceHover = pick("#d1d1d6", "#48484a")
	t.SurfacePressed = pick("#c7c7cc", "#636366")
	t.Border = pick("#c6c6c8", "#38383a") // separator
	t.Text = pick("#000000", "#ffffff")
	t.TextMuted = ui.Hex("#8e8e93") // systemGray
	t.Danger = pick("#ff3b30", "#ff453a")
	t.Warning = pick("#ff9500", "#ff9f0a")
	t.Success = pick("#34c759", "#30d158")
	t.Radius = 8
	return &t
}

// cardColor is secondarySystemGroupedBackground, the face of a grouped list.
func cardColor(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.Hex("#2c2c2e")
	}
	return ui.Hex("#ffffff")
}

func (a *app) view(c *ui.Context) {
	t := appleTheme(c)
	c.SetTheme(t)
	// A material shows wherever the view draws no background: the sidebar.
	vibrant := c.Vibrancy()
	if vibrant {
		c.Root().Background(ui.Transparent)
	}
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		side := ui.Column(c).Width(sidebarWidth).Shrink(0)
		if !vibrant {
			side.Background(t.Surface)
		}
		side.Children(func() { a.sidebar(c, vibrant) })
		ui.Box(c).Width(1).Background(t.Border)
		ui.Column(c).Grow(1).MinWidth(0).Background(t.Background).Children(func() { a.pane(c) })
	})
	a.editor(c)
}

func (a *app) sidebar(c *ui.Context, vibrant bool) {
	t := c.Theme()
	ui.Box(c).Height(titleBarHeight).Shrink(0).DragWindow()
	badge := func(host string) {
		if n := a.active(host); n > 0 {
			ui.Badge(c, strconv.Itoa(n))
		}
	}
	list := ui.Sidebar(c, &a.selected, func() {
		ui.SidebarSection(c, "隧道", nil, func() {
			ui.SidebarItem(c, allID, allIcon, "全部隧道").Children(func() { badge("") })
		})
		ui.SidebarSection(c, "SSH 主机", nil, func() {
			for _, h := range a.hosts {
				ui.SidebarItem(c, hostID(h.Alias), serverIcon, h.Alias).Tooltip(h.Address()).
					Children(func() { badge(h.Alias) })
			}
		})
	}).Grow(1).Label("主机")
	if vibrant {
		list.Background(ui.Transparent)
	}
	// Where the hosts come from, and a way to read them again.
	ui.Row(c).Padding(8, 12).Gap(6).AlignItems(ui.Center).Children(func() {
		note := fmt.Sprintf("~/.ssh/config 中有 %d 个主机", len(a.hosts))
		if a.hostsErr != "" {
			note = "无法读取 ~/.ssh/config"
		}
		ui.Text(c, note).FontSize(11).TextColor(t.TextMuted).Grow(1).SingleLine().Tooltip(a.hostsErr)
		if toolButton(c, "reload", refreshIcon, "重新读取 SSH 配置").Clicked() {
			a.reloadHosts()
		}
	})
}

// toolButton is a borderless icon button, as in a macOS toolbar.
func toolButton(c *ui.Context, key string, svg *ui.SVG, label string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c.Key(key)).Size(28, 24).Radius(6).Center().TextColor(t.TextMuted).Label(label).Tooltip(label)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() { ui.Icon(c, svg).Size(14, 14) })
	return b
}

func (a *app) pane(c *ui.Context) {
	t := c.Theme()
	bar := c.TitleBar()
	title, subtitle := "全部隧道", fmt.Sprintf("已开启 %d / %d", a.active(""), len(a.tunnels))
	host := ""
	if a.selected != allID {
		host = a.selected[len(hostID("")):]
		title, subtitle = host, host
		if i := slices.IndexFunc(a.hosts, func(h Host) bool { return h.Alias == host }); i >= 0 {
			subtitle = a.hosts[i].Address()
		}
	}
	ui.Row(c).Height(titleBarHeight).Shrink(0).AlignItems(ui.Center).Gap(12).
		Padding(0, bar.Right+16, 0, 20).DragWindow().Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, title).FontSize(15).Bold().SingleLine()
			ui.Text(c, subtitle).FontSize(11).TextColor(t.TextMuted).SingleLine()
		})
		// A capsule of Liquid Glass, as a toolbar button on macOS 26.
		add := ui.Row(c.Key("add")).Padding(6, 12).Gap(6).Radius(999).AlignItems(ui.Center).
			Material(glass.Glass{Tint: t.Accent, Interactive: true}).Label("新建隧道").Children(func() {
			ui.Icon(c, plusIcon).Size(13, 13).TextColor(t.AccentText)
			ui.Text(c, "新建隧道").FontSize(12).FontWeight(600).TextColor(t.AccentText).SingleLine()
		})
		if add.Clicked() {
			a.edit(Tunnel{Host: host})
		}
	})
	ts := a.visible()
	if len(ts) == 0 {
		a.empty(c, host)
		return
	}
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Padding(8, 20, 24).Gap(8).Children(func() {
			// An inset grouped list, as SwiftUI's List draws one.
			ui.Column(c).Radius(12).Clip().Background(cardColor(t)).
				Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05)).Children(func() {
				for i, tn := range ts {
					if i > 0 {
						ui.Box(c).Height(1).Margin(0, 0, 0, 38).Background(t.Border)
					}
					a.row(c, tn, host == "")
				}
			})
			ui.Text(c, "点击隧道可编辑。连接沿用你的 ssh 配置：密钥、ProxyJump 等。").
				FontSize(11).TextColor(t.TextMuted).Padding(0, 14)
		})
	})
}

func (a *app) row(c *ui.Context, tn Tunnel, showHost bool) {
	t := c.Theme()
	st := a.state(tn.ID)
	dot, status := t.TextMuted.Alpha(0.5), "已关闭"
	switch st.status {
	case Connecting:
		dot, status = t.Warning, "连接中…"
	case Connected:
		dot, status = t.Success, "已连接"
	case Failed:
		dot, status = t.Danger, "失败"
	}
	row := ui.Row(c.Key(tn.ID)).Padding(10, 14).Gap(12).AlignItems(ui.Center).Label(tn.Name)
	if row.Hovered() {
		row.Background(t.Surface.Alpha(0.5))
	}
	row.Children(func() {
		ui.Box(c).Size(10, 10).Radius(5).Background(dot).Shrink(0).Tooltip(status)
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, tn.Name).FontWeight(500).SingleLine()
			summary := tn.Summary()
			if showHost {
				summary += "  ·  经由 " + tn.Host
			}
			ui.Text(c, summary).Font("monospace").FontSize(11).TextColor(t.TextMuted).SingleLine()
			if st.msg != "" {
				color := t.Danger
				if st.status != Failed { // open, but its target is unreachable
					color = t.Warning
				}
				ui.Text(c, st.msg).FontSize(11).TextColor(color).MaxLines(2).Selectable()
			}
		})
		if st.status == Connecting {
			ui.Text(c, status).FontSize(11).TextColor(t.TextMuted).SingleLine()
		}
		if ui.Switch(c, &st.on).Label("启用 " + tn.Name).Changed() {
			a.setOpen(tn.ID, st.on)
		}
	})
	row.ContextMenu(func(m *ui.Menu) {
		if m.Item("编辑…").Chosen() {
			a.edit(tn)
		}
		if m.Item("删除").Chosen() {
			a.remove(tn.ID)
		}
	})
	if row.Clicked() {
		a.edit(tn)
	}
}

func (a *app) empty(c *ui.Context, host string) {
	t := c.Theme()
	ui.Column(c).Grow(1).Center().Gap(10).Padding(24).Children(func() {
		ui.Icon(c, tunnelIcon).Size(40, 40).TextColor(t.TextMuted.Alpha(0.6))
		ui.Text(c, "还没有隧道").FontSize(17).Bold()
		hint := "通过某个 SSH 主机转发端口。"
		if len(a.hosts) == 0 {
			hint = "先在 ~/.ssh/config 中添加 Host，再在侧边栏底部重新读取。"
		}
		ui.Text(c, hint).TextColor(t.TextMuted).TextAlign(ui.Center).MaxWidth(320)
		if len(a.hosts) > 0 && ui.PrimaryButton(c.Key("empty-add"), "新建隧道").Radius(999).Padding(6, 16).Clicked() {
			a.edit(Tunnel{Host: host})
		}
	})
}

// draft is the tunnel the editor changes, its ports as typed.
type draft struct {
	tunnel     Tunnel // its ID is "" for a new one
	kind       int    // index in kinds
	listenPort string
	targetPort string
}

func newDraft(tn Tunnel) draft {
	return draft{tunnel: tn, kind: max(slices.Index(kinds, tn.Kind), 0),
		listenPort: fmtPort(tn.ListenPort), targetPort: fmtPort(tn.TargetPort)}
}

// fmtPort shows a port in an input, empty while unset.
func fmtPort(p int) string {
	if p == 0 {
		return ""
	}
	return strconv.Itoa(p)
}

// result returns the tunnel the editor describes.
func (d *draft) result() Tunnel {
	tn := d.tunnel
	tn.Kind = kinds[d.kind]
	tn.ListenHost, tn.TargetHost = strings.TrimSpace(tn.ListenHost), strings.TrimSpace(tn.TargetHost)
	tn.ListenPort, _ = strconv.Atoi(strings.TrimSpace(d.listenPort))
	tn.TargetPort, _ = strconv.Atoi(strings.TrimSpace(d.targetPort))
	if tn.Kind == KindDynamic {
		tn.TargetHost, tn.TargetPort = "", 0
	}
	if tn.Name = strings.TrimSpace(tn.Name); tn.Name == "" {
		tn.Name = tn.Summary()
	}
	return tn
}

// preset is a common tunnel the editor fills in with one click.
type preset struct {
	label  string
	tunnel Tunnel
}

var presets = []preset{
	{"动态代理：SOCKS5 1080", Tunnel{Name: "SOCKS5 代理", Kind: KindDynamic, ListenHost: "127.0.0.1", ListenPort: 1080}},
	{"本地转发：MySQL 3306 → 本机 3307", Tunnel{Name: "MySQL", Kind: KindLocal, ListenHost: "127.0.0.1", ListenPort: 3307, TargetHost: "127.0.0.1", TargetPort: 3306}},
	{"本地转发：Web 80 → 本机 8080", Tunnel{Name: "Web", Kind: KindLocal, ListenHost: "127.0.0.1", ListenPort: 8080, TargetHost: "127.0.0.1", TargetPort: 80}},
	{"远程转发：服务器 9000 → 本机 3000", Tunnel{Name: "远程转发 9000", Kind: KindRemote, ListenHost: "127.0.0.1", ListenPort: 9000, TargetHost: "127.0.0.1", TargetPort: 3000}},
}

// fill puts a preset in the draft, keeping the tunnel's identity, host and
// a name the user typed.
func (d *draft) fill(p Tunnel) {
	p.ID, p.Host = d.tunnel.ID, d.tunnel.Host
	typed := d.tunnel.Name != "" && !slices.ContainsFunc(presets, func(q preset) bool {
		return q.tunnel.Name == d.tunnel.Name
	})
	if typed {
		p.Name = d.tunnel.Name
	}
	*d = newDraft(p)
}

// edit opens the editor on tn; a new tunnel has no ID.
func (a *app) edit(tn Tunnel) {
	if tn.Host == "" && len(a.hosts) > 0 {
		tn.Host = a.hosts[0].Alias
	}
	if tn.Kind == "" {
		tn.Kind, tn.ListenHost, tn.TargetHost = KindLocal, "127.0.0.1", "127.0.0.1"
	}
	a.draft = newDraft(tn)
	a.editing = true
}

// sideText titles and describes where a tunnel listens or what it reaches,
// which depends on its kind.
type sideText struct{ title, about string }

var (
	kindAbout = []string{
		"在本机开一个端口，经服务器访问目标",
		"在服务器开一个端口，访问本机上的目标",
		"在本机开一个 SOCKS5 代理，流量从服务器发出",
	}
	listenText = map[Kind]sideText{
		KindLocal:   {"本机监听地址", "本机将监听此地址，通常为 127.0.0.1"},
		KindRemote:  {"服务器监听地址", "服务器将监听此地址；对外开放需服务器开启 GatewayPorts"},
		KindDynamic: {"本机监听地址", "SOCKS5 代理在本机监听的地址，通常为 127.0.0.1"},
	}
	targetText = map[Kind]sideText{
		KindLocal:  {"远端目标地址", "从服务器访问的地址：通常是服务器上的 127.0.0.1 或其内网主机地址"},
		KindRemote: {"本机目标地址", "连接将转发到本机的这个地址，通常为 127.0.0.1"},
	}
)

func (a *app) editor(c *ui.Context) {
	d := &a.draft
	ui.DialogBase(c, &a.editing, func(backdrop, panel ui.Element) {
		t := c.Theme()
		_, h := c.Size()
		backdrop.Background(ui.RGBA(0, 0, 0, 0.3))
		panel.Width(620).MaxHeight(h-40).Radius(16).Clip().Background(t.Background).
			Shadow(0, 12, 40, 0, ui.RGBA(0, 0, 0, 0.3)).InsetShadow(0, 0, 0, 1, t.Border.Alpha(0.6))
		heading := "新建隧道"
		if d.tunnel.ID != "" {
			heading = "编辑隧道"
		}
		ui.Text(c, heading).FontSize(15).Bold().Padding(18, 20, 10).Shrink(0)
		hosts := make([]string, len(a.hosts))
		for i, h := range a.hosts {
			hosts[i] = h.Alias
		}
		kind := kinds[d.kind]
		ui.Scroll(c).Shrink(1).MinHeight(0).Children(func() {
			ui.Column(c).Padding(0, 20, 4).Gap(14).Children(func() {
				section(c, func() {
					setting(c, "名称", "显示在列表和菜单栏中，留空则自动生成", func() {
						ui.TextInput(c, &d.tunnel.Name).Placeholder("例如 MySQL").Width(258).Label("名称").AutoFocus()
					})
				}, func() {
					setting(c, "SSH 主机", "来自 ~/.ssh/config", func() {
						ui.Select(c, &d.tunnel.Host, hosts).Width(258).Label("SSH 主机")
					})
				}, func() {
					setting(c, "转发类型", kindAbout[d.kind], func() {
						ui.Segmented(c, &d.kind, "本地转发", "远程转发", "动态代理").Label("转发类型")
					})
				})
				rows := []func(){func() {
					s := listenText[kind]
					setting(c, s.title, s.about, func() { addressInputs(c, s.title, &d.tunnel.ListenHost, &d.listenPort) })
				}}
				if s, ok := targetText[kind]; ok {
					rows = append(rows, func() {
						setting(c, s.title, s.about, func() { addressInputs(c, s.title, &d.tunnel.TargetHost, &d.targetPort) })
					})
				}
				section(c, rows...)
				section(c, func() {
					setting(c, "快速填充", "一键填好常见场景，再按需修改", func() {
						ui.Column(c).Gap(6).AlignItems(ui.Start).Children(func() {
							for _, p := range presets {
								if chip(c, p.label).Clicked() {
									d.fill(p.tunnel)
								}
							}
						})
					})
				})
			})
		})
		tn := d.result()
		invalid := tn.Validate()
		ui.Row(c).Padding(14, 20, 18).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
			if tn.ID != "" && ui.Button(c, "删除").TextColor(t.Danger).Clicked() {
				a.remove(tn.ID)
				a.editing = false
			}
			if invalid != nil {
				ui.Text(c, invalid.Error()).FontSize(11).TextColor(t.TextMuted).Grow(1).TextAlign(ui.End)
			} else {
				ui.Spacer(c)
			}
			if ui.Button(c, "取消").Clicked() {
				a.editing = false
			}
			if ui.PrimaryButton(c, "保存").Disabled(invalid != nil).Clicked() {
				if tn.ID == "" {
					tn.ID = newID()
				}
				a.put(tn)
				a.selected = hostID(tn.Host)
				a.editing = false
			}
		})
	})
}

// section groups settings rows on a card, between hairlines, as the
// grouped forms of macOS's settings.
func section(c *ui.Context, rows ...func()) {
	t := c.Theme()
	ui.Column(c).Radius(10).Clip().Background(cardColor(t)).Children(func() {
		for i, row := range rows {
			if i > 0 {
				ui.Box(c).Height(1).Background(t.Border.Alpha(0.6))
			}
			row()
		}
	})
}

// setting is a row of a section: its title and what it is for on the
// left, its controls on the right.
func setting(c *ui.Context, title, about string, controls func()) {
	t := c.Theme()
	ui.Row(c).Padding(12, 16).Gap(16).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, title).FontWeight(600)
			ui.Text(c, about).FontSize(11).TextColor(t.TextMuted)
		})
		ui.Row(c).Gap(8).AlignItems(ui.Center).Shrink(0).Children(controls)
	})
}

// addressInputs edits a host and a port, side by side.
func addressInputs(c *ui.Context, label string, host, port *string) {
	ui.TextInput(c, host).Placeholder("127.0.0.1").Width(170).Label(label)
	ui.TextInput(c, port).Placeholder("端口").Width(80).Label(label + "端口")
}

// chip is a small bordered button, as a suggestion to pick.
func chip(c *ui.Context, label string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c.Key(label)).Padding(5, 10).Radius(6).Border(1, t.Border).Label(label)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() { ui.Text(c, label).FontSize(12).SingleLine() })
	return b
}
