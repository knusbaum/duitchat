package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"9fans.net/go/draw"
	"github.com/mjl-/duit"
)

func readChannel(d *duit.DUI, disp *mainDisplay) error {
	var (
		n   int
		ba  [4096]byte
		err error
	)
	bs := ba[:]
	for n, err = disp.f.Read(bs); err == nil; n, err = disp.f.Read(bs) {
		disp.edit.Append(bs[:n])
	}
	if err != io.EOF {
		return err
	}
	disp.edit.ScrollCursor(d)
	return nil
}

type mainDisplay struct {
	name string
	f    *os.File
	msg  *duit.Field
	edit *duit.Edit
	kids []*duit.Kid
}

func (a *application) processInput(msg string) {
	channel := a.current.name
	if channel == "log" || channel == "raw" {
		return
	}

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
	err := a.msg(channel, msg)
	if err != nil {
		log.Printf("Error sending message: %s", err)
		err = a.openCtl()
		if err == nil {
			a.msg(channel, msg)
		}
		a.setStatus()
	}
}

func (a *application) makeMainDisplay(name string) (*mainDisplay, error) {
	rf, err := os.Open("/tmp/9irc/" + name)
	if err != nil {
		return nil, err
	}
	var msg *duit.Field
	msg = &duit.Field{
		Placeholder: "Message",
		Keys: func(k rune, m draw.Mouse) (e duit.Event) {
			//log.Printf("KEYS: k [%#v], m [%#v]", k, m)
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
	kids := duit.NewKids(msg, edit)
	return &mainDisplay{
		name: name,
		f:    rf,
		msg:  msg,
		edit: edit,
		kids: kids,
	}, nil
}

func (a *application) makeDisplays(dir string) (map[string]*mainDisplay, error) {
	infos, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	displays := make(map[string]*mainDisplay)
	for i := range infos {
		if infos[i].Name() == "ctl" {
			continue
		}
		display, err := a.makeMainDisplay(infos[i].Name())
		if err != nil {
			return nil, err
		}
		err = readChannel(a.d, display)
		if err != nil {
			return nil, err
		}
		displays[infos[i].Name()] = display
	}
	return displays, nil
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

type mainBox struct {
	duit.Box
	display *mainDisplay
}

type application struct {
	chanlist *duit.List
	displays map[string]*mainDisplay
	current  *mainDisplay
	mainbox  *duit.Box
	status   *duit.Field
	d        *duit.DUI
	ctl      io.ReadWriteCloser
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

func (a *application) loadDisplays() {
	edisplays, err := a.makeDisplays("/tmp/9irc")
	if err != nil {
		log.Fatal(err)
	}
	a.displays = edisplays

	a.chanlist = &duit.List{
		Changed: func(index int) (e duit.Event) {
			channel := a.chanlist.Values[index].Text
			display := a.displays[channel]
			log.Printf("Switching to channel %s (%#v)", channel, display)
			a.mainbox.Kids = display.kids
			a.current = display
			a.d.MarkLayout(a.d.Top.UI)
			return
		},
	}
	for k := range a.displays {
		a.chanlist.Values = append(a.chanlist.Values, &duit.ListValue{Text: k})
	}
}

// This is really wasteful and bad.
func (a *application) reloadDisplays() {
	edisplays, err := a.makeDisplays("/tmp/9irc")
	if err != nil {
		log.Fatal(err)
	}
	a.displays = edisplays

	a.chanlist.Values = nil
	for k := range a.displays {
		a.chanlist.Values = append(a.chanlist.Values, &duit.ListValue{Text: k})
	}
}

func main() {
	var a application
	ed, err := duit.NewDUI("test", nil)
	if err != nil {
		log.Fatal(err)
	}
	a.d = ed

	a.loadDisplays()
	eraw := a.displays["raw"]
	if eraw == nil {
		log.Fatal("No raw file present.")
	}
	emain := &duit.Box{
		Width:   0,
		Reverse: true,
		Kids:    eraw.kids,
	}
	a.current = eraw
	a.mainbox = emain
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
					a.mainbox,
				),
			},
		),
	}

	err = a.openCtl()
	if err != nil {
		log.Fatal(err)
	}
	a.setStatus()

	updates := make(chan string, 10)
	go watchDir("/tmp/9irc", updates)

	a.d.Render()
	for {
		// where we listen on two channels
		select {
		case u := <-updates:
			if display, ok := a.displays[u]; ok {
				err = readChannel(a.d, display)
				if err != nil {
					log.Fatal(err)
				}
				a.d.MarkDraw(display.edit)
				a.d.Draw()
			} else {
				a.reloadDisplays()
				a.d.MarkDraw(a.chanlist)
				a.d.Draw()
			}
		case e := <-a.d.Inputs:
			// inputs are: mouse events, keyboard events, window resize events,
			// functions to call, recoverable errors
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
