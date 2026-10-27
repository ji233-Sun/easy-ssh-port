package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Kind is the direction a tunnel forwards in, as ssh's -L, -R and -D.
type Kind string

const (
	// KindLocal listens on this Mac and reaches a target from the server
	// (-L).
	KindLocal Kind = "local"
	// KindRemote listens on the server and reaches a target from this Mac
	// (-R).
	KindRemote Kind = "remote"
	// KindDynamic is a SOCKS5 proxy on this Mac whose connections leave
	// from the server (-D).
	KindDynamic Kind = "dynamic"
)

// kinds lists the kinds in the order the editor shows them.
var kinds = []Kind{KindLocal, KindRemote, KindDynamic}

// Tunnel is a port forward through one of the ssh config's hosts: it
// listens on one side and forwards what connects to a target reached from
// the other side. A dynamic tunnel has no target.
type Tunnel struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Host       string `json:"host"` // an alias of the ssh config
	Kind       Kind   `json:"kind"`
	ListenHost string `json:"listenHost,omitempty"` // "" is ssh's default, the loopback
	ListenPort int    `json:"listenPort"`
	TargetHost string `json:"targetHost,omitempty"`
	TargetPort int    `json:"targetPort,omitempty"`
}

// readyMarker is what ssh prints, through LocalCommand, once it has
// connected and set up the forward.
const readyMarker = "easy-ssh-port:ready"

// Args returns the arguments of the ssh command that keeps the tunnel open.
// The host's own settings (keys, ProxyJump, ...) come from the ssh config;
// the options here make ssh fail rather than prompt or half-work, and keep
// the process to this tunnel alone.
func (t Tunnel) Args() []string {
	spec := strconv.Itoa(t.ListenPort)
	if t.ListenHost != "" {
		spec = bracket(t.ListenHost) + ":" + spec
	}
	if t.Kind != KindDynamic {
		spec += ":" + bracket(t.TargetHost) + ":" + strconv.Itoa(t.TargetPort)
	}
	flag := map[Kind]string{KindLocal: "-L", KindRemote: "-R", KindDynamic: "-D"}[t.Kind]
	return []string{
		"-N",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		"-o", "ExitOnForwardFailure=yes",
		"-o", "ServerAliveInterval=30",
		"-o", "ServerAliveCountMax=3",
		"-o", "ControlMaster=no",
		"-o", "ControlPath=none",
		"-o", "PermitLocalCommand=yes",
		"-o", "LocalCommand=echo " + readyMarker,
		flag, spec,
		"--", t.Host,
	}
}

// bracket writes an IPv6 address as ssh's forward specs take it.
func bracket(host string) string {
	if strings.Contains(host, ":") {
		return "[" + host + "]"
	}
	return host
}

func hostPort(host string, port int) string {
	if host == "" {
		host = "localhost"
	}
	return bracket(host) + ":" + strconv.Itoa(port)
}

// Summary describes where the tunnel listens and what it reaches.
func (t Tunnel) Summary() string {
	listen := hostPort(t.ListenHost, t.ListenPort)
	target := hostPort(t.TargetHost, t.TargetPort)
	switch t.Kind {
	case KindRemote:
		return "服务器 " + listen + " → 本机 " + target
	case KindDynamic:
		return "SOCKS5 " + listen
	default:
		return "本机 " + listen + " → 远端 " + target
	}
}

// Validate reports what keeps the tunnel from working, if anything.
func (t Tunnel) Validate() error {
	valid := func(p int) bool { return p > 0 && p <= 65535 }
	switch {
	case t.Host == "":
		return errors.New("请选择 SSH 主机")
	case !valid(t.ListenPort):
		return errors.New("监听端口需在 1–65535 之间")
	case t.Kind != KindDynamic && t.TargetHost == "":
		return errors.New("请填写目标地址")
	case t.Kind != KindDynamic && !valid(t.TargetPort):
		return errors.New("目标端口需在 1–65535 之间")
	}
	return nil
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// loadTunnels reads the tunnels saved at path; there are none before the
// first save.
func loadTunnels(path string) ([]Tunnel, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var ts []Tunnel
	return ts, json.Unmarshal(data, &ts)
}

// saveTunnels writes the tunnels to path, replacing the file at once so a
// crash never leaves half of it.
func saveTunnels(path string, ts []Tunnel) error {
	data, err := json.MarshalIndent(ts, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tunnels-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
