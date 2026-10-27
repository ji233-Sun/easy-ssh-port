package main

import (
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

func TestArgs(t *testing.T) {
	for _, tc := range []struct {
		tunnel  Tunnel
		forward []string
	}{
		{Tunnel{Host: "db", Kind: KindLocal, ListenHost: "127.0.0.1", ListenPort: 15432, TargetHost: "10.0.0.5", TargetPort: 5432}, []string{"-L", "127.0.0.1:15432:10.0.0.5:5432"}},
		{Tunnel{Host: "db", Kind: KindLocal, ListenPort: 15432, TargetHost: "::1", TargetPort: 5432}, []string{"-L", "15432:[::1]:5432"}},
		{Tunnel{Host: "db", Kind: KindRemote, ListenHost: "0.0.0.0", ListenPort: 9000, TargetHost: "127.0.0.1", TargetPort: 3000}, []string{"-R", "0.0.0.0:9000:127.0.0.1:3000"}},
		{Tunnel{Host: "db", Kind: KindDynamic, ListenHost: "127.0.0.1", ListenPort: 1080}, []string{"-D", "127.0.0.1:1080"}},
	} {
		args := tc.tunnel.Args()
		if got := args[len(args)-4:]; !reflect.DeepEqual(got, append(tc.forward, "--", "db")) {
			t.Errorf("%s: args end %q", tc.tunnel.Kind, got)
		}
		if !slices.Contains(args, "LocalCommand=echo "+readyMarker) || !slices.Contains(args, "ExitOnForwardFailure=yes") {
			t.Errorf("%s: args %q", tc.tunnel.Kind, args)
		}
	}
}

func TestValidate(t *testing.T) {
	ok := Tunnel{Host: "db", Kind: KindLocal, ListenPort: 1, TargetHost: "x", TargetPort: 65535}
	if err := ok.Validate(); err != nil {
		t.Error(err)
	}
	for _, bad := range []Tunnel{
		{Kind: KindLocal, ListenPort: 1, TargetHost: "x", TargetPort: 1},
		{Host: "db", Kind: KindLocal, ListenPort: 0, TargetHost: "x", TargetPort: 1},
		{Host: "db", Kind: KindLocal, ListenPort: 1, TargetHost: "", TargetPort: 1},
		{Host: "db", Kind: KindRemote, ListenPort: 1, TargetHost: "x", TargetPort: 70000},
	} {
		if bad.Validate() == nil {
			t.Errorf("%+v is valid", bad)
		}
	}
	if err := (Tunnel{Host: "db", Kind: KindDynamic, ListenPort: 1080}).Validate(); err != nil {
		t.Error(err)
	}
}

func TestStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tunnels.json")
	if ts, err := loadTunnels(path); err != nil || ts != nil {
		t.Fatalf("before saving: %v, %v", ts, err)
	}
	want := []Tunnel{postgres, {ID: "s", Name: "SOCKS", Host: "db", Kind: KindDynamic, ListenPort: 1080}}
	if err := saveTunnels(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := loadTunnels(path)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("loaded %+v, %v", got, err)
	}
}

func TestTail(t *testing.T) {
	var tl tail
	tl.Write([]byte("Warning: Permanently added.\r\nPermission denied (publickey).\r\n"))
	if got := tl.lastLine(); got != "Permission denied (publickey)." {
		t.Errorf("lastLine() = %q", got)
	}
	for range 1000 {
		tl.Write([]byte("channel 3: open failed\n"))
	}
	if len(tl.b) > tailSize {
		t.Errorf("tail keeps %d bytes", len(tl.b))
	}
}

func TestTailLines(t *testing.T) {
	var lines []string
	tl := tail{onLine: func(l string) { lines = append(lines, l) }}
	// ssh's writes need not end at line ends.
	for _, s := range []string{"channel 1: open ", "failed: connect failed: Connection refused\r\n\nchannel 2", ": open failed\n"} {
		tl.Write([]byte(s))
	}
	want := []string{"channel 1: open failed: connect failed: Connection refused", "channel 2: open failed"}
	if !reflect.DeepEqual(lines, want) {
		t.Errorf("lines %q", lines)
	}
}
