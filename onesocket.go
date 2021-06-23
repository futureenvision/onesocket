package onesocket

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// SECTION: WebSocket Handler
type onChannel func(connection *Connection, messageType int, message string)

type Connection struct {
	uuid             string
	groups           []string
	socketConnection websocket.Conn
}

type inChannelData struct {
	Channel     string
	Message     string
	messageType int
	Connection  *Connection
}

type outChannelData struct {
	Channel string `json:"channel"`
	Message string `json:"message"`
}

var upgrader = websocket.Upgrader{}
var socketDataChannel = make(chan inChannelData)
var channels = map[string]onChannel{}
var connections = map[string]Connection{}

func processChan() {
	for data := range socketDataChannel {
		if function, ok := channels[data.Channel]; ok {
			function(data.Connection, data.messageType, data.Message)
		}
	}
}

func listener(w http.ResponseWriter, r *http.Request) {
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }

	socketConnection, error := upgrader.Upgrade(w, r, nil)
	connection := connect(socketConnection)
	if error != nil {
		log.Print("[upgrade][error] -> ", error)
		disconnect(connection.uuid)
		return
	}
	defer socketConnection.Close()

	for {
		messageType, message, err := socketConnection.ReadMessage()
		if err != nil {
			log.Println("[read][error] -> ", err)
			disconnect(connection.uuid)
			break
		}

		packChannelData(&connection, messageType, string(message))
	}
}

func connect(socketConnection *websocket.Conn) (connection Connection) {
	connection = Connection{
		uuid:             uuid.New().String(),
		socketConnection: *socketConnection,
	}
	connections[connection.uuid] = connection
	return connection
}

func disconnect(uuid string) {
	delete(connections, uuid)
}

func packChannelData(connection *Connection, messageType int, jsonString string) {
	var channelData inChannelData
	err := json.Unmarshal([]byte(jsonString), &channelData)
	if err == nil {
		channelData.Connection = connection
		channelData.messageType = messageType
		socketDataChannel <- channelData
	} else {
		log.Println("[channel][error] -> Failed to convert json string into struct")
	}
}

// SECTION: WebSocket
type WebSocket struct {
	Host string
	Port int16
}

func (*WebSocket) JoinGroup(connection *Connection, name string) {
	for _, value := range connection.groups {
		if value == name {
			return
		}
	}
	connection.groups = append(connection.groups, name)
}

func RemoveIndex(s []string, index int) []string {
	return append(s[:index], s[index+1:]...)
}

func (*WebSocket) LeaveGroup(connection *Connection, name string) {
	for index, value := range connection.groups {
		if value == name {
			connection.groups = RemoveIndex(connection.groups, index)
		}
	}
}

func (*WebSocket) On(channel string, function onChannel) {
	channels["channel"] = function
}

func (*WebSocket) Emit(connection *Connection, messageType int, channel string, message string) {
	channelData := outChannelData{
		Channel: channel,
		Message: message,
	}
	data, err := json.Marshal(channelData)
	if err == nil {
		err := connection.socketConnection.WriteMessage(messageType, data)
		if err != nil {
			log.Println("[emit][error] -> ", err)
		}
	}
}

func (*WebSocket) Broadcast(connection *Connection, messageType int, channel string, message string) {
	for _, conn := range connections {
		if conn.uuid != connection.uuid {
			channelData := outChannelData{
				Channel: channel,
				Message: message,
			}
			data, err := json.Marshal(channelData)
			if err == nil {
				err := conn.socketConnection.WriteMessage(messageType, data)
				if err != nil {
					log.Println("[emit][error] -> ", err)
				}
			}
		}
	}
}

func (*WebSocket) EmitToClient(connection *Connection, messageType int, uuid string, channel string, message string) {
	if conn, ok := connections[uuid]; ok {
		if conn.uuid != connection.uuid {
			log.Println("[use 'Emit' to send data to the current client] ")
		}

		channelData := outChannelData{
			Channel: channel,
			Message: message,
		}
		data, err := json.Marshal(channelData)
		if err == nil {
			err := conn.socketConnection.WriteMessage(messageType, data)
			if err != nil {
				log.Println("[emit][error] -> ", err)
			}
		}
	}
}

func (*WebSocket) EmitToGroup(connection *Connection, messageType int, name string, channel string, message string) {
	for _, conn := range connections {
		for _, group := range conn.groups {
			if group == name {
				channelData := outChannelData{
					Channel: channel,
					Message: message,
				}
				data, err := json.Marshal(channelData)
				if err == nil {
					err := conn.socketConnection.WriteMessage(messageType, data)
					if err != nil {
						log.Println("[emit][error] -> ", err)
					}
				}
			}
		}
	}
}

func (socket *WebSocket) ListenAndServe() {
	go processChan()
	addr := flag.String("addr", socket.Host+":"+fmt.Sprint(socket.Port), "http service address")
	flag.Parse()
	log.SetFlags(0)
	fmt.Printf("[Servering ... %s]\n", *addr)
	http.HandleFunc("/echo", listener)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
