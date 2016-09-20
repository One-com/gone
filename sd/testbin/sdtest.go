package main

import (
	sd ".."
	"bufio"
	"fmt"
	"log"
	"net"
)

func handleConn(conn net.Conn, quit chan struct{}) {
	b := bufio.NewReader(conn)
	for {
		line, err := b.ReadBytes('\n')
		if err != nil {
			break
		}
		fmt.Println(line)
		if line[0] == 'q' {
			close(quit)
			break
		}
		conn.Write(line)
	}
	conn.Close()
}

func main() {

	l, err := sd.ListenTCP("tcp", nil)
	if err != nil {
		log.Fatal(err)
	}

	log.Println(l.Addr())

	quit := make(chan struct{})

	err = sd.NotifyStatus(sd.StatusReady, "Up and running")
	if err != nil {
		log.Println(err)
	}

	go func() {
		select {
		case <-quit:
			fmt.Println("Exiting")
			l.Close()
		}
	}()

	for {

		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handleConn(conn, quit)
	}
}
