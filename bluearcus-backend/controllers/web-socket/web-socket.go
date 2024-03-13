package websocket

import (
	"bluearcus-backend/inits"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

// ImplementingWebSocketForLiveUpdate initializes WebSocket functionality for live updates.
func WebsocketHandler(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP connection to WebSocket
	upgrader := inits.GetWebSocketUpgrader()
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Could not upgrade connection to WebSocket", http.StatusBadRequest)
		return
	}

	// Handle WebSocket messages
	go func() {
		defer conn.Close() // Close the WebSocket connection when the goroutine exits
		for {
			select {
			case broadcast := <-inits.Socketchannel:
				// Write the message to the WebSocket connection
				err := conn.WriteMessage(websocket.TextMessage, broadcast)
				if err != nil {
					log.Println("Error writing to WebSocket:", err)
				}
			case <- inits.CloseSocketchannel:
				log.Println("WebSocket shut down")
				return // Exit the goroutine if CloseSocketchannel is closed
			}
		}
	}()
}