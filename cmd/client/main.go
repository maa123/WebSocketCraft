package main

import (
	"context"
	"flag"
	"log"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

func readTCP(c net.Conn) ([]byte, error) {
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

	shouldCloseMcCh := make(chan bool, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cleanup := func() {
		cancel()
		c.Close()
	}
	var resultOnce sync.Once
	var wg sync.WaitGroup
	wg.Add(2)

	// Only the first disconnect reason should be reported back to the caller.
	signalResult := func(shouldCloseMinecraft bool) {
		resultOnce.Do(func() {
			select {
			case shouldCloseMcCh <- shouldCloseMinecraft:
			case <-ctx.Done():
			}
		})
	}

	go func(mc net.Conn, c *websocket.Conn) {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Printf("websocket read error: %v", err)
				signalResult(false)
				return
			}
			if _, err := mc.Write(message); err != nil {
				log.Printf("minecraft write error: %v", err)
				signalResult(true)
				return
			}
		}
	}(mc, c)

	go func(mc net.Conn, c *websocket.Conn) {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			message, err := readTCP(mc)
			if err != nil {
				log.Printf("minecraft read error: %v", err)
				signalResult(true)
				return
			}
			if err := c.WriteMessage(websocket.BinaryMessage, message); err != nil {
				log.Printf("websocket write error: %v", err)
				signalResult(false)
				return
			}
		}
	}(mc, c)

	shouldCloseMinecraft := <-shouldCloseMcCh
	cleanup()
	wg.Wait()

	if shouldCloseMinecraft {
		mc.Close()
		proxyCh <- true
		return
	}

	proxyCh <- false
}

func main() {
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
			if <-proxyCh {
				break
			} else {
				time.Sleep(time.Millisecond * 500)
				log.Print("WebSocket reconnecting...")
				go proxy(conn, proxyCh, *address, *scheme, *path)
			}
		}
	}
}
