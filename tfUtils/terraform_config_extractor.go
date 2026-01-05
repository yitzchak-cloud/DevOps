package tfUtils

import (
    "encoding/json"
    "os"
    "path/filepath"

    "github.com/hashicorp/hcl/v2"
    "github.com/hashicorp/hcl/v2/hclparse"
    "github.com/rs/zerolog"
    "github.com/zclconf/go-cty/cty"
)

// TerraformConfigExtractor מאפשר חילוץ ערכים מקונפיגורציה של Terraform
type TerraformConfigExtractor struct {
    log    *zerolog.Logger
    dir    string
    parser *hclparse.Parser
}

// NewTerraformConfigExtractor יוצר extractor חדש
func NewTerraformConfigExtractor(log *zerolog.Logger, dir string) *TerraformConfigExtractor {
    return &TerraformConfigExtractor{
        log:    log,
        dir:    dir,
        parser: hclparse.NewParser(),
    }
}

// ExtractVariable מחלץ ערך של משתנה מכל המקורות האפשריים
// תומך ב: קבצי .tf, .tfvars, .hcl, ו-terraform.tfstate
func (e *TerraformConfigExtractor) ExtractVariable(varName string, sources ...ConfigSource) string {
    e.log.Debug().Str("variable", varName).Msg("🔍 Searching for variable...")

    // אם לא צוינו sources, נשתמש בכולם
    if len(sources) == 0 {
        sources = []ConfigSource{
            ConfigSourceTfFiles,
            ConfigSourceBackendFiles,
            ConfigSourceTerraformState,
        }
    }

    for _, source := range sources {
        var value string
        switch source {
        case ConfigSourceTfFiles:
            value = e.extractFromTfFiles(varName)
        case ConfigSourceBackendFiles:
            value = e.extractFromBackendFiles(varName)
        case ConfigSourceTerraformState:
            value = e.extractFromTerraformState(varName)
        }

        if value != "" {
            return value
        }
    }

    e.log.Debug().Str("variable", varName).Msg("ℹ️ Variable not found in any source")
    return ""
}

// ConfigSource מגדיר מהיכן לחפש את הקונפיגורציה
type ConfigSource int

const (
    ConfigSourceTfFiles ConfigSource = iota
    ConfigSourceBackendFiles
    ConfigSourceTerraformState
)

// extractFromTfFiles מחלץ משתנה מקבצי .tf
func (e *TerraformConfigExtractor) extractFromTfFiles(varName string) string {
    files, _ := filepath.Glob(filepath.Join(e.dir, "*.tf"))

    for _, file := range files {
        if value := e.extractFromHclFile(file, varName, "terraform", "backend", "gcs"); value != "" {
            e.log.Info().
                Str("variable", varName).
                Str("value", value).
                Str("source", file).
                Msg("📍 Found variable in .tf file")
            return value
        }
    }
    return ""
}

// extractFromBackendFiles מחלץ משתנה מקבצי backend config
func (e *TerraformConfigExtractor) extractFromBackendFiles(varName string) string {
    possibleFiles := []string{
        "backend.tfvars",
        "backend.hcl",
        "config.gcs.tfbackend",
        "backend.tf",
    }

    for _, fileName := range possibleFiles {
        filePath := filepath.Join(e.dir, fileName)
        if _, err := os.Stat(filePath); os.IsNotExist(err) {
            continue
        }

        e.log.Debug().Str("file", filePath).Msg("Found backend config file")

        // נסה קודם כ-attributes file (tfvars/hcl)
        if value := e.extractFromAttributesFile(filePath, varName); value != "" {
            e.log.Info().
                Str("variable", varName).
                Str("value", value).
                Str("source", filePath).
                Msg("📍 Found variable in backend config file")
            return value
        }

        // אם זה backend.tf, נסה למצוא בתוך הבלוק
        if value := e.extractFromHclFile(filePath, varName, "terraform", "backend", "gcs"); value != "" {
            e.log.Info().
                Str("variable", varName).
                Str("value", value).
                Str("source", filePath).
                Msg("📍 Found variable in backend.tf")
            return value
        }
    }

    return ""
}

// extractFromAttributesFile מחלץ attribute מקובץ פשוט (כמו tfvars)
func (e *TerraformConfigExtractor) extractFromAttributesFile(filePath, attrName string) string {
    hclFile, diags := e.parser.ParseHCLFile(filePath)
    if diags.HasErrors() {
        e.log.Debug().Str("file", filePath).Msg("Skipping due to parse errors")
        return ""
    }

    attrs, diags := hclFile.Body.JustAttributes()
    if diags.HasErrors() {
        return ""
    }

    if attr, ok := attrs[attrName]; ok {
        val, diags := attr.Expr.Value(nil)
        if !diags.HasErrors() && val.Type() == cty.String {
            return val.AsString()
        }
    }

    return ""
}

// extractFromHclFile מחלץ attribute מתוך nested blocks בקובץ HCL
// blockPath: רשימת הבלוקים שצריך לעבור (לדוגמה: "terraform", "backend", "gcs")
func (e *TerraformConfigExtractor) extractFromHclFile(filePath, attrName string, blockPath ...string) string {
    hclFile, diags := e.parser.ParseHCLFile(filePath)
    if diags.HasErrors() {
        return ""
    }

    body := hclFile.Body
    
    // עוברים על כל הבלוקים בנתיב
    for i, blockType := range blockPath {
        schema := &hcl.BodySchema{
            Blocks: []hcl.BlockHeaderSchema{{
                Type: blockType,
                LabelNames: getLabelNames(i, len(blockPath)),
            }},
        }

        content, _, diags := body.PartialContent(schema)
        if diags.HasErrors() || len(content.Blocks) == 0 {
            return ""
        }

        // אם זה הבלוק האחרון, נחפש את ה-attribute
        if i == len(blockPath)-1 {
            for _, block := range content.Blocks {
                // בדיקה שהתווית מתאימה (למשל "gcs" עבור backend)
                if len(block.Labels) > 0 && i < len(blockPath)-1 {
                    continue
                }
                if len(block.Labels) > 0 && block.Labels[0] != blockPath[i] {
                    continue
                }

                attrs, _ := block.Body.JustAttributes()
                if attr, ok := attrs[attrName]; ok {
                    val, _ := attr.Expr.Value(nil)
                    if val.Type() == cty.String {
                        return val.AsString()
                    }
                }
            }
        } else {
            // ממשיכים לבלוק הבא
            if len(content.Blocks) > 0 {
                body = content.Blocks[0].Body
            }
        }
    }

    return ""
}

// getLabelNames מחזיר label names לפי המיקום בנתיב
func getLabelNames(currentIndex, totalBlocks int) []string {
    // הבלוק האחרון בדרך כלל צריך label (כמו "gcs")
    if currentIndex == totalBlocks-1 {
        return []string{"type"}
    }
    return nil
}

// extractFromTerraformState מחלץ משתנה מ-terraform.tfstate המקומי
func (e *TerraformConfigExtractor) extractFromTerraformState(varName string) string {
    stateFile := filepath.Join(e.dir, ".terraform", "terraform.tfstate")

    if _, err := os.Stat(stateFile); os.IsNotExist(err) {
        return ""
    }

    data, err := os.ReadFile(stateFile)
    if err != nil {
        return ""
    }

    var state map[string]interface{}
    if err := json.Unmarshal(data, &state); err != nil {
        return ""
    }

    // ניווט ל-backend.config
    if backend, ok := state["backend"].(map[string]interface{}); ok {
        if config, ok := backend["config"].(map[string]interface{}); ok {
            if value, ok := config[varName].(string); ok {
                e.log.Info().
                    Str("variable", varName).
                    Str("value", value).
                    Str("source", stateFile).
                    Msg("📍 Found variable in terraform.tfstate")
                return value
            }
        }
    }

    return ""
}