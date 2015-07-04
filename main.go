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
	"gopkg.in/igm/sockjs-go.v2/sockjs"
)

type ChangeEvent int

const (
	DELETE ChangeEvent = 0
	WRITE  ChangeEvent = 1
)

type Changes map[string]ChangeEvent

var port string
var folderPath string

func main() {
	port = ""
	port = os.Getenv("PORT")
	if port == "" {
		port = "9090"
	}
	log.Println("port", port)

	folderPath = "./"

	if len(os.Args) > 1 {
		folderPath = os.Args[1]
	}

	serverAddresses := []string{"http://localhost.:" + port}

	if len(os.Args) > 3 {
		if os.Args[2] == "on" {
			serverAddresses = append(serverAddresses, os.Args[3])
		}
	}

	mirror(serverAddresses)

}
func checkLastModified(w http.ResponseWriter, r *http.Request, modtime time.Time) bool {
	if modtime.IsZero() {
		return false
	}

	// The Date-Modified header truncates sub-second precision, so
	// use mtime < t+1s instead of mtime <= t to check for unmodified.
	if t, err := time.Parse(http.TimeFormat, r.Header.Get("If-Modified-Since")); err == nil && modtime.Before(t.Add(1*time.Second)) {
		h := w.Header()
		delete(h, "Content-Type")
		delete(h, "Content-Length")
		w.WriteHeader(http.StatusNotModified)
		return true
	}
	w.Header().Set("Last-Modified", modtime.UTC().Format(http.TimeFormat))
	return false
}

type InMemoryFile struct {
	Bytes []byte
	Time  time.Time
}

func serve() {
	log.Println("serving...")

	memory := make(map[string]InMemoryFile)

	var sessions []sockjs.Session
	http.Handle("/_sockjs/", sockjs.NewHandler("/_sockjs", sockjs.DefaultOptions, func(session sockjs.Session) {
		log.Println("SOCKJS : getting session", session)
		sessions = append(sessions, session)
		// for {
		// 	if msg, err := session.Recv(); err == nil {
		// 		session.Send(msg)
		// 		continue
		// 	}
		// 	break
		// }
	}))

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

			memory[path] = InMemoryFile{Bytes: fileBytes, Time: time.Now()}
			for _, session := range sessions {
				session.Send("write:" + path)
			}

		} else if r.Method == "GET" {
			log.Println("Getting...")
			value, ok := memory[path]
			if ok {
				if !checkLastModified(w, r, value.Time) { //TODO use the time
					w.Write(value.Bytes)
				}

			} else {
				w.WriteHeader(404)
				fmt.Fprint(w, "not found : "+r.URL.String())
			}

		} else if r.Method == "DELETE" {
			log.Println("Deleting...")
			delete(memory, path)
			for _, session := range sessions {
				session.Send("delete:" + path)
			}
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
		for k := range changes { //TODO paralelize
			sendChange(k, changes[k], serverAddresses)
		}
		time.Sleep(1500 * time.Millisecond)
	}
}

func newfileUploadRequest(url, paramName, path string) (*http.Request, error) {
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
	log.Println("new request on ", url)
	req, reqErr := http.NewRequest("PUT", url, body)
	if reqErr == nil {
		req.Header.Add("Content-Type", contentType)
	}
	return req, reqErr
}

func sendChange(path string, change ChangeEvent, serverAddresses []string) {
	path = filepath.ToSlash(path)
	serverPath, _ := filepath.Rel(folderPath, path)
	serverPath = filepath.ToSlash(serverPath)
	log.Println("sending change ", path, change)

	for _, serverAddress := range serverAddresses { //TODO parralelize
		url := serverAddress + "/" + serverPath
		if change == WRITE {
			sendFileToServer(url, path)
		} else if change == DELETE {
			removeFileFromServer(url, path)
		}

	}

}

func removeFileFromServer(url, path string) {
	req, reqErr := http.NewRequest("DELETE", url, nil)
	if reqErr != nil {
		log.Println("error req", reqErr)
	}
	client := &http.Client{}
	_, err := client.Do(req)
	if err != nil {
		log.Println("request sending error", err)
	}
}

func sendFileToServer(url, path string) {
	request, err := newfileUploadRequest(url, "file", path)
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

func mirror(serverAddresses []string) {
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
		skip := false
		stat, statErr := os.Stat(event.Name)
		if statErr == nil {
			if stat.IsDir() { //TODO check sub folder and files : are they trigering events o delete from a parent
				skip = true
				log.Println("skip Folder")
			}
		} else {
			log.Println(statErr)
		}
		if !skip {
			if event.Op == fsnotify.Write || event.Op == fsnotify.Create {
				(changes)[event.Name] = WRITE
			} else if event.Op == fsnotify.Remove || event.Op == fsnotify.Rename {
				(changes)[event.Name] = DELETE //TODO make rename more efficient by dealign with it specifically
			}
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
