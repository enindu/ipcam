package main

import (
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/blackjack/webcam"
	"github.com/gorilla/websocket"
)

const (
	V4L2_JPEG = 0x47504A50
	V4L2_YUYV = 0x56595559
)

var (
	connections      = make(map[*websocket.Conn]bool)
	connectionsMutex = sync.Mutex{}
	frame            = make(chan []byte)
)

func main() {
	camera, exception := webcam.Open("/dev/video0")
	if exception != nil {
		log.Fatalln(exception)
	}

	_, _, _, exception = camera.SetImageFormat(V4L2_JPEG, 640, 480)
	if exception != nil {
		log.Fatalln(exception)
	}

	exception = camera.StartStreaming()
	if exception != nil {
		log.Fatalln(exception)
	}

	defer camera.Close()
	defer camera.StopStreaming()

	go readFrame(camera)
	go writeMessage()

	http.HandleFunc("/", index)

	exception = http.ListenAndServe(":8000", nil)
	if exception != nil {
		log.Fatalln(exception)
	}
}

func readFrame(c *webcam.Webcam) {
	for {
		frameBytes, exception := c.ReadFrame()
		if exception != nil {
			continue
		}

		frame <- frameBytes

		time.Sleep(10 * time.Millisecond)
	}
}

func writeMessage() {
	for {
		if len(connections) < 1 {
			continue
		}

		connectionsMutex.Lock()

		for connection := range connections {
			exception := connection.WriteMessage(websocket.BinaryMessage, <-frame)
			if exception != nil {
				log.Printf("%s is disconnected\n", connection.RemoteAddr().String())
				connection.Close()
				delete(connections, connection)

				break
			}
		}

		connectionsMutex.Unlock()
	}
}

func index(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: checkOrigin,
	}

	connection, exception := upgrader.Upgrade(w, r, nil)
	if exception != nil {
		log.Fatalln(exception)
	}

	connectionsMutex.Lock()

	for existingConnection := range connections {
		if strings.Split(existingConnection.RemoteAddr().String(), ":")[0] == strings.Split(connection.RemoteAddr().String(), ":")[0] {
			log.Printf("%s is disconnected (Existing)\n", existingConnection.RemoteAddr().String())
			existingConnection.Close()
			delete(connections, existingConnection)
		}
	}

	connectionsMutex.Unlock()

	connections[connection] = true

	log.Printf("%s is connected\n", connection.RemoteAddr().String())
}

func checkOrigin(r *http.Request) bool {
	return true
}
