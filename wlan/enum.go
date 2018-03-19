package wlan

import (
	nl80211 "github.com/mdlayher/wifi"
)

type Device struct {
	iface *nl80211.Interface
}

func (d *Device) Name() string { return d.iface.Name }

func Enumerate() ([]Device, error) {
	c, err := nl80211.New()
	if err != nil {
		return nil, err
	}
	defer c.Close()
	ifaces, err := c.Interfaces()
	if err != nil {
		return nil, err
	}
	ret := []Device{}
	for _, iface := range ifaces {
		ret = append(ret, Device{iface})
	}
	return ret, nil
}
