package audio

import (
	"context"
	"os/exec"
)

// Play plays a sound to completion.
func Play(ctx context.Context, path string) error {
	// TODO: get error data from stderr
	return exec.CommandContext(ctx, "/usr/bin/mpg321", "-q", path).Run()
}

// Say converts text to speech and plays to completion.
func Say(ctx context.Context, txt string) error {
	// TODO: get error data from stderr
	return exec.CommandContext(ctx, "/usr/bin/flite", "-voice", "awb", "-t", txt).Run()
}
