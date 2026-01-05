package main

import (
	"embed"
	"net/http"
	"DevOps/logger" // וודא שהנתיב ל-logger נכון
	"github.com/gorilla/websocket"
	"time"
)

//go:embed web
var content embed.FS // מטמיע את תיקיית 'web' לתוך הבינארי

// הגדרת הממשק לשדרוג חיבורי HTTP ל-WebSocket
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // מאפשר לכל דומיין להתחבר (לא מומלץ בסביבת פרודקשן)
	},
}

// handleWebSockets מטפל בחיבורי WebSocket ומזרים את הלוגים
func handleWebSockets(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to upgrade connection to WebSocket")
		return
	}
	defer conn.Close()

	log.Info().Str("remote_addr", r.RemoteAddr).Msg("New WebSocket client connected")

	// קריאה מהערוץ ושליחה ללקוח
	for logEntry := range logger.LogStream {
		if err := conn.WriteMessage(websocket.TextMessage, logEntry); err != nil {
			log.Info().Str("remote_addr", r.RemoteAddr).Msg("Client disconnected")
			return
		}
	}
}

// startWebServer מגדיר ומפעיל את שרת האינטרנט
func startWebServer() {
	// הגשת קובץ ה-HTML הראשי (המציג את הלוגים)
	http.Handle("/", http.FileServer(http.FS(content)))
	
	// נקודת הקצה (Endpoint) לחיבורי WebSocket
	http.HandleFunc("/ws/logs", handleWebSockets)
	
	port := "9090"
	log.Info().Msgf("🌐 Starting Web Server on http://localhost:%s. Open this URL in your browser to view logs.", port)

	// הפעלת השרת
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal().Err(err).Msg("Web server failed to start")
	}
	// 3. המתן כמה שניות לוודא שהשרת עלה
	time.Sleep(2 * time.Second)
	
}