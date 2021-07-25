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
type onChannel func(connection *Connection, inDataType int, inData map[string]interface{})

type Connection struct {
	Uuid             string
	Groups           []string
	socketConnection websocket.Conn
}

type inChannelData struct {
	Channel     string
	RequestId   string
	Message     map[string]interface{}
	messageType int
	Connection  *Connection
}

type outChannelData struct {
	Channel string      `json:"channel"`
	Message interface{} `json:"message"`
}

var upgrader = websocket.Upgrader{}
var socketDataChannel = make(chan inChannelData)
var channels = map[string]onChannel{}
var connections = map[string]Connection{}

func processChan() {
	for data := range socketDataChannel {
		if function, ok := channels[data.Channel]; ok {
			go function(data.Connection, data.messageType, data.Message)
		}
	}
}

func listener(w http.ResponseWriter, r *http.Request) {
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }

	socketConnection, error := upgrader.Upgrade(w, r, nil)
	connection := connect(socketConnection)
	if error != nil {
		log.Print("[upgrade][error] -> ", error)
		disconnect(connection.Uuid)
		return
	}
	defer socketConnection.Close()

	for {
		messageType, message, err := socketConnection.ReadMessage()
		if err != nil {
			log.Println("[read][error] -> ", err)
			disconnect(connection.Uuid)
			break
		}

		packChannelData(&connection, messageType, string(message))
	}
}

func connect(socketConnection *websocket.Conn) (connection Connection) {
	connection = Connection{
		Uuid:             uuid.New().String(),
		socketConnection: *socketConnection,
	}
	connections[connection.Uuid] = connection
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
	Host     string
	Port     int16
	Endpoint string
}

func (*WebSocket) JoinGroup(connection *Connection, name string) {
	for _, value := range connection.Groups {
		if value == name {
			return
		}
	}
	connection.Groups = append(connection.Groups, name)
}

func RemoveIndex(s []string, index int) []string {
	return append(s[:index], s[index+1:]...)
}

func (*WebSocket) LeaveGroup(connection *Connection, name string) {
	for index, value := range connection.Groups {
		if value == name {
			connection.Groups = RemoveIndex(connection.Groups, index)
		}
	}
}

func (*WebSocket) On(channel string, function onChannel) {
	channels[channel] = function
}

func (*WebSocket) Emit(connection *Connection, outDataType int, channel string, outData interface{}) {
	channelData := outChannelData{
		Channel: channel,
		Message: outData,
	}
	data, err := json.Marshal(channelData)
	if err == nil {
		err := connection.socketConnection.WriteMessage(outDataType, data)
		if err != nil {
			log.Println("[emit][error] -> ", err)
		}
	}
}

func (*WebSocket) Broadcast(excludeConnection *Connection, outDataType int, channel string, outData interface{}) {
	for _, conn := range connections {
		uuid := ""
		if excludeConnection != nil {
			uuid = excludeConnection.Uuid
		}
		if conn.Uuid != uuid {
			channelData := outChannelData{
				Channel: channel,
				Message: outData,
			}
			data, err := json.Marshal(channelData)
			if err == nil {
				err := conn.socketConnection.WriteMessage(outDataType, data)
				if err != nil {
					log.Println("[emit][error] -> ", err)
				}
			}
		}
	}
}

func (*WebSocket) EmitToClient(uuid string, channel string, outDataType int, outData interface{}) {
	for _, conn := range connections {
		if conn.Uuid == uuid {
			channelData := outChannelData{
				Channel: channel,
				Message: outData,
			}
			data, err := json.Marshal(channelData)
			if err == nil {
				err := conn.socketConnection.WriteMessage(outDataType, data)
				if err != nil {
					log.Println("[emit][error] -> ", err)
				}
			}
		}
	}
}

func (*WebSocket) EmitToGroup(connection *Connection, outDataType int, name string, channel string, outData interface{}) {
	for _, conn := range connections {
		for _, group := range conn.Groups {
			if group == name {
				channelData := outChannelData{
					Channel: channel,
					Message: outData,
				}
				data, err := json.Marshal(channelData)
				if err == nil {
					err := conn.socketConnection.WriteMessage(outDataType, data)
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
	log.Printf("[Servering ... %s%s]\n", *addr, socket.Endpoint)
	http.HandleFunc(socket.Endpoint, listener)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
