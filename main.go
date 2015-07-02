package main

import (
	"log"
	"os"
	"time"

	//"github.com/kardianos/osext"
	"gopkg.in/fsnotify.v1"
)

type ChangeEvent int

const (
	DELETE ChangeEvent = 0
	WRITE  ChangeEvent = 1
)

func merge()

func main() {
	// folderPath, err := osext.ExecutableFolder()
	// if err != nil {
	// 	log.Fatal(err)
	// }

	command := "./"

	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	if command == "serve" {
		serve()
	} else if command != "" {
		mirror(command)
	}

}

func serve() {

}

// port := os.Getenv("PORT")
// if port == "" {
// 	port = "9090"
// }
//
// httpErr := http.ListenAndServe(":"+port, nil)
// if httpErr != nil {
// 	log.Fatal("ListenAndServe: ", httpErr)
// }

func coalesce(in <-chan fsnotify.Event, out chan<- fsnotify.Event, period int) {
	var event ChangeEvent
	timer := time.NewTimer(0)

	var timerCh <-chan time.Time
	var outCh chan<- Event

	for {
		select {
		case e := <-in:
			event = mergeFSEvent(event, e)
			if timerCh == nil {
				timer.Reset(period * time.Millisecond)
				timerCh = timer.C
			}
		case <-timerCh:
			outCh = out
			timerCh = nil
		case outCh <- event:
			event = NewEvent()
			outCh = nil
		}
	}
}

func mirror(folderPath string) {

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event := <-watcher.Events:
				log.Println("event:", event)
				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Println("modified file:", event.Name)
				}
			case err := <-watcher.Errors:
				log.Println("error:", err)
			}
		}
	}()

	err = watcher.Add(folderPath)
	if err != nil {
		log.Fatal(err)
	}
	<-done

}
