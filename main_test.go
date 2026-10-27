package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

// fakeRunner records what the app asks of ssh.
type fakeRunner struct{ started, stopped []string }

func (f *fakeRunner) start(t Tunnel) error { f.started = append(f.started, t.ID); return nil }
func (f *fakeRunner) stop(id string)       { f.stopped = append(f.stopped, id) }

func testApp(t *testing.T) (*app, *fakeRunner) {
	t.Helper()
	cfg := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(cfg, []byte("Host db\n  HostName db.example.com\n  User ji\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := &fakeRunner{}
	return newApp(r, cfg), r
}

var postgres = Tunnel{ID: "pg", Name: "Postgres", Host: "db", Kind: KindLocal,
	ListenHost: "127.0.0.1", ListenPort: 15432, TargetHost: "127.0.0.1", TargetPort: 5432}

func TestNewTunnel(t *testing.T) {
	a, _ := testApp(t)
	a.storePath = filepath.Join(t.TempDir(), "tunnels.json")
	tt := ui.NewTester(a.view, 760, 620)
	if !tt.HasText("还没有隧道") || !tt.HasText("db") {
		t.Fatalf("texts %q of an empty app", tt.Texts())
	}
	if err := tt.Click("新建隧道"); err != nil {
		t.Fatal(err)
	}
	if !a.editing || a.draft.tunnel.Host != "db" || !tt.HasText("本机监听地址") || !tt.HasText("远端目标地址") {
		t.Fatalf("editing %v, draft %+v, texts %q", a.editing, a.draft, tt.Texts())
	}
	tt.Type("Postgres") // the name field has the focus
	if err := tt.Click("本机监听地址端口"); err != nil {
		t.Fatal(err)
	}
	tt.Type("15432")
	if err := tt.Click("远端目标地址端口"); err != nil {
		t.Fatal(err)
	}
	tt.Type("5432")
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	if a.editing || len(a.tunnels) != 1 {
		t.Fatalf("editing %v, tunnels %+v after saving", a.editing, a.tunnels)
	}
	got := a.tunnels[0]
	if got.Name != "Postgres" || got.Summary() != "本机 127.0.0.1:15432 → 远端 127.0.0.1:5432" || got.ID == "" {
		t.Errorf("saved %+v", got)
	}
	if saved, err := loadTunnels(a.storePath); err != nil || len(saved) != 1 {
		t.Errorf("stored %+v, %v", saved, err)
	}
	if !tt.HasText("本机 127.0.0.1:15432 → 远端 127.0.0.1:5432") {
		t.Errorf("texts %q after saving", tt.Texts())
	}
}

func TestPreset(t *testing.T) {
	a, _ := testApp(t)
	tt := ui.NewTester(a.view, 760, 620)
	a.edit(Tunnel{})
	tt.Frame()
	if err := tt.Click("动态代理：SOCKS5 1080"); err != nil {
		t.Fatal(err)
	}
	got := a.draft.result()
	if got.Kind != KindDynamic || got.ListenPort != 1080 || got.Host != "db" || got.Name != "SOCKS5 代理" {
		t.Errorf("draft %+v after the SOCKS5 preset", got)
	}
	if tt.HasText("远端目标地址") {
		t.Error("a dynamic tunnel shows a target")
	}
	// A name the user typed survives another preset.
	a.draft.tunnel.Name = "Office MySQL"
	tt.Frame()
	if err := tt.Click("本地转发：MySQL 3306 → 本机 3307"); err != nil {
		t.Fatal(err)
	}
	got = a.draft.result()
	if got.Kind != KindLocal || got.ListenPort != 3307 || got.TargetPort != 3306 || got.Name != "Office MySQL" {
		t.Errorf("draft %+v after the MySQL preset", got)
	}
}

func TestSwitch(t *testing.T) {
	a, r := testApp(t)
	a.setTunnels([]Tunnel{postgres})
	tt := ui.NewTester(a.view, 760, 620)
	if err := tt.Click("启用 Postgres"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(r.started, []string{"pg"}) || a.state("pg").status != Connecting {
		t.Fatalf("started %v, state %+v", r.started, *a.state("pg"))
	}
	if a.editing {
		t.Error("the switch opened the editor")
	}
	a.ready("pg")
	if a.state("pg").status != Connected || a.active("db") != 1 {
		t.Errorf("state %+v once ready", *a.state("pg"))
	}
	tt.Frame()
	if err := tt.Click("启用 Postgres"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(r.stopped, []string{"pg"}) || a.state("pg").on {
		t.Errorf("stopped %v, state %+v", r.stopped, *a.state("pg"))
	}
}

func TestFailure(t *testing.T) {
	a, _ := testApp(t)
	a.setTunnels([]Tunnel{postgres})
	a.setOpen("pg", true)
	a.exited("pg", "Permission denied (publickey).")
	tt := ui.NewTester(a.view, 760, 620)
	if st := a.state("pg"); st.on || st.status != Failed {
		t.Errorf("state %+v after ssh exited", *st)
	}
	if !tt.HasText("Permission denied (publickey).") {
		t.Errorf("texts %q", tt.Texts())
	}
	// News of a tunnel the user closed meanwhile changes nothing.
	a.ready("pg")
	if a.state("pg").status != Failed {
		t.Errorf("state %+v after a late ready", *a.state("pg"))
	}
}

func TestConnFailed(t *testing.T) {
	a, _ := testApp(t)
	a.setTunnels([]Tunnel{postgres})
	a.setOpen("pg", true)
	a.ready("pg")
	a.connFailed("pg", "channel 1: open failed: connect failed: Connection refused")
	tt := ui.NewTester(a.view, 760, 620)
	if st := a.state("pg"); !st.on || st.status != Connected {
		t.Errorf("state %+v: the tunnel itself stays open", *st)
	}
	if !tt.HasText("访问目标失败：connect failed: Connection refused") {
		t.Errorf("texts %q", tt.Texts())
	}
	// Reopening clears the warning.
	a.setOpen("pg", false)
	a.setOpen("pg", true)
	if a.state("pg").msg != "" {
		t.Errorf("msg %q after reopening", a.state("pg").msg)
	}
}

func TestEditAndDelete(t *testing.T) {
	a, r := testApp(t)
	a.setTunnels([]Tunnel{postgres})
	tt := ui.NewTester(a.view, 760, 620)
	if err := tt.Click("Postgres"); err != nil {
		t.Fatal(err)
	}
	if !a.editing || a.draft.tunnel.ID != "pg" || a.draft.listenPort != "15432" || !tt.HasText("编辑隧道") {
		t.Fatalf("editing %v, draft %+v", a.editing, a.draft)
	}
	if err := tt.Click("删除"); err != nil {
		t.Fatal(err)
	}
	if a.editing || len(a.tunnels) != 0 || !slices.Contains(r.stopped, "pg") {
		t.Errorf("editing %v, tunnels %+v, stopped %v", a.editing, a.tunnels, r.stopped)
	}
}
