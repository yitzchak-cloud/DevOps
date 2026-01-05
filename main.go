package main

import (
	"DevOps/logger" 
	"DevOps/dockerUtils"
	"DevOps/gcpUtils"
	"DevOps/tfUtils"
	"github.com/rs/zerolog"
)

// הגדרת משתנה הלוגר הראשי - משותף לכל הקבצים באמצעות הגדרתו כאן
var log zerolog.Logger

const (
	buildPath       = "."
	ImageName       = "myapp"
	RepoName        = "wi-registry"
	projectId       = "sky-geo-dig-dev-u-onbd-1"
	Region          = "me-west1"
	DockerNamespace = "myusername"
	TerraformDir    = "C:\\Users\\YDamen\\Documents\\testTerraform"
	VarFile 	   = "variables.tfvars"
	BackendVarsFile = "backend.tfvars"
)

var (
	DockerConfig = dockerUtils.PushConfig{	
		Registry:        dockerUtils.RegistryDocker,
		DockerNamespace: DockerNamespace,
		ImageName:       ImageName,
		Tag:             "latest",
	}

	gcpConfig = dockerUtils.PushConfig{
		Registry:  dockerUtils.RegistryGCP,
		ImageName: ImageName,
		Tag:       "v1.0.0",
		ProjectID: projectId,
		Region:    Region,
		RepoName:  RepoName,
	}

	opts = tfUtils.TerraformOptions{
        ProjectID:       projectId,
        TerraformDir:    TerraformDir,
        VarFile:         VarFile,
        BackendVarsFile: BackendVarsFile,
        Destroy:         true, // שנה ל-true אם אתה רוצה למחוק
    }
)




func main() {

	log = logger.InitLogger(true)
	go startWebServer()
	
	gcpUtils.RunGCPCheck(&log, projectId)

	// dockerUtils.FullBuildTagPushWithRegistry(
	// 	&log,
	// 	".",
	// 	"myapp:latest",
	// 	DockerConfig,
	// ) 

	// dockerUtils.FullBuildTagPushWithRegistry(
	// 	&log,
	// 	".",
	// 	"myapp:latest",
	// 	gcpConfig,
	// )

    tfUtils.RunTerraformWorkflow(&log, opts)
	
	select {}
}