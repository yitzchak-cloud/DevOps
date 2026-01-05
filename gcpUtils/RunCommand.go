package gcpUtils

import (
	"bytes"
	"os/exec"

	"github.com/rs/zerolog"
)


func RunCommand(log *zerolog.Logger, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args... )
	log.Debug().Strs("args", args).Msgf("⚙️ Executing command: %s", name)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	if err != nil {
		log.Error().
			Err(err).
			Str("output", out.String()).
			Str("command", name).
			Msg("❌ Command execution failed")
	}

	return out.String(), err
}

