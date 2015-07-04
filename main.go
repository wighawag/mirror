package main

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
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

var port string

func main() {
	port = ""
	port = os.Getenv("PORT")
	if port == "" {
		port = "9090"
	}
	log.Println("port", port)

	path := "./"

	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	serverAddresses := []string{"http://localhost.:" + port}

	if len(os.Args) > 3 {
		if os.Args[2] == "on" {
			serverAddresses = append(serverAddresses, os.Args[3])
		}
	}

	mirror(path, serverAddresses)

}

func serve() {
	log.Println("serving...")

	memory := make(map[string][]byte)

	http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("handling response :  %q", html.EscapeString(r.URL.Path))
		//fmt.Fprintf(w, "Hello : %q", html.EscapeString(r.URL.Path))
		path := r.URL.Path
		if r.Method == "PUT" {
			log.Println("handling PUT")

			r.ParseMultipartForm(32 << 20)
			file, _, fileFormErr := r.FormFile("file")
			if fileFormErr != nil {
				log.Println("FileFormErr", fileFormErr)
				return
			}
			defer file.Close()

			fileBytes, readErr := ioutil.ReadAll(file)
			if readErr != nil {
				log.Println("read error", readErr)
			}
			log.Println("FileBytes", string(fileBytes))

			memory[path] = fileBytes

		} else if r.Method == "GET" {
			w.Write(memory[path])
		} else if r.Method == "DELETE" {
			delete(memory, path)
		} else {
			log.Println("NOT HANDLED")
		}
	}))

	go listen()

}

func listen() {
	httpErr := http.ListenAndServe(":"+port, nil)
	if httpErr != nil {
		log.Fatal("ListenAndServe: ", httpErr)
	}
	log.Println("done serving")
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

func sendCoalescedChanges(in <-chan Changes, serverAddresses []string) {
	for {
		changes := <-in
		log.Println("beforemap:", changes)
		for k := range changes {
			sendChange(k, changes[k], serverAddresses)
		}
		time.Sleep(1500 * time.Millisecond)
	}
}

func newfileUploadRequest(uri, paramName, path string) (*http.Request, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(paramName, filepath.Base(path))
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(part, file)

	err = writer.Close()
	if err != nil {
		return nil, err
	}
	contentType := writer.FormDataContentType()
	url := uri + "/" + path
	log.Println("new request on ", url)
	req, reqErr := http.NewRequest("PUT", url, body)
	if reqErr == nil {
		req.Header.Add("Content-Type", contentType)
	}
	return req, reqErr
}

func sendChange(path string, change ChangeEvent, serverAddresses []string) {
	path = filepath.ToSlash(path)
	log.Println("sending change ", path, change)
	for _, serverAddress := range serverAddresses { //TODO parralelize
		requestOnServer(serverAddress, path)
	}

}

func requestOnServer(serverAddress, path string) {
	request, err := newfileUploadRequest(serverAddress, "file", path)
	if err != nil {
		log.Fatal(err)
	}
	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		log.Fatal(err)
	} else {
		body := &bytes.Buffer{}
		_, err := body.ReadFrom(resp.Body)
		if err != nil {
			log.Fatal(err)
		}
		resp.Body.Close()
		log.Println(resp.StatusCode)
		log.Println(resp.Header)
		log.Println(body)
	}
}

func mirror(folderPath string, serverAddresses []string) {
	serve()

	done := make(chan bool)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	err = watcher.Add(folderPath)
	if err != nil {
		log.Fatal(err)
	}

	fileList := []string{}
	walkErr := filepath.Walk(folderPath, func(path string, f os.FileInfo, err error) error {
		if f.IsDir() {
			log.Println("adding ", path)
			watchErr := watcher.Add(path)
			if watchErr != nil {
				return watchErr
			}
		} else {
			sendChange(path, WRITE, serverAddresses)
		}
		return nil
	})

	if walkErr != nil {
		log.Fatal(walkErr)
	}

	for _, file := range fileList {
		fmt.Println(file)
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

	go sendCoalescedChanges(output, serverAddresses)

	// catch fsnotify errors
	go func() {
		for {
			err := <-watcher.Errors
			log.Println("error:", err)
		}
	}()

	<-done

}
