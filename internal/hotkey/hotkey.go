// Package hotkey registers the KDE/Wayland global shortcut through XDG Portal.
package hotkey

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/godbus/dbus/v5"
)

const (
	portalBus  = "org.freedesktop.portal.Desktop"
	portalPath = "/org/freedesktop/portal/desktop"
)

func Run() error {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return err
	}
	defer conn.Close()
	signals := make(chan *dbus.Signal, 16)
	conn.Signal(signals)
	defer conn.RemoveSignal(signals)
	for _, rule := range []string{
		"type='signal',interface='org.freedesktop.portal.Request',member='Response'",
		"type='signal',interface='org.freedesktop.portal.GlobalShortcuts',member='Activated'",
	} {
		if call := conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule); call.Err != nil {
			return call.Err
		}
	}
	portal := conn.Object(portalBus, dbus.ObjectPath(portalPath))
	var request dbus.ObjectPath
	options := map[string]dbus.Variant{"handle_token": dbus.MakeVariant("promptpatch_create"), "session_handle_token": dbus.MakeVariant("promptpatch_session")}
	if err := portal.Call("org.freedesktop.portal.GlobalShortcuts.CreateSession", 0, options).Store(&request); err != nil {
		return err
	}
	created, err := waitResponse(signals, request)
	if err != nil {
		return err
	}
	session, ok := created["session_handle"].Value().(string)
	if !ok {
		return fmt.Errorf("portal did not return a session")
	}
	shortcut := []struct {
		ID      string
		Options map[string]dbus.Variant
	}{{"open", map[string]dbus.Variant{"description": dbus.MakeVariant("Open Promptcheck"), "preferred_trigger": dbus.MakeVariant("CTRL+g")}}}
	options = map[string]dbus.Variant{"handle_token": dbus.MakeVariant("promptpatch_bind")}
	if err := portal.Call("org.freedesktop.portal.GlobalShortcuts.BindShortcuts", 0, dbus.ObjectPath(session), shortcut, "", options).Store(&request); err != nil {
		return err
	}
	if _, err := waitResponse(signals, request); err != nil {
		return err
	}
	for signal := range signals {
		if signal.Name != "org.freedesktop.portal.GlobalShortcuts.Activated" || len(signal.Body) < 2 {
			continue
		}
		if id, _ := signal.Body[1].(string); id == "open" {
			_ = openGUI()
		}
	}
	return nil
}

func waitResponse(signals <-chan *dbus.Signal, request dbus.ObjectPath) (map[string]dbus.Variant, error) {
	for signal := range signals {
		if signal.Name != "org.freedesktop.portal.Request.Response" || signal.Path != request || len(signal.Body) != 2 {
			continue
		}
		response, _ := signal.Body[0].(uint32)
		if response != 0 {
			return nil, fmt.Errorf("portal request was cancelled")
		}
		results, _ := signal.Body[1].(map[string]dbus.Variant)
		return results, nil
	}
	return nil, fmt.Errorf("portal disconnected")
}

func openGUI() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	return exec.Command(executable, "gui").Start()
}

func Install() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	directory := filepath.Join(home, ".config", "systemd", "user")
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	service := "[Unit]\nDescription=Promptcheck global shortcut\n\n[Service]\nExecStart=" + executable + " hotkey\nRestart=on-failure\n\n[Install]\nWantedBy=default.target\n"
	path := filepath.Join(directory, "promptcheck-hotkey.service")
	if err := os.WriteFile(path, []byte(service), 0600); err != nil {
		return err
	}
	if err := exec.Command("systemctl", "--user", "daemon-reload").Run(); err != nil {
		return err
	}
	return exec.Command("systemctl", "--user", "enable", "--now", "promptcheck-hotkey").Run()
}
