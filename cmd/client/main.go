package main

import (
	"flag"
	"log"
	"net"
	"net/url"
	"time"

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

func proxy(mc net.Conn, proxyCh chan<- bool, address string, scheme string, path string) {
	alive := true
	u := url.URL{Scheme: scheme, Host: address, Path: path}
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
		c.Close()
		proxyCh <- false
		return
	}
	doneCh := make(chan bool)
	go func(mc net.Conn, c *websocket.Conn) {
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Print("Error, ", err)
				alive = false
				break
			}
			mc.Write(message)
		}
		doneCh <- false
		return
	}(mc, c)
	go func(mc net.Conn, c *websocket.Conn) {
		for {
			if !alive {
				break
			}
			message, err := readTCP(mc)
			if err != nil {
				log.Print("Error readTCP, ", err)
				break
			}
			if !alive {
				break
			}
			log.Print(message)
			err = c.WriteMessage(websocket.BinaryMessage, message)
			if err != nil {
				log.Print("Error Write WS, ", err)
			}
		}
		doneCh <- true
		return
	}(mc, c)
	if <-doneCh {
		mc.Close()
		c.Close()
		<-doneCh
		proxyCh <- true
	} else {
		c.Close()
		log.Print("WebSocket Close")
		proxyCh <- false
	}
	return
}

func main () {
	address := flag.String("server", "127.0.0.1:8080", "Server Address")
	scheme := flag.String("scheme", "wss", "WebSocket scheme")
	path := flag.String("path", "/", "Path")
	listener, err := net.Listen("tcp", ":25560")
	flag.Parse()
	if err != nil {
		log.Print(err)
		panic("Error")
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Print("Error: ", err)
		}
		proxyCh := make(chan bool)
		log.Print(*address, *scheme, *path)
		log.Print(conn)
		go proxy(conn, proxyCh, *address, *scheme, *path)
		for {
			if <- proxyCh {
				break
			} else {
				time.Sleep(time.Millisecond * 500)
				log.Print("WebSocket reconnecting...")
				go proxy(conn, proxyCh, *address, *scheme, *path)
			}
		}
	}
}
