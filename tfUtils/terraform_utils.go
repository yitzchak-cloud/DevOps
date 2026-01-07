package tfUtils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"


	"DevOps/gcpUtils" // וודא שהנתיב תואם ל-go.mod שלך

	"cloud.google.com/go/storage"
	"github.com/rs/zerolog"
	"google.golang.org/api/iterator"
)

// TFConfig מחזיק את ההגדרות להרצת טראפורם
type TFConfig struct {
    Dir             string
    VarFile         string
    BackendVarsFile string
    Vars            map[string]string 
}

// TerraformOptions מגדיר את כל מה שצריך להרצה
type TerraformOptions struct {
	ProjectID       string
	TerraformDir    string
	VarFile         string
	BackendVarsFile string
	Destroy         bool
}

// ExtractBackendBucket מחלץ את שם ה-bucket מהגדרות ה-backend
func ExtractBackendBucket(log *zerolog.Logger, dir string) string {
    extractor := NewTerraformConfigExtractor(log, dir)
	// // חיפוש bucket מכל המקורות
	// extractor.ExtractVariable("bucket")

	// // חיפוש רק מקבצי .tf
	// extractor.ExtractVariable("bucket", ConfigSourceTfFiles)

	// // חיפוש רק מ-backend files
	// extractor.ExtractVariable("bucket", ConfigSourceBackendFiles)
    return extractor.ExtractVariable("bucket")
}

func ensureGCSBucket(log *zerolog.Logger, projectID, bucketName string) error {
    log.Info().Str("bucket", bucketName).Str("project", projectID).Msg("🧐 Checking remote state bucket...")
    ctx := context.Background()
    client, err := storage.NewClient(ctx)
    if err != nil {
        return fmt.Errorf("❌ failed to create GCP client: %w", err)
    }
    defer client.Close()

    bucket := client.Bucket(bucketName)
    attrs, err := bucket.Attrs(ctx)

    if err == nil {
        // שימוש ב-attrs כדי למנוע את שגיאת הקומפילציה
        // אנחנו בודקים אם ה-Bucket קיים ונגיש לנו
        log.Info().
            Str("bucket", bucketName).
            Uint64("project_number", attrs.ProjectNumber).
            Str("location", attrs.Location).
            Msg("✅ Bucket exists. Checking if it's the right one...")

        // הערה חשובה: ב-GCP, אם השם תפוס ע"י מישהו אחר, 
        // לעיתים קרובות לא תהיה לך גישה אפילו ל-Attrs (תקבל 403).
        // אם הצלחת לקרוא Attrs אבל אתה לא רואה אותו בפרויקט שלך ב-Console,
        // סימן שהוא שייך לפרויקט אחר שבו יש לך הרשאות.
        
        return nil 
    }

    // אם קיבלנו שגיאה, נבדוק אם זה בגלל שהוא לא קיים
    log.Warn().Err(err).Str("bucket", bucketName).Msg("🪣 Bucket not found or not accessible, attempting to create...")

    newAttrs := &storage.BucketAttrs{
        Location: "me-west1",
    }

    if err := bucket.Create(ctx, projectID, newAttrs); err != nil {
        // כאן תקבל שגיאת "Conflict" (409) אם השם תפוס גלובלית
        log.Error().Err(err).Str("bucket", bucketName).Msg("❌ Failed to create GCS bucket")
        return fmt.Errorf("conflict: bucket name '%s' might be taken globally: %w", bucketName, err)
    }

    log.Info().Str("bucket", bucketName).Msg("🎉 Successfully created remote state bucket")
    return nil
}

func createDefaultFiles(log *zerolog.Logger, dir, projectID string) error {
    // יצירת התיקייה במידה ולא קיימת
    if err := os.MkdirAll(dir, 0755); err != nil {
        return fmt.Errorf("failed to create directory: %w", err)
    }

    // 1. backend.tf - עכשיו הוא גנרי ומושך נתונים מהקונפיגורציה
    backendContent := `terraform {
  backend "gcs" {}
}`

    // 2. backend.tfvars - מכיל את הערכים הספציפיים לבאקט
    backendVarsContent := fmt.Sprintf(`bucket = "%s-tfstate"
prefix = "terraform/state"
`, projectID)

    // 3. provider.tf - משתמש במשתנים במקום בערכים קבועים
    providerContent := `provider "google" {
  project = var.project_id
  region  = var.region
}
provider "google-beta" {
  project = var.project_id
  region  = var.region
}`

    // 4. variables.tf - הגדרת המשתנים
    variablesContent := `variable "project_id" {
  type = string
}
variable "region" {
  type    = string
  default = "me-west1"
}`

    // 5. terraform.tfvars - הערכים למשתנים
    tfvarsContent := fmt.Sprintf(`project_id = "%s"
region     = "me-west1"
`, projectID)

    // מפת קבצים לכתיבה נוחה בלולאה
    files := map[string]string{
        "backend.tf":       backendContent,
        "backend.tfvars":   backendVarsContent,
        "provider.tf":      providerContent,
        "variables.tf":     variablesContent,
        "variables.tfvars": tfvarsContent,
        "main.tf":          "# Main resources\n",
    }

    for name, content := range files {
        path := filepath.Join(dir, name)
        if err := os.WriteFile(path, []byte(content), 0644); err != nil {
            return fmt.Errorf("failed to write %s: %w", name, err)
        }
    }

    log.Info().Str("projectID", projectID).Msg("📂 Generated modular Terraform files with tfvars")
    return nil
}

func Init(log *zerolog.Logger, config TFConfig) error {
	log.Info().Str("dir", config.Dir).Msg("🛠️ Initializing Terraform...")

	baseArgs := []string{"init", "-upgrade", "-input=false"}
	if config.BackendVarsFile != "" {
		baseArgs = append(baseArgs, fmt.Sprintf("-backend-config=%s", config.BackendVarsFile))
	}

	_, err := RunTerraform(log, config.Dir, baseArgs...)
	if err == nil {
		return nil
	}

	log.Warn().Msg("⚠️ Standard init failed, trying -reconfigure...")
	reconfigArgs := append(baseArgs, "-reconfigure")
	_, err = RunTerraform(log, config.Dir, reconfigArgs...)
	if err == nil {
		return nil
	}

	log.Warn().Msg("⚠️ Reconfigure failed, trying -migrate-state...")
	migrateArgs := append(baseArgs, "-migrate-state")
	_, err = RunTerraform(log, config.Dir, migrateArgs...)
	return err
}

func Apply(log *zerolog.Logger, config TFConfig) error {
    log.Info().Msg("🚀 Running Terraform Apply...")
    args := []string{"apply", "-auto-approve"}
    
    // הוספת קובץ משתנים אם קיים
    if config.VarFile != "" {
        args = append(args, fmt.Sprintf("-var-file=%s", config.VarFile))
    }
    
    // הוספת משתנים בודדים (כמו Project ID)
    for key, value := range config.Vars {
        args = append(args, "-var", fmt.Sprintf("%s=%s", key, value))
    }

    _, err := RunTerraform(log, config.Dir, args...)
    return err
}

func Destroy(log *zerolog.Logger, config TFConfig) error {
	log.Info().Msg("🔥 Running Terraform Destroy...")
	args := []string{"destroy", "-auto-approve"}
	if config.VarFile != "" {
		args = append(args, fmt.Sprintf("-var-file=%s", config.VarFile))
	}
	_, err := RunTerraform(log, config.Dir, args...)
	return err
}


// deleteGCSBucket מוחק את כל האובייקטים בבוקט ואז מוחק את הבוקט עצמו
func deleteGCSBucket(log *zerolog.Logger, projectID, bucketName string) error {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create storage client: %v", err)
	}
	defer client.Close()

	bucket := client.Bucket(bucketName)

	// GCP מחייב שהבוקט יהיה ריק לפני מחיקה. נמחק את כל האובייקטים (קובצי ה-state):
	it := bucket.Objects(ctx, nil)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to list objects in bucket: %v", err)
		}
		if err := bucket.Object(attrs.Name).Delete(ctx); err != nil {
			return fmt.Errorf("failed to delete object %s: %v", attrs.Name, err)
		}
		log.Debug().Str("object", attrs.Name).Msg("Deleted object from bucket")
	}

	// כעת כשהבוקט ריק, ניתן למחוק אותו
	if err := bucket.Delete(ctx); err != nil {
		return fmt.Errorf("failed to delete bucket %s: %v", bucketName, err)
	}
	return nil
}

// RunTerraformWorkflow - הפונקציה המרכזית המעודכנת
func RunTerraformWorkflow(log *zerolog.Logger, opts TerraformOptions) {
	log.Info().Msg("🚀 Starting Smart Terraform Workflow")

	// 1. בדיקת GCP
	gcpUtils.RunGCPCheck(log, opts.ProjectID)

	// 2. בדיקת קבצים - אם אין קבצי tf, ניצור ברירת מחדל
	files, _ := filepath.Glob(filepath.Join(opts.TerraformDir, "*.tf"))
	if len(files) == 0 {
		if err := createDefaultFiles(log, opts.TerraformDir, opts.ProjectID); err != nil {
			log.Fatal().Err(err).Msg("❌ Failed to create default files")
		}
	}

	// 3. חילוץ שם הבוקט ווידוא קיומו ב-GCP (ה-Parser סורק את כל הקבצים)
	bucketName := ExtractBackendBucket(log, opts.TerraformDir)
	if bucketName != "" {
		if err := ensureGCSBucket(log, opts.ProjectID, bucketName); err != nil {
			log.Fatal().Err(err).Msg("❌ Failed to verify or create the remote state bucket. Stopping workflow.")
		}
	} else {
		log.Fatal().Msg("❌ Critical Error: No GCS bucket name could be extracted from .tf files or backend config. Terraform cannot manage state.")
	}

	tfConfig := TFConfig{
		Dir:             opts.TerraformDir,
		VarFile:         opts.VarFile,
		BackendVarsFile: opts.BackendVarsFile,
		// Vars: map[string]string{
		// 	"project_info": opts.ProjectID,
			
		// },
	}

	// 4. אתחול
	if err := Init(log, tfConfig); err != nil {
		log.Fatal().Err(err).Msg("❌ Terraform Init failed")
	}

	// 5. הרצה
	if opts.Destroy {
		// הרצת ה-Destroy של המשאבים בתוך טראפורם
		if err := Destroy(log, tfConfig); err != nil {
			log.Fatal().Err(err).Msg("❌ Terraform Destroy failed")
		}

		// אם ה-Destroy הצליח, נמחק גם את הבוקט של ה-State
		if bucketName != "" {
			log.Info().Str("bucket", bucketName).Msg("🗑️ Terraform Destroy succeeded. Deleting state bucket...")
			if err := deleteGCSBucket(log, opts.ProjectID, bucketName); err != nil {
				log.Error().Err(err).Msg("❌ Failed to delete state bucket")
			} else {
				log.Info().Msg("✅ State bucket deleted successfully")
			}
		}
	} else {
		// הרצת Apply רגיל
		if err := Apply(log, tfConfig); err != nil {
			log.Fatal().Err(err).Msg("❌ Terraform Apply failed")
		}
	}

	log.Info().Msg("✨ Terraform workflow completed successfully!")
}