package daemon

import (
	"context"

	"github.com/bikeos/bosd/audio"
)

func (d *daemon) startAudio() error {
	bootmp3 := d.s.ConfigPath("boot.mp3")
	if err := audio.Play(context.TODO(), bootmp3); err != nil {
		return err
	}
	// TODO: listen/poll for state changes
	return nil
}
