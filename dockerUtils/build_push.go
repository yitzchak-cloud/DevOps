package dockerUtils

import (
	"fmt"
	"errors"
	"github.com/rs/zerolog"
)

type RegistryType string

const (
	RegistryDocker RegistryType = "docker"
	RegistryGCP    RegistryType = "gcp"
)


type PushConfig struct {
	Registry RegistryType

	// Docker Hub
	DockerNamespace string // username או organization
	ImageName       string // myapp
	Tag             string // latest

	// GCP only
	ProjectID string
	Region    string
	RepoName  string
}



// DockerBuild builds a Docker image from the specified Dockerfile context.
// buildPath is the context path (where the Dockerfile is located, usually ".").
// tagName is the name and tag for the resulting image (e.g., "myrepo/myapp:latest").
func DockerBuild(log *zerolog.Logger, buildPath, tagName string) error {
	log.Info().Str("path", buildPath).Str("tag", tagName).Msg("🔨 Building Docker image...")
	
	args := []string{"build", "-t", tagName, buildPath}
	
	if err := RunCommand(log, "docker", args...); err != nil {
		log.Error().Str("tag", tagName).Msg("❌ Docker build failed")
		return err
	}
	
	log.Info().Str("tag", tagName).Msg("✅ Docker image built successfully")
	return nil
}

// DockerTag tags an existing image with an additional name/tag.
func DockerTag(log *zerolog.Logger, sourceTag, targetTag string) error {
	log.Info().Str("source", sourceTag).Str("target", targetTag).Msg("🏷️ Tagging Docker image...")
	
	args := []string{"tag", sourceTag, targetTag}
	
	if err := RunCommand(log, "docker", args...); err != nil {
		log.Error().Str("source", sourceTag).Str("target", targetTag).Msg("❌ Docker tag failed")
		return err
	}
	
	log.Info().Str("target", targetTag).Msg("✅ Docker image tagged successfully")
	return nil
}

// DockerPush pushes a tagged Docker image to a remote registry.
// imageTag is the name and tag of the image to push (e.g., "myrepo/myapp:latest").
func DockerPush(log *zerolog.Logger, imageTag string) error {
	log.Info().Str("tag", imageTag).Msg("⬆️ Pushing Docker image to registry...")
	
	args := []string{"push", imageTag}
	
	if err := RunCommand(log, "docker", args...); err != nil {
		log.Error().Str("tag", imageTag).Msg("❌ Docker push failed")
		return err
	}
	
	log.Info().Str("tag", imageTag).Msg("✅ Docker image pushed successfully")
	return nil
}

func buildRemoteTag(log *zerolog.Logger, cfg PushConfig) (string, error) {
	log.Debug().
		Str("registry", string(cfg.Registry)).
		Msg("🔧 Building remote image tag")

	switch cfg.Registry {

	case RegistryDocker:
		if cfg.DockerNamespace == "" {
			log.Error().
				Msg("❌ Docker namespace (username/org) is required")
			return "", errors.New("missing Docker namespace")
		}

		tag := fmt.Sprintf(
			"%s/%s:%s",
			cfg.DockerNamespace,
			cfg.ImageName,
			cfg.Tag,
		)

		log.Info().
			Str("remoteTag", tag).
			Msg("📦 Using Docker Hub image tag")

		return tag, nil

	case RegistryGCP:
		if cfg.ProjectID == "" || cfg.Region == "" || cfg.RepoName == "" {
			log.Error().
				Str("project", cfg.ProjectID).
				Str("region", cfg.Region).
				Str("repo", cfg.RepoName).
				Msg("❌ Missing GCP registry parameters")
			return "", errors.New("missing GCP registry parameters")
		}

		tag := fmt.Sprintf(
			"%s-docker.pkg.dev/%s/%s/%s:%s",
			cfg.Region,
			cfg.ProjectID,
			cfg.RepoName,
			cfg.ImageName,
			cfg.Tag,
		)

		log.Info().
			Str("remoteTag", tag).
			Msg("📦 Using GCP Artifact Registry image tag")

		return tag, nil

	default:
		log.Error().
			Str("registry", string(cfg.Registry)).
			Msg("❌ Unsupported registry type")
		return "", errors.New("unsupported registry type")
	}
}


func ensureGCPAuth(log *zerolog.Logger, region string) error {
	host := fmt.Sprintf("%s-docker.pkg.dev", region)

	log.Info().
		Str("host", host).
		Msg("🔐 Configuring Docker authentication for GCP Artifact Registry")

	if err := RunCommand(
		log,
		"gcloud",
		"auth",
		"configure-docker",
		host,
		"--quiet",
	); err != nil {

		log.Error().
			Err(err).
			Str("host", host).
			Msg("❌ Failed to configure Docker auth for GCP")

		return err
	}

	log.Info().
		Str("host", host).
		Msg("✅ Docker authenticated with GCP Artifact Registry")

	return nil
}


// FullBuildTagPush performs the build, tag, and push sequence.
func FullBuildTagPush(log *zerolog.Logger, buildPath, localTag, remoteTag string) error {
	log.Info().Msg("🚀 Starting Full Docker Build, Tag, and Push process...")
	
	RunDockerCheck(log)

	// 1. Build
	if err := DockerBuild(log, buildPath, localTag); err != nil {
		log.Error().Err(err).Msg("❌ Docker build failed")
		return err
	}

	// 2. Tag (optional, only if remoteTag is different from localTag)
	if localTag != remoteTag {
		if err := DockerTag(log, localTag, remoteTag); err != nil {
			log.Error().Err(err).Msg("❌ Docker tag failed")
			return err
		}
	} else {
		log.Debug().Msg("Skipping explicit tag step as localTag equals remoteTag")
	}

	// 3. Push
	if err := DockerPush(log, remoteTag); err != nil {
		log.Error().Err(err).Msg("❌ Docker push failed")
		return  err
	}

	log.Info().Msg("✨ Full Docker process completed successfully.")
	return nil
}


func FullBuildTagPushWithRegistry(
	log *zerolog.Logger,
	buildPath string,
	localTag string,
	cfg PushConfig,
) error {

	log.Info().
		Str("buildPath", buildPath).
		Str("localTag", localTag).
		Str("registry", string(cfg.Registry)).
		Msg("🚀 Starting Docker build/tag/push with registry config")

	remoteTag, err := buildRemoteTag(log, cfg)
	if err != nil {
		log.Error().Err(err).Msg("❌ Failed to build remote tag")
		return err
	}

	// חיבור ל־GCP אם צריך
	if cfg.Registry == RegistryGCP {
		if err := ensureGCPAuth(log, cfg.Region); err != nil {
			return err
		}
	}

	// שימוש מלא בקוד הקיים שלך
	if err := FullBuildTagPush(log, buildPath, localTag, remoteTag); err != nil {
		log.Error().
			Err(err).
			Msg("❌ Full Docker process failed")
		return err
	}

	log.Info().
		Msg("✨ Docker build/tag/push completed successfully")

	return nil
}
