package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadHosts(t *testing.T) {
	dir := t.TempDir()
	write := func(name, s string) string {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	write("conf.d/work", `
Host jump
  HostName jump.work.example
  User ops
`)
	cfg := write("config", `
# A comment.
Host web web-alias
  HostName=web.example.com
  User   deploy
  Port 2222
  User ignored

Include conf.d/*

Host *.internal !bastion
  User nobody

Match host web
  Port 9999

Host "quoted"
  HostName q.example.com

Host *
  ServerAliveInterval 60
`)
	hosts, err := loadHosts(cfg)
	if err != nil {
		t.Fatal(err)
	}
	want := []Host{
		{Alias: "web", HostName: "web.example.com", User: "deploy", Port: "2222"},
		{Alias: "web-alias", HostName: "web.example.com", User: "deploy", Port: "2222"},
		{Alias: "jump", HostName: "jump.work.example", User: "ops"},
		{Alias: "quoted", HostName: "q.example.com"},
	}
	if !reflect.DeepEqual(hosts, want) {
		t.Errorf("hosts\n got %+v\nwant %+v", hosts, want)
	}
	if got := hosts[0].Address(); got != "deploy@web.example.com:2222" {
		t.Errorf("Address() = %q", got)
	}
}

func TestLoadHostsMissing(t *testing.T) {
	if _, err := loadHosts(filepath.Join(t.TempDir(), "none")); err == nil {
		t.Error("no error for a missing config")
	}
}
