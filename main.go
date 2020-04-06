package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"9fans.net/go/draw"
	"github.com/mjl-/duit"
)

type Channel struct {
	name string
	f    *os.File
	msg  *duit.Field
	edit *duit.Edit
	kids []*duit.Kid
}

func (c *Channel) follow(d *duit.DUI) {
	var ba [4096]byte
	bs := ba[:]
	for {
		n, err := c.f.Read(bs)
		if err != nil {
			log.Printf("ERR reading %s: %s", c.name, err)
			time.Sleep(1 * time.Second)
			continue
		}
		c.edit.Append(bs[:n])
		c.edit.ScrollCursor(d)
		d.MarkDraw(c.edit)
		d.Call <- func() { d.Draw() }
	}
}

type application struct {
	chanlist  *duit.List
	channels  map[string]*Channel
	current   *Channel
	chanPanel *duit.Box
	status    *duit.Field
	d         *duit.DUI
	ctl       io.ReadWriteCloser
}

func (a *application) processInput(msg string) {
	channel := a.current.name

	if strings.HasPrefix(msg, "/") {
		// We have a command
		ps := strings.Split(msg, " ")
		parts := ps[:0]
		for _, s := range ps {
			if s != "" {
				parts = append(parts, s)
			}
		}
		switch parts[0] {
		case "/j":
			log.Printf("JOIN %s", parts[1])
			a.join(parts[1])
		case "/q":
			log.Printf("QUIT")
		case "/p":
			log.Printf("PART")
		case "/n":
			log.Printf("NICK %s", parts[1])
			a.nick(parts[1])
		default:
			log.Printf("UNKNOWN: [%#v]", parts)
		}
		return
	}

	if channel == "log" || channel == "raw" {
		return
	}

	err := a.msg(channel, msg)
	if err != nil {
		log.Printf("Error sending message: %s", err)
		err = a.nopenCtl("/mnt/9irc/ctl")
		if err == nil {
			a.msg(channel, msg)
		}
		a.setStatus()
	}
}

func (a *application) newChannel(dir, name string) (*Channel, error) {
	rf, err := os.Open(dir + "/" + name)
	if err != nil {
		return nil, err
	}
	var msg *duit.Field
	msg = &duit.Field{
		Placeholder: "Message",
		Keys: func(k rune, m draw.Mouse) (e duit.Event) {
			if k == '\n' {
				a.processInput(msg.Text)
				msg.Text = ""
				e.Consumed = true
				a.d.MarkDraw(msg)
			}
			return
		},
	}
	edit := &duit.Edit{}
	c := &Channel{
		name: name,
		f:    rf,
		msg:  msg,
		edit: edit,
		kids: duit.NewKids(msg, edit),
	}
	return c, nil
}

func (a *application) makeChannels(dir string) (map[string]*Channel, error) {
	infos, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	channels := make(map[string]*Channel)
	for i := range infos {
		if infos[i].Name() == "ctl" {
			continue
		}
		channel, err := a.newChannel(dir, infos[i].Name())
		if err != nil {
			return nil, err
		}
		channels[infos[i].Name()] = channel
	}
	return channels, nil
}

func (a *application) msg(channel, msg string) error {
	return a.send(fmt.Sprintf("msg %s %s\n", channel, msg))
}

func (a *application) nick(nick string) error {
	return a.send(fmt.Sprintf("nick %s\n", nick))
}

func (a *application) join(channel string) error {
	return a.send(fmt.Sprintf("join %s\n", channel))
}

func (a *application) send(raw string) error {
	if a.ctl == nil {
		return fmt.Errorf("Cannot send. Disconnected from 9irc.")
	}
	_, err := a.ctl.Write([]byte(raw))
	if err != nil {
		return err
	}
	return nil
}

func (a *application) setStatus() {
	a.status.Text = "Reading /tmp/9irc"
	if a.ctl != nil {
		a.status.Text += " (connected)"
	} else {
		a.status.Text += " (disconnected)"
	}
	a.d.MarkDraw(a.status)
}

func (a *application) loadDisplays(dir string) {
	channels, err := a.makeChannels(dir)
	if err != nil {
		log.Fatal(err)
	}
	a.channels = channels

	a.chanlist = &duit.List{
		Changed: func(index int) (e duit.Event) {
			name := a.chanlist.Values[index].Text
			channel := a.channels[name]
			log.Printf("Switching to channel %s (%#v)", name, channel)
			a.chanPanel.Kids = channel.kids
			a.current = channel
			a.d.MarkLayout(a.d.Top.UI)
			return
		},
	}
	for k := range a.channels {
		a.chanlist.Values = append(a.chanlist.Values, &duit.ListValue{Text: k})
	}
}

// This is really wasteful and bad.
func (a *application) reloadDisplays(dir string) {
	channels, err := a.makeChannels(dir)
	if err != nil {
		log.Fatal(err)
	}
	a.channels = channels
	a.chanlist.Values = nil
	for k := range a.channels {
		a.chanlist.Values = append(a.chanlist.Values, &duit.ListValue{Text: k})
	}
}

func NewApplication(dir string) (*application, error) {
	var a application
	d, err := duit.NewDUI("duitchat", nil)
	if err != nil {
		return nil, err
	}

	a.d = d
	a.loadDisplays(dir)
	raw := a.channels["raw"]
	if raw == nil {
		return nil, errors.New("No raw file present.")
	}
	chanPanel := &duit.Box{
		Width:   0,
		Reverse: true,
		Kids:    raw.kids,
	}
	a.current = raw
	a.chanPanel = chanPanel
	a.status = &duit.Field{
		Keys: func(k rune, m draw.Mouse) (e duit.Event) {
			e.Consumed = true
			return
		},
	}

	a.d.Top.UI = &duit.Box{
		Width: -1,
		Kids: duit.NewKids(
			a.status,
			&duit.Grid{
				Columns: 2,
				Kids: duit.NewKids(
					&duit.Box{
						Width: 150,
						Kids:  duit.NewKids(a.chanlist),
					},
					a.chanPanel,
				),
			},
		),
	}

	for _, c := range a.channels {
		go c.follow(a.d)
	}

	return &a, nil
}

func (a *application) nopenCtl(fname string) error {
	if a.ctl != nil {
		a.ctl.Close()
		a.ctl = nil
	}
	c, err := os.OpenFile(fname, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	a.ctl = c
	return nil
}

func main() {
	a, err := NewApplication("/mnt/9irc")
	if err != nil {
		log.Fatal(err)
	}

	err = a.nopenCtl("/mnt/9irc/ctl")
	if err != nil {
		log.Fatal(err)
	}
	a.setStatus()

	a.d.Render()
	for {
		// where we listen on two channels
		select {
		case e := <-a.d.Inputs:
			// inputs are: mouse events, keyboard events, window resize events,
			// functions to call, recoverable errors
			if !(e.Type == duit.InputMouse || e.Type == duit.InputKey) {
				log.Printf("Event: %#v", e)
			}
			a.d.Input(e)

		case warn, ok := <-a.d.Error:
			// on window close (clicking the X in the top corner),
			// the channel is closed and the application should quit.
			// otherwise, err is a warning or recoverable error.
			if !ok {
				return
			}
			log.Printf("duit: %s\n", warn)
		}
	}
}
