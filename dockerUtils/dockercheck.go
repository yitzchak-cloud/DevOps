package dockerUtils

import (
	"time"

	"github.com/rs/zerolog"
)


// IsDockerDaemonReady checks if the Docker daemon is responsive by running `docker info`.
func IsDockerDaemonReady(log *zerolog.Logger) bool {
	return RunCommand(log, "docker", "info") == nil
}

// StartDockerDesktop attempts to launch Docker Desktop on Windows.
func StartDockerDesktop(log *zerolog.Logger) error {
	log.Info().Msg("--- 🐳 Docker Daemon is not ready. Attempting to launch Docker Desktop...")
	dockerPath := "C:\\Program Files\\Docker\\Docker\\Docker Desktop.exe"
	err := RunCommand(log, "cmd", "/C", "start", "", dockerPath)

	if err != nil {
		log.Error().Err(err).Msg("🚨 Ensure the path to Docker Desktop.exe is correct or Docker is installed.")
		return err
	}

	log.Info().Msg("+++ ✅ Docker Desktop launch command issued successfully. Please check your screen.")
	return nil
}

// WaitForDaemonReady waits for the Docker daemon to become ready up to a given timeout.
func WaitForDaemonReady(log *zerolog.Logger, timeout time.Duration) bool {
	log.Info().Dur("timeout", timeout).Msg(">>> ⏳ Waiting for Docker Daemon to become ready...")
	startTime := time.Now()

	for time.Since(startTime) < timeout {
		if IsDockerDaemonReady(log) {
			log.Info().Msg("🎉 Docker Daemon is now responsive!")
			return true
		}
		time.Sleep(2 * time.Second)
		log.Debug().Msg("Retrying 'docker info' check...")
	}
	return false
}

// RunDockerCheck contains the main logic for checking and starting Docker.
func RunDockerCheck(log *zerolog.Logger) {
	log.Info().Msg(" 🔍 Checking Docker Daemon Status on Windows...")

	if IsDockerDaemonReady(log) {
		log.Info().Msg("✅ Docker Daemon is running and ready. Proceeding with the build.")
		return
	}

	if err := StartDockerDesktop(log); err != nil {
		log.Error().Msg("❌ Failed to initiate Docker Desktop launch. Stopping build process.")
		return
	}

	if WaitForDaemonReady(log, 60*time.Second) {
		log.Info().Msg("✨ Docker Daemon is ready after launch and wait. Proceeding with the build.")
	} else {
		log.Error().Msg("❌ 🛑 Timeout reached. Docker Daemon did not become ready after launch. Stopping build.")
	}
}
