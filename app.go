package main

import (
	"log"
	"slices"
	"strings"
)

// Status is where a tunnel is in its life.
type Status int

const (
	Off Status = iota
	Connecting
	Connected
	Failed
)

// tunnelState is the state of a tunnel while the app runs. on is what
// its switch shows: the user wants it open.
type tunnelState struct {
	on     bool
	status Status
	msg    string // why it failed, or why its last connection did
}

// app is the state the window shows. Everything in it is touched on the
// main thread only: the view, menu clicks and window.Update.
type app struct {
	run       tunnelRunner
	storePath string // "" keeps the tunnels in memory, as in tests
	onChange  func() // the tray's menu follows the tunnels

	sshConfig string
	hosts     []Host
	hostsErr  string
	tunnels   []Tunnel
	states    map[string]*tunnelState

	selected string // the sidebar's choice: allID or hostID(alias)
	editing  bool
	draft    draft
}

const allID = "all"

func hostID(alias string) string { return "host:" + alias }

func newApp(run tunnelRunner, sshConfig string) *app {
	a := &app{run: run, sshConfig: sshConfig, states: map[string]*tunnelState{}, selected: allID}
	a.reloadHosts()
	return a
}

func (a *app) reloadHosts() {
	hosts, err := loadHosts(a.sshConfig)
	a.hosts, a.hostsErr = hosts, ""
	if err != nil {
		a.hostsErr = err.Error()
	}
}

func (a *app) setTunnels(ts []Tunnel) {
	a.tunnels = ts
	for _, t := range ts {
		a.state(t.ID)
	}
}

func (a *app) state(id string) *tunnelState {
	st := a.states[id]
	if st == nil {
		st = &tunnelState{}
		a.states[id] = st
	}
	return st
}

func (a *app) find(id string) (int, bool) {
	i := slices.IndexFunc(a.tunnels, func(t Tunnel) bool { return t.ID == id })
	return i, i >= 0
}

// visible returns the tunnels the sidebar's choice shows.
func (a *app) visible() []Tunnel {
	if a.selected == allID {
		return a.tunnels
	}
	var ts []Tunnel
	for _, t := range a.tunnels {
		if hostID(t.Host) == a.selected {
			ts = append(ts, t)
		}
	}
	return ts
}

// active counts the open tunnels through host, or through any host for "".
func (a *app) active(host string) int {
	n := 0
	for _, t := range a.tunnels {
		if a.state(t.ID).on && (host == "" || t.Host == host) {
			n++
		}
	}
	return n
}

// setOpen opens or closes a tunnel, as its switch or the tray asks.
func (a *app) setOpen(id string, on bool) {
	i, ok := a.find(id)
	if !ok {
		return
	}
	st := a.state(id)
	st.on, st.msg = on, ""
	if !on {
		st.status = Off
		a.run.stop(id)
	} else if err := a.run.start(a.tunnels[i]); err != nil {
		st.on, st.status, st.msg = false, Failed, err.Error()
	} else {
		st.status = Connecting
	}
	a.changed()
}

// ready and exited take the runner's news, on the main thread.
func (a *app) ready(id string) {
	if st := a.states[id]; st != nil && st.on {
		st.status = Connected
		a.changed()
	}
}

// connFailed takes ssh's word that the open tunnel could not reach its
// target, as "channel 2: open failed: connect failed: Connection refused";
// the tunnel stays open, and its row warns.
func (a *app) connFailed(id, line string) {
	if st := a.states[id]; st != nil && st.on {
		_, why, _ := strings.Cut(line, "open failed: ")
		st.msg = "访问目标失败：" + why
		a.changed()
	}
}

func (a *app) exited(id, msg string) {
	if st := a.states[id]; st != nil && st.on {
		st.on, st.status, st.msg = false, Failed, msg
		a.changed()
	}
}

// put adds or replaces a tunnel, reopening it with its new settings if it
// was open.
func (a *app) put(t Tunnel) {
	if i, ok := a.find(t.ID); ok {
		a.tunnels[i] = t
	} else {
		a.tunnels = append(a.tunnels, t)
	}
	if a.state(t.ID).on {
		a.run.stop(t.ID)
		a.setOpen(t.ID, true)
	}
	a.save()
}

func (a *app) remove(id string) {
	a.run.stop(id)
	if i, ok := a.find(id); ok {
		a.tunnels = slices.Delete(a.tunnels, i, i+1)
	}
	delete(a.states, id)
	a.save()
}

func (a *app) save() {
	if a.storePath != "" {
		if err := saveTunnels(a.storePath, a.tunnels); err != nil {
			log.Println("saving tunnels:", err)
		}
	}
	a.changed()
}

func (a *app) changed() {
	if a.onChange != nil {
		a.onChange()
	}
}
