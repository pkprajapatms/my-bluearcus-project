package inits

import (
	"net/http"
	"github.com/gorilla/websocket"
)

var SocketUpgrader websocket.Upgrader

var Socketchannel      = make(chan []byte)
var CloseSocketchannel = make(chan bool)

func GetWebSocketUpgrader() websocket.Upgrader{
	return SocketUpgrader
}


// Initialize database connection
func InitWebSocket() {
	SocketUpgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			// Allow all connections by returning true
			return true
		},
	}
}