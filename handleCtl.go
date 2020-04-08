package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

type Msg struct {
	from string
	msg  string
}

func nick(nick string) string {
	return fmt.Sprintf("nick %s\n", nick)
}

func join(channel string) string {
	return fmt.Sprintf("join %s\n", channel)
}

func processMsg(msg Msg) string {
	if strings.HasPrefix(msg.msg, "/") {
		// We have a command
		ps := strings.Split(msg.msg, " ")
		parts := ps[:0]
		for _, s := range ps {
			if s != "" {
				parts = append(parts, s)
			}
		}
		switch parts[0] {
		case "/j":
			log.Printf("JOIN %s", parts[1])
			return join(parts[1])
		case "/q":
			log.Printf("QUIT")
		case "/p":
			log.Printf("PART")
		case "/n":
			log.Printf("NICK %s", parts[1])
			return nick(parts[1])
		case "/m":
			log.Printf("MSG [%s]: %s", parts[1], strings.Join(parts[2:], " "))
			return fmt.Sprintf("msg %s %s\n", parts[1], strings.Join(parts[2:], " "))
		default:
			log.Printf("UNKNOWN: [%#v]", parts)
		}
		return ""
	}

	if msg.from == "log" || msg.from == "raw" {
		return ""
	}

	return fmt.Sprintf("msg %s %s\n", msg.from, msg.msg)
}

func (a *App) readCtl(f *os.File) chan []byte {
	c := make(chan []byte)
	go func() {
		for {
			select {
			case <-a.shutdown:
				close(c)
				return
			default:
			}
			bs := make([]byte, 1024)
			n, err := f.Read(bs)
			if err != nil {
				log.Printf("Failed to read ctl file: %s", err)
				time.Sleep(1 * time.Second)
				continue
			}
			c <- bs[:n]
		}
	}()
	return c
}

func (a *App) handleCtl(f *os.File, msgs chan Msg) {
	defer f.Close()
	c := a.readCtl(f)
	for {
		select {
		case bs := <-c:
			log.Printf("CTL: %s", string(bs))
		case m := <-msgs:
			out := processMsg(m)
			if out != "" {
				_, err := f.Write([]byte(out)) // TODO: do something with n
				if err != nil {
					log.Printf("Error writing: %s", err)
				}
			}
		case <-a.shutdown:
			return
		}
	}
}
