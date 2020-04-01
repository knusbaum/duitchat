package main

import (
	"io"
	"io/ioutil"
	"log"
	"os"
	"syscall"
	"time"
)

type fstatus struct {
	qid   syscall.Qid
	mtime time.Time
}

func makeStatuses(dir string) map[string]fstatus {
	statuses := make(map[string]fstatus)
	info, err := os.Stat(dir)
	if err != nil {
		log.Fatal(err)
	}

	qid := info.Sys().(*syscall.Dir).Qid
	mtime := info.ModTime()
	statuses[dir] = fstatus{qid, mtime}

	infos, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}
	for _, info := range infos {
		qid := info.Sys().(*syscall.Dir).Qid
		mtime := info.ModTime()
		path := dir + "/" + info.Name()
		statuses[path] = fstatus{qid, mtime}
	}
	return statuses
}

func watchDir_old(dir string, updates chan<- string) {
	statuses := makeStatuses(dir)

	for {
		for path, status := range statuses {
			info, err := os.Stat(path)
			if err != nil {
				log.Fatal(err)
			}
			qid := info.Sys().(*syscall.Dir).Qid
			mtime := info.ModTime()
			if qid != status.qid {
				log.Printf("%s: QIDs differ.", path)
			}
			if mtime != status.mtime {
				log.Printf("%s: mtime differs.", path)
			}
			if qid != status.qid || mtime != status.mtime {
				log.Printf("Alerting that [%s] has changed.", info.Name())
				updates <- info.Name()
				if path == dir {
					log.Printf("Reloading dir.")
					statuses = makeStatuses(dir)
					break
				}
			}

			statuses[path] = fstatus{qid, mtime}
			time.Sleep(1 * time.Second)
		}
	}

}

// This version of watchDir reads from the raw file since that should
// always indicate when new data is available on other files.
func watchDir(dir string, updates chan<- string) {
	statuses := makeStatuses(dir)
	f, err := os.Open(dir + "/raw")
	bs := make([]byte, 1024)
	for _, err = f.Read(bs); err == nil; _, err = f.Read(bs) {
	}

	for {
		_, err := f.Read(bs)
		if err == nil {
			for path, status := range statuses {
				// Maybe don't stat each one, just do another ReadDir?
				info, err := os.Stat(path)
				if err != nil {
					log.Fatal(err)
				}
				qid := info.Sys().(*syscall.Dir).Qid
				mtime := info.ModTime()
				if qid != status.qid || mtime != status.mtime {
					updates <- info.Name()
					if path == dir {
						log.Printf("Reloading dir.")
						statuses = makeStatuses(dir)
						break
					}
				}
				statuses[path] = fstatus{qid, mtime}
			}
		} else if err != io.EOF {
			log.Fatal(err)
		}
		time.Sleep(1000 * time.Millisecond)
	}
}

func (a *application) openCtl() error {
	if a.ctl != nil {
		a.ctl.Close()
		a.ctl = nil
	}
	c, err := os.OpenFile("/srv/9irc", os.O_RDWR, 0)
	if err != nil {
		return err
	}
	a.ctl = c
	return nil
}
