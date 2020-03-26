package main

import (
	//"fmt"
	"log"
	"os"
	"strings"
	"io"

	"github.com/fsnotify/fsnotify"
	"github.com/mjl-/duit"
)

var e *duit.Edit

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
				file := strings.TrimPrefix(event.Name, dir + "/")
				updates<-file
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

func main() {
	d, err := duit.NewDUI("test", nil)
	if err != nil {
		log.Fatal(err)
	}
	//	x := &duit.List{
	//		Values: []*duit.ListValue{
	//			&duit.ListValue{Text: "a"},
	//			&duit.ListValue{Text: "b"},
	//			&duit.ListValue{Text: "c"},
	//		},
	//	}
	chanlist := &duit.List{}
	e = &duit.Edit{}

	d.Top.UI = &duit.Grid{
		Columns: 2,
		Kids: duit.NewKids(
			&duit.Box{
				Width: 150,
				Kids:  duit.NewKids(chanlist),
			},
			&duit.Box{
				Width:   0,
				Reverse: true,
				Kids: duit.NewKids(
					&duit.Field{Placeholder: "Message"},
					e,
				),
			},
		),
	}
	

	updates := make(chan string, 10)
	go watchDir("/tmp/9irc", updates)

	rf, err := os.Open("/tmp/9irc/raw")
	if err != nil {
		log.Fatal(err)
	}

	var (
		n int
		ba [4096]byte
	)
	bs := ba[:]
	for n, err = rf.Read(bs); err == nil; n, err = rf.Read(bs) {
		//fmt.Printf(string(bs[:n]))
		e.Append(bs[:n])
	}
	if err != io.EOF {
		log.Fatal(err)
	}
	e.ScrollCursor(d)
	d.Render()
	for {
		// where we listen on two channels
		select {
		case u := <-updates:
			log.Printf("Got Update!: [%s]\n", u)
			if u == "raw" {
				log.Println("Updating raw")
				for n, err = rf.Read(bs); err == nil; n, err = rf.Read(bs) {
					log.Printf("Append [%s]\n", string(bs[:n]))
					e.Append(bs[:n])
					
				}
				if err != io.EOF {
					log.Fatal(err)
				}
				//e.ScrollCursor(d)
				d.MarkDraw(e)
				d.Draw()
//					n, err = rf.Read(bs)
//					if err != nil {
//						log.Printf("Tried to read from raw, but got: %s\n", err)
//					} else {
//						fmt.Printf(string(bs[:n]))
//					}
			}
		case e := <-d.Inputs:
			// inputs are: mouse events, keyboard events, window resize events,
			// functions to call, recoverable errors
			d.Input(e)

		case warn, ok := <-d.Error:
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
