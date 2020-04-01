// +build !plan9

package main

import (
	"github.com/fsnotify/fsnotify"
)

func watchDir(dir string, updates chan<- string) {
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
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("error:", err)
		}
	}
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
