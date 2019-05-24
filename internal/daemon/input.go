package daemon

import (
	"context"
	"io"
	"strings"

	"github.com/gvalkov/golang-evdev"
	log "github.com/sirupsen/logrus"
)

type InputEvent int

const (
	InputUp = iota
	InputDown
	InputLeft
	InputRight
)

func NewInputChannel(ctx context.Context) (<-chan InputEvent, error) {
	dev, err := openGamepad()
	if err != nil {
		return nil, err
	}
	ch, donec := make(chan InputEvent, 1), make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			dev.Release()
		case <-donec:
		}
	}()
	go func() {
		defer func() {
			dev.Release()
			close(donec)
			close(ch)
		}()
		for {
			ev, err := dev.ReadOne()
			if err != nil {
				log.Errorf("input: %v", err)
				break
			}
			if ev.Type != evdev.EV_ABS {
				continue
			}
			var iev InputEvent
			if ev.Code == evdev.ABS_Y {
				if ev.Value == 0 {
					iev = InputUp
				} else if ev.Value == 255 {
					iev = InputDown
				} else {
					continue
				}
			} else if ev.Code == evdev.ABS_X {
				if ev.Value == 0 {
					iev = InputLeft
				} else if ev.Value == 255 {
					iev = InputRight
				} else {
					continue
				}
			} else {
				continue
			}
			select {
			case ch <- iev:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}

func openGamepad() (*evdev.InputDevice, error) {
	devices, _ := evdev.ListInputDevices()
	for _, dev := range devices {
		if strings.TrimSpace(dev.Name) == "usb gamepad" {
			return evdev.Open(dev.Fn)
		}
	}
	return nil, io.EOF
}
