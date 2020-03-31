package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"
	//"time"

	"9fans.net/go/draw"
	"github.com/fsnotify/fsnotify"
	"github.com/mjl-/duit"
)

//var e *duit.Edit

func watchDir(dir string, updates chan<- string) {
	//	rf, err := os.Open("/tmp/9irc/raw")
	//	if err != nil {
	//		log.Fatal(err)
	//	}
	//
	//	var (
	//		n int
	//		ba [1024]byte
	//	)
	//	bs := ba[:]
	//	for n, err = rf.Read(bs); err == nil; n, err = rf.Read(bs) {
	//		fmt.Printf(string(bs[:n]))
	//	}
	//
	//	if err != io.EOF {
	//		log.Fatal(err)
	//	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	err = watcher.Add(dir)
	if err != nil {
		log.Fatal(err)
	}
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			log.Println("event:", event)
			if event.Op&fsnotify.Write == fsnotify.Write {
				log.Println("modified file:", event.Name)
				file := strings.TrimPrefix(event.Name, dir+"/")
				updates <- file
				//				if event.Name == "/tmp/9irc/raw" {
				//					n, err = rf.Read(bs)
				//					if err != nil {
				//						log.Printf("Tried to read from raw, but got: %s\n", err)
				//					} else {
				//						fmt.Printf(string(bs[:n]))
				//					}
				//				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("error:", err)
		}
	}
}

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
			//_, err = a.ctl.Write([]byte(fmt.Sprintf("msg %s %s\n", channel, msg)))
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

func (a *application) openCtl() error {
	if a.ctl != nil {
		a.ctl.Close()
		a.ctl = nil
	}
	c, err := net.Dial("unix", "/tmp/9irc/ctl")
	if err != nil {
		return err
	}
	a.ctl = c
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
	ctl      net.Conn
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

func main() {
	var a application
	ed, err := duit.NewDUI("test", nil)
	if err != nil {
		log.Fatal(err)
	}
	a.d = ed

	edisplays, err := a.makeDisplays("/tmp/9irc")
	if err != nil {
		log.Fatal(err)
	}
	a.displays = edisplays

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

	a.status = &duit.Field{Keys: func(k rune, m draw.Mouse) (e duit.Event) { e.Consumed = true; return }}

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
			log.Printf("Got Update!: [%s]\n", u)
			if display, ok := a.displays[u]; ok {
				err = readChannel(a.d, display)
				if err != nil {
					log.Fatal(err)
				}
				a.d.MarkDraw(display.edit)
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
