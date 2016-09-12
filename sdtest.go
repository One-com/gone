package main

import (
	"./sd"
	"bufio"
	"log"
	"net"
)

func handleConn(conn net.Conn) {
	b := bufio.NewReader(conn)
	for {
		line, err := b.ReadBytes('\n')
		if err != nil {
			break
		}
		conn.Write(line)
	}
}

func main() {

	l, err := sd.ListenTCP("tcp",nil)
	if err != nil {
		log.Fatal(err)
	}

	log.Println(l.Addr())
	
	sd.NotifyStatus(sd.StatusReady, "Up and running")
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handleConn(conn)
	}
}

