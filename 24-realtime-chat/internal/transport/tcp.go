package transport

import (
	"bufio"
	"encoding/json"
	"net"
	"strings"

	"realtimechat/internal/chat"
)

// ServeTCP serves plain line-based clients (e.g. telnet) on ln until it is
// closed. The first line from a client is its name; every later line is a
// message, shown to others as "Name: message".
func ServeTCP(ln net.Listener, hub *chat.Hub) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go handleTCPConn(conn, hub)
	}
}

func handleTCPConn(conn net.Conn, hub *chat.Hub) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	_, _ = conn.Write([]byte("Enter your name: "))
	nameLine, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	name := strings.TrimSpace(nameLine)
	if name == "" {
		name = "anon"
	}

	c := chat.NewClient(hub, name, chat.DefaultRoom, sendBuffer)
	if !hub.Register(c) {
		return
	}
	defer hub.Unregister(c)

	go tcpWritePump(conn, c)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		text := strings.TrimSpace(line)
		switch text {
		case "":
			continue
		case "/quit", "exit":
			return
		default:
			hub.Message(c, text)
		}
	}
}

// tcpWritePump turns hub frames into plain text lines.
func tcpWritePump(conn net.Conn, c *chat.Client) {
	for data := range c.Send() {
		var out chat.Outgoing
		if json.Unmarshal(data, &out) != nil {
			continue
		}
		var line string
		switch out.Type {
		case chat.TypeMessage:
			line = out.From + ": " + out.Text + "\n"
		case chat.TypeSystem:
			line = "* " + out.Text + "\n"
		case chat.TypeJoin:
			line = "* you are " + out.From + " in " + out.Room + "\n"
		case chat.TypePresence:
			line = "* online: " + strings.Join(out.Members, ", ") + "\n"
		case chat.TypeError:
			line = "! " + out.Text + "\n"
		}
		if line == "" {
			continue
		}
		if _, err := conn.Write([]byte(line)); err != nil {
			return
		}
	}
}
