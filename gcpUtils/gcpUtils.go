package gcpUtils

import (
	"strings"
	"time"

	"github.com/rs/zerolog"
)


func IsGCPAuthenticated(log *zerolog.Logger) bool {
    // שלב 1: בדוק שיש חשבון פעיל
    out, err := RunCommand(
        log,
        "gcloud",
        "auth",
        "list",
        "--filter=status:ACTIVE",
        "--format=value(account)",
    )
    if err != nil || strings.TrimSpace(out) == "" {
        return false
    }

    // שלב 2: בדיקה אם ה-token פעיל (נסה להוציא access token)
    _, err = RunCommand(
        log,
        "gcloud",
        "auth",
        "print-access-token",
    )
    if err != nil {
        log.Error().Err(err).Msg("GCP token invalid or expired")
        return false
    }

    return true
}


func GCPLogin(log *zerolog.Logger) error {
	log.Info().Msg("🔐 Not authenticated to GCP. Running gcloud auth login...")
	_, err := RunCommand(log, "gcloud", "auth", "login")
	return err
}

func IsGCPApplicationDefaultAuthenticated(log *zerolog.Logger) bool {
    // נסה להוציא את ה-ADC token
    _, err := RunCommand(
        log,
        "gcloud",
        "auth",
        "application-default",
        "print-access-token",
    )
    if err != nil {
        log.Warn().Err(err).Msg("Application Default Credentials invalid or missing")
        return false
    }
    return true
}

func GCPApplicationDefaultLogin(log *zerolog.Logger) error {
    log.Info().Msg("🔐 Not authenticated for Application Default Credentials. Running gcloud auth application-default login...")
    _, err := RunCommand(log, "gcloud", "auth", "application-default", "login")
    return err
}


func GetCurrentProject(log *zerolog.Logger) (string, error) {
	out, err := RunCommand(
		log,
		"gcloud",
		"config",
		"get-value",
		"project",
	)

	if err != nil {
		return "", err
	}

	return strings.TrimSpace(out), nil
}

func SetProject(log *zerolog.Logger, projectID string) error {
	log.Info().Str("project", projectID).Msg("🔄 Switching GCP project...")
	_, err := RunCommand(
		log,
		"gcloud",
		"config",
		"set",
		"project",
		projectID,
	)
	return err
}

func WaitForGCPAuth(log *zerolog.Logger, timeout time.Duration) bool {
	start := time.Now()

	for time.Since(start) < timeout {
		if IsGCPAuthenticated(log) {
			return true
		}
		time.Sleep(2 * time.Second)
	}
	return false
}


func RunGCPCheck(log *zerolog.Logger, expectedProject string) {
	log.Info().Msg("🔍 Checking GCP authentication and project...")

	// 1️⃣ Auth check
	if !IsGCPAuthenticated(log) {
		log.Warn().Msg("⚠️ Not authenticated to GCP")

		if err := GCPLogin(log); err != nil {
			log.Fatal().Err(err).Msg("❌ Failed to authenticate to GCP")
			return
		}

		if !WaitForGCPAuth(log, 60*time.Second) {
			log.Fatal().Msg("❌ GCP authentication timeout")
			return
		}
	}

	log.Info().Msg("✅ Authenticated to GCP")

	// 2️⃣ Project check
	currentProject, err := GetCurrentProject(log)
	if err != nil {
		log.Fatal().Err(err).Msg("❌ Failed to get current GCP project")
		return
	}

	log.Info().
		Str("current", currentProject).
		Str("expected", expectedProject).
		Msg("📌 Checking active project")

	if currentProject != expectedProject {
		log.Warn().Msg("⚠️ Active project does not match expected project")

		if err := SetProject(log, expectedProject); err != nil {
			log.Fatal().Err(err).Msg("❌ Failed to switch GCP project")
			return
		}

		log.Info().Msg("✅ Project switched successfully")
	} else {
		log.Info().Msg("✅ Correct GCP project already active")
	}

	log.Info().Msg("🚀 GCP environment ready – continuing execution")

	// 3️⃣ ADC check for Terraform / SDK
	if !IsGCPApplicationDefaultAuthenticated(log) {
		log.Warn().Msg("⚠️ ADC not authenticated")

		if err := GCPApplicationDefaultLogin(log); err != nil {
			log.Fatal().Err(err).Msg("❌ Failed to authenticate ADC")
			return
		}

		// אפשר גם כאן לחכות עד שהטוקן פעיל
		start := time.Now()
		timeout := 60 * time.Second
		for time.Since(start) < timeout {
			if IsGCPApplicationDefaultAuthenticated(log) {
				break
			}
			time.Sleep(2 * time.Second)
		}
	}
	log.Info().Msg("✅ Application Default Credentials ready")

}
