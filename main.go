package main

import (
	"log"
	"os"

	//"github.com/kardianos/osext"
	"gopkg.in/fsnotify.v1"
)

func main() {
	// folderPath, err := osext.ExecutableFolder()
	// if err != nil {
	// 	log.Fatal(err)
	// }

	folderPath := "./"

	command := ""
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	if command == "serve" {

	} else if command != "" {
		folderPath = command
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
	log.Println("done")
}
