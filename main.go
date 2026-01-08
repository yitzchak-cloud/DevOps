package main
//ADDED COMMENT
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
	ImageName       = "wiki"
	RepoName        = "wiki-registry"
	ProjectID       = "sky-geo-dig-dev-t-cant-1"
	Region          = "me-west1"
	DockerNamespace = "myusername"
	TerraformDir    = "."
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
		Tag:       "latest",
		ProjectID: ProjectID,
		Region:    Region,
		RepoName:  RepoName,
	}

	opts = tfUtils.TerraformOptions{
        ProjectID:       ProjectID,
        TerraformDir:    TerraformDir,
        VarFile:         VarFile,
        BackendVarsFile: BackendVarsFile,
        Destroy:         false, // שנה ל-true אם אתה רוצה למחוק
    }
)




func main() {

	log = logger.InitLogger(true)
	go startWebServer()
	
	gcpUtils.RunGCPCheck(&log, ProjectID)

	// dockerUtils.FullBuildTagPushWithRegistry(
	// 	&log,
	// 	".",
	// 	"myapp:latest",
	// 	DockerConfig,
	// ) 

	// dockerUtils.FullBuildTagPushWithRegistry(
	// 	&log,
	// 	".",
	// 	"wiki:latest",
	// 	gcpConfig,
	// )

	
    tfUtils.RunTerraformWorkflow(&log, opts)
	
	select {}
}
