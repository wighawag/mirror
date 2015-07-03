package main

import (
	"fmt"
	"html"
	"log"
	"net/http"
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

type Changes map[string]ChangeEvent

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
	log.Println("serving...")
	port := os.Getenv("PORT")
	if port == "" {
		port = "9090"
	}

	http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello : %q", html.EscapeString(r.URL.Path))
	}))

	httpErr := http.ListenAndServe(":"+port, nil)
	if httpErr != nil {
		log.Fatal("ListenAndServe: ", httpErr)
	}
}

func coalesce(in <-chan fsnotify.Event, out chan<- Changes, merge func(Changes, fsnotify.Event), period time.Duration) {
	changes := make(Changes)
	timer := time.NewTimer(0)

	var timerCh <-chan time.Time
	var outCh chan<- Changes

	for {
		log.Println("changes", changes)
		select {
		case e := <-in:
			log.Println("receiving")
			merge(changes, e)
			if timerCh == nil {
				timer.Reset(period * time.Millisecond)
				timerCh = timer.C
			}
		case <-timerCh:
			log.Println("ticking")
			outCh = out
			timerCh = nil
		case outCh <- changes:
			log.Println("changes outputed")
			changes = make(Changes)
			outCh = nil
		}
	}
}

func slowReceive(in <-chan Changes) {
	for {
		beforemap := <-in
		log.Println("beforemap:", beforemap)
		for k := range beforemap {
			log.Println(k, beforemap[k])
		}
		time.Sleep(1500 * time.Millisecond)

		log.Println("aftermap:", beforemap)
		for k := range beforemap {
			log.Println(k, beforemap[k])
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

	err = watcher.Add(folderPath)
	if err != nil {
		log.Fatal(err)
	}

	output := make(chan Changes)
	go coalesce(watcher.Events, output, func(changes Changes, event fsnotify.Event) {
		log.Println("coalescing", event)
		if event.Op == fsnotify.Write || event.Op == fsnotify.Create {
			(changes)[event.Name] = WRITE
		} else if event.Op == fsnotify.Remove || event.Op == fsnotify.Rename {
			(changes)[event.Name] = DELETE //TODO make rename more efficient by dealign with it specifically
		}

	}, 500)

	go slowReceive(output)

	go func() {
		for {

			err := <-watcher.Errors
			log.Println("error:", err)
		}
	}()

	<-done

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
