package main

import (
	"flag"
	"log"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)



func readTCP (c net.Conn) ([]byte, error) {
	buf := make([]byte, 1024)
	n, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func proxy(mc net.Conn, proxyCh chan<- bool, address string, scheme string, path string) {
	u := url.URL{Scheme: scheme, Host: address, Path: path}
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Printf("dial: %v", err)
		proxyCh <- false
		return
	}

	resultCh := make(chan bool, 2)
	var wg sync.WaitGroup
	wg.Add(2)

	go func(mc net.Conn, c *websocket.Conn) {
		defer wg.Done()
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Printf("websocket read error: %v", err)
				resultCh <- false
				return
			}
			if _, err := mc.Write(message); err != nil {
				log.Printf("write to minecraft error: %v", err)
				resultCh <- true
				return
			}
		}
	}(mc, c)

	go func(mc net.Conn, c *websocket.Conn) {
		defer wg.Done()
		for {
			message, err := readTCP(mc)
			if err != nil {
				log.Printf("read from minecraft error: %v", err)
				resultCh <- true
				return
			}
			if err := c.WriteMessage(websocket.BinaryMessage, message); err != nil {
				log.Printf("websocket write error: %v", err)
				resultCh <- false
				return
			}
		}
	}(mc, c)

	status := <-resultCh
	c.Close()
	wg.Wait()

	if status {
		mc.Close()
		proxyCh <- true
		return
	}

	proxyCh <- false
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
