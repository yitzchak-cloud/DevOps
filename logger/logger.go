package logger

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/rs/zerolog"
)

var LogStream = make(chan []byte, 100)
var logMu sync.Mutex

type multiWriterLog struct {
	fileWriter io.Writer
}

func (w *multiWriterLog) Write(p []byte) (n int, err error) {
	// 1. הפיכת ה-JSON הדחוס ל-JSON מעוצב (Pretty Print)
	var jsonObj interface{}
	// מפענחים את השורה שקיבלנו מ-zerolog
	if err := json.Unmarshal(p, &jsonObj); err != nil {
		// אם זה לא JSON תקני, נכתוב אותו כפי שהוא
		return w.fileWriter.Write(p)
	}

	// יוצרים JSON חדש עם רווחים (4 רווחים להזחה)
	prettyJSON, err := json.MarshalIndent(jsonObj, "", "    ")
	if err != nil {
		return w.fileWriter.Write(p)
	}

	// מוסיפים ירידת שורה כפולה בין לוג ללוג כדי שיהיה קל להבדיל ביניהם
	prettyJSON = append(prettyJSON, []byte("\n\n")...)

	// 2. כתיבה לקובץ הלוג (הגרסה המעוצבת)
	_, err = w.fileWriter.Write(prettyJSON)
	if err != nil {
		return 0, err
	}

	// 3. שליחת הלוג המעוצב לערוץ ה-WebSocket
	logMu.Lock()
	defer logMu.Unlock()
	select {
	case LogStream <- append([]byte{}, prettyJSON...):
	default:
	}

	// מחזירים את האורך המקורי של p כדי ש-zerolog לא יחשוב שהייתה שגיאה
	return len(p), nil
}

// InitLogger מאתחל את מערכת הלוגינג
// cleanLogs: אם true, הקובץ יימחק וייווצר מחדש בכל הפעלה
func InitLogger(cleanLogs bool) zerolog.Logger {
    logFilePath := filepath.Join(".", "app.log")
    
    // קביעת הדגלים לפתיחת הקובץ
    // O_CREATE: יוצר את הקובץ אם לא קיים
    // O_WRONLY: פותח לכתיבה בלבד
    flags := os.O_CREATE | os.O_WRONLY
    
    if cleanLogs {
        // מרוקן את הקובץ (Truncate)
        flags |= os.O_TRUNC
    } else {
        // מוסיף לסוף הקובץ הקיים (Append)
        flags |= os.O_APPEND
    }

    logFile, err := os.OpenFile(logFilePath, flags, 0644)
    if err != nil {
        panic("Failed to open log file: " + err.Error())
    }

    writer := &multiWriterLog{fileWriter: logFile}

    // הגדרת פורמט זמן קריא
    zerolog.TimeFieldFormat = "2006-01-02 15:04:05"
    
    logr := zerolog.New(writer).With().Timestamp().Logger()

    if cleanLogs {
        logr.Info().Msg("Logger initialized. Previous logs cleared.")
    } else {
        logr.Info().Msg("Logger initialized. Appending to existing logs.")
    }
    
    return logr
}