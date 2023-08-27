// Command client is a terminal chat client. Reading stdin and the socket
// happen on separate goroutines, so incoming messages show up while you type.
// Type /quit or exit to leave.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"realtimechat/internal/chat"
)

func main() {
	addr := flag.String("addr", "ws://localhost:8080/ws", "server WebSocket URL")
	user := flag.String("user", "", "username (prompted if empty)")
	room := flag.String("room", chat.DefaultRoom, "room to join")
	reconnect := flag.Bool("reconnect", false, "reconnect with exponential backoff on disconnect")
	flag.Parse()

	name := strings.TrimSpace(*user)
	if name == "" {
		fmt.Print("Enter your name: ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		name = strings.TrimSpace(line)
		if name == "" {
			log.Fatal("a username is required")
		}
	}

	// Shared stdin reader feeds outbound messages across reconnects.
	lines := make(chan string, 16)
	go func() {
		sc := bufio.NewScanner(os.Stdin)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()

	backoff := time.Second
	for {
		if err := session(*addr, name, *room, lines); err != nil {
			log.Printf("disconnected: %v", err)
		}
		if !*reconnect {
			return
		}
		log.Printf("reconnecting in %s...", backoff)
		time.Sleep(backoff)
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

// session dials the server and runs until the connection closes or the user
// quits. It returns nil on a clean /quit.
func session(addr, name, room string, lines <-chan string) error {
	u, err := url.Parse(addr)
	if err != nil {
		return err
	}
	q := u.Query()
	q.Set("user", name)
	q.Set("room", room)
	u.RawQuery = q.Encode()

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	readErr := make(chan error, 1)
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				readErr <- err
				return
			}
			var out chat.Outgoing
			if json.Unmarshal(data, &out) != nil {
				continue
			}
			switch out.Type {
			case chat.TypeMessage:
				fmt.Printf("%s: %s\n", out.From, out.Text)
			case chat.TypeSystem:
				fmt.Printf("* %s\n", out.Text)
			case chat.TypePresence:
				fmt.Printf("* online: %s\n", strings.Join(out.Members, ", "))
			case chat.TypeJoin:
				fmt.Printf("* you are %q in %q\n", out.From, out.Room)
			case chat.TypeError:
				fmt.Printf("! %s\n", out.Text)
			}
		}
	}()

	for {
		select {
		case err := <-readErr:
			return err
		case line, ok := <-lines:
			if !ok {
				_ = sendQuit(conn)
				return nil
			}
			text := strings.TrimSpace(line)
			if text == "" {
				continue
			}
			if text == "/quit" || text == "exit" {
				_ = sendQuit(conn)
				return nil
			}
			if err := sendJSON(conn, chat.Incoming{Type: chat.TypeMessage, Text: text}); err != nil {
				return err
			}
		}
	}
}

func sendJSON(conn *websocket.Conn, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, data)
}

func sendQuit(conn *websocket.Conn) error {
	return sendJSON(conn, chat.Incoming{Type: chat.TypeQuit})
}
