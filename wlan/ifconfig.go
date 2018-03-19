package wlan

import (
	"os/exec"
)

func ifdown(iface string) error {
	return exec.Command("/sbin/ifconfig", iface, "down").Run()
}

func ifup(iface string) error {
	return exec.Command("/sbin/ifconfig", iface, "up").Run()
}
