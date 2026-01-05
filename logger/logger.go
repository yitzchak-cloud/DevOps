// logger/logger.go
package logger

import (
	"io"
	"os"
	"path/filepath"
	"sync"
	"github.com/rs/zerolog"
)

// LogStream הוא ערוץ (Channel) שאליו נשלחים כל הלוגים החדשים (בפורמט JSON String)
var LogStream = make(chan []byte, 100)
// logMu משמש לנעילה לשמירה על בטיחות השרשור של הכתיבה
var logMu sync.Mutex

// multiWriterLog הוא מבנה שמיישם את הממשק io.Writer
// הוא כותב בו זמנית גם לקובץ וגם לערוץ ה-LogStream
type multiWriterLog struct {
	fileWriter io.Writer
}

func (w *multiWriterLog) Write(p []byte) (n int, err error) {
	// 1. כתיבה לקובץ הלוג
	n, err = w.fileWriter.Write(p)
	if err != nil {
		return n, err
	}

	// 2. שליחת הלוג לערוץ ה-WebSocket (ב-goroutine כדי לא לחסום את הלוגר)
	logMu.Lock()
	defer logMu.Unlock()
	select {
	case LogStream <- append([]byte{}, p...): // שליחת עותק של הנתונים
	default:
		// אם הערוץ מלא, מדלגים על הלוג הזה כדי למנוע חסימה
		// במערכות גדולות יותר נרצה לטפל בזה בצורה חזקה יותר
	}

	return n, err
}

// InitLogger מאתחל את מערכת הלוגינג
func InitLogger() zerolog.Logger {
	// יצירת קובץ הלוג
	logFilePath := filepath.Join(".", "app.log")
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		(&zerolog.Logger{}).Fatal().Err(err).Msg("Failed to open log file")
	}

	// הגדרת ה-multiWriterLog שיכתוב לקובץ וגם לערוץ
	writer := &multiWriterLog{fileWriter: logFile}

	// הגדרת Zerolog להשתמש ב-writer המותאם אישית
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	logr := zerolog.New(writer).With().Timestamp().Logger()

	logr.Info().Msgf("Logger initialized. Logs are being streamed and written to %s", logFilePath)
	
	return logr
}

