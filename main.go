package main

import (
	"flag"
	"io"
	"io/ioutil"
	"log"
	"os"
	"time"

	"9fans.net/go/draw"
	"github.com/mjl-/duit"
)

type Watched struct {
	f       *os.File
	display []*duit.Kid
	ctl     *os.File // may be nil.
}

type App struct {
	ui       *duit.DUI
	list     *duit.List // Value contains a *Watched
	main     *duit.Box
	shutdown chan struct{}
}

func (a *App) SignalDraw() {
	select {
	case a.ui.Call <- func() { a.ui.Draw() }:
	default:
	}
}

func (a *App) ListSelect(index int) (e duit.Event) {
	watched := a.list.Values[index].Value.(*Watched)
	a.main.Kids = watched.display
	a.ui.MarkLayout(a.ui.Top.UI)
	return
}

func NewApp() (*App, error) {
	var app App
	d, err := duit.NewDUI("duitchat", nil)
	if err != nil {
		return nil, err
	}
	app.ui = d
	app.list = &duit.List{
		Changed: func(index int) (e duit.Event) {
			return app.ListSelect(index)
		},
	}
	app.main = &duit.Box{
		Width:   0,
		Reverse: true,
	}
	app.shutdown = make(chan struct{})

	// TODO: switch top level to duit.Split
	app.ui.Top.UI = &duit.Grid{
		Columns: 2,
		Kids: duit.NewKids(
			&duit.Box{
				Width: 150,
				Kids:  duit.NewKids(app.list),
			},
			app.main,
		),
	}
	return &app, nil
}

func (a *App) follow(f *os.File, path string, e *duit.Edit) {
	defer f.Close()

	finfo, err := f.Stat()
	if err != nil {
		log.Printf("ERR Getting file stats: %s", err)
		return
	}
	fsize := finfo.Size()

	var ba [8192]byte
	bs := ba[:]
	for {
		n, err := f.Read(bs)
		if err != nil {
			if err != io.EOF {
				log.Printf("ERR reading %s", err)

				// Get offset.
				off, err := f.Seek(0, os.SEEK_CUR)
				if err != nil {
					log.Printf("ERR Determining stream offset.")
				}

				f, err = os.Open(path)
				if err != nil {
					log.Printf("ERR Reopening: %s", err)
					return
				}
				
				log.Printf("[follow %s] Seeking to %d", path, off)
				noff, err := f.Seek(off, os.SEEK_SET)
				if err != nil {
					log.Printf("ERR Seeking: %s", err)
				}

				if noff != off {
					log.Printf("Tried to seek to %d, but seeked to %d.", off, noff)
				}
			}
			time.Sleep(1 * time.Second)
			continue
		}
		e.Append(bs[:n])
		if e.Size() < fsize {
			log.Printf("SKIPPING REDRAW")
			continue
		}
		e.ScrollCursor(a.ui)
		a.ui.MarkDraw(e)
		a.SignalDraw()
		//a.ui.Call <- func() { a.ui.Draw() }
	}
}

func (a *App) hasWatched(name string) bool {
	for _, val := range a.list.Values {
		if val.Text == name {
			return true
		}
	}
	return false
}

func (a *App) addDir(dir string) error {
	msgs := make(chan Msg)
	ctlok := a.handleCtl(dir, msgs)

	go func() {
		for {
			infos, err := ioutil.ReadDir(dir)
			if err != nil {
				log.Printf("Error listing dir: %s", err)
				return
			}
			for _, info := range infos {
				name := info.Name()
				if name == "ctl" {
					continue
				}
				if a.hasWatched(name) {
					continue
				}

				f, err := os.Open(dir + "/" + name)
				if err != nil {
					log.Printf("Failed to open %s/%s: %s", dir, name, err)
					continue
				}

				edit := &duit.Edit{}
				go a.follow(f, dir + "/" + name, edit)

				uis := []duit.UI{edit}
				if ctlok {
					var msg *duit.Field
					msg = &duit.Field{
						Placeholder: dir + "/" + name,
						Keys: func(k rune, m draw.Mouse) (e duit.Event) {
							if k == '\n' {
								msgs <- Msg{from: name, msg: msg.Text}
								msg.Text = ""
								e.Consumed = true
								a.ui.MarkDraw(msg)
								//a.ui.Call <- func() { a.ui.Draw() }
								a.SignalDraw()
							}
							return
						},
					}
					uis = []duit.UI{msg, edit}
				}

				w := &Watched{
					display: duit.NewKids(uis...),
				}
				lv := &duit.ListValue{Text: name, Value: w}
				a.list.Values = append(a.list.Values, lv)
				a.ui.MarkDraw(a.list)
				a.ui.MarkLayout(a.list)
				//a.ui.Call <- func() { a.ui.Draw() }
				a.SignalDraw()
			}

			time.Sleep(2 * time.Second)
			//log.Printf("Adding dir again!")
		}
	}()
	return nil
}

func main() {
	dirFlag := flag.String("dir", "/mnt/9irc", "specifies the directory to watch.")
	flag.Parse()

	app, err := NewApp()
	if err != nil {
		log.Fatal(err)
	}

	err = app.addDir(*dirFlag)
	if err != nil {
		log.Fatal(err)
	}

	app.ui.Render()
	app.ui.Draw()
	for {
		// where we listen on two channels
		select {
		case e := <-app.ui.Inputs:
			// inputs are: mouse events, keyboard events, window resize events,
			// functions to call, recoverable errors
			app.ui.Input(e)

		case _, ok := <-app.ui.Error:
			// on window close (clicking the X in the top corner),
			// the channel is closed and the application should quit.
			// otherwise, err is a warning or recoverable error.
			if !ok {
				return
			}
			//log.Printf("duit: %s\n", warn)
		}
	}
}
