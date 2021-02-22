package main

import (
	"log"
	"net"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{}

func readTCP (c net.Conn) ([]byte, error) {
	buf := make([]byte, 1024)
	n, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func proxy(w http.ResponseWriter, r *http.Request) {
	mc, err := net.Dial("tcp", "127.0.0.1:25565")
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade error:", err)
		mc.Close()
		c.Close()
		return
	}
	doneCh := make(chan bool)
	go func(mc net.Conn, c *websocket.Conn) {
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Print("Error, ", err)
				break
			}
			mc.Write(message)
		}
		doneCh <- true
	}(mc, c)
	go func(mc net.Conn, c *websocket.Conn) {
		for {
			message, err := readTCP(mc)
			if err != nil {
				log.Print("Error, ", err)
				break
			}
			err = c.WriteMessage(websocket.BinaryMessage, message)
			if err != nil {
				log.Print("Error, ", err)
			}
		}
		doneCh <- true
	}(mc, c)
	<-doneCh
	mc.Close()
	c.Close()
	<-doneCh
}

func main() {
	http.HandleFunc("/", proxy)
	err := http.ListenAndServe(":8080", nil)

	if err != nil {
		panic("Error")
	}
}
