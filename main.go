package main

import (
	_ "embed"
	"log"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

//go:embed resources/tray.png
var trayIcon []byte

func main() {
	var (
		a        *app
		win      *mygo.Window
		tray     *mygo.Tray
		quitting bool // OnBeforeQuit and OnClose run on the main thread
	)
	// The runner reports from its goroutines; Update brings the news to
	// the main thread, where the state lives.
	run := newSSHRunner(
		func(id string) { win.Update(func() { a.ready(id) }) },
		func(id, msg string) { win.Update(func() { a.connFailed(id, msg) }) },
		func(id, msg string) { win.Update(func() { a.exited(id, msg) }) },
	)
	a = newApp(run, defaultSSHConfig())

	mygo.App.WhenReady(func() {
		if dir, err := mygo.App.Path(mygo.PathUserData); err != nil {
			log.Println("data directory:", err)
		} else {
			a.storePath = filepath.Join(dir, "tunnels.json")
			ts, err := loadTunnels(a.storePath)
			if err != nil {
				log.Println("loading tunnels:", err)
			}
			a.setTunnels(ts)
		}

		win = mygo.NewWindow(mygo.WindowOptions{
			Title:     "Easy SSH Port",
			Width:     760,
			Height:    620,
			MinWidth:  600,
			MinHeight: 440,
			// The view draws under the title bar, the traffic lights inset
			// over the sidebar, whose material shows through.
			TitleBarStyle:  mygo.TitleBarHiddenInset,
			TitleBarHeight: titleBarHeight,
			Vibrancy:       mygo.VibrancySidebar,
			// Opens where the user left it last time.
			StateKey: "main",
			Content:  ui.View(a.view),
		})
		show := func() { win.Show(); win.Focus() }

		var err error
		tray, err = mygo.NewTray(mygo.TrayOptions{
			ID:             "tunnels",
			Icon:           trayIcon,
			IconIsTemplate: true,
			ToolTip:        "Easy SSH Port",
		})
		if err != nil {
			log.Println("tray:", err)
		} else {
			a.onChange = func() { tray.SetMenu(trayMenu(a, win, show)) }
			a.changed()
			// Closing the window keeps the tunnels open, in the menu bar.
			win.OnClose(func(e *mygo.CloseEvent) {
				if !quitting {
					e.PreventDefault()
					win.Hide()
				}
			})
		}
		mygo.App.OnActivate(func(bool) { show() })
	})
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { quitting = true })
	mygo.App.OnQuit(run.stopAll)
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// trayMenu lists the tunnels to open and close them from the menu bar.
func trayMenu(a *app, win *mygo.Window, show func()) *mygo.Menu {
	var items []*mygo.MenuItem
	for _, t := range a.tunnels {
		id, on := t.ID, a.state(t.ID).on
		items = append(items, &mygo.MenuItem{
			Label:   t.Name + "  —  " + t.Host,
			Type:    mygo.MenuItemCheckbox,
			Checked: on,
			Click: func(*mygo.MenuItem, *mygo.Window) {
				a.setOpen(id, !on)
				win.Invalidate()
			},
		})
	}
	if len(items) == 0 {
		items = append(items, &mygo.MenuItem{Label: "没有隧道", Disabled: true})
	}
	return mygo.NewMenu(append(items,
		mygo.Separator(),
		&mygo.MenuItem{Label: "打开 Easy SSH Port…", Click: func(*mygo.MenuItem, *mygo.Window) { show() }},
		&mygo.MenuItem{Role: mygo.RoleQuit},
	))
}
