package main

import (
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/skratchdot/open-golang/open"
)

func in(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

var acceptedImageExt = []string{".jpg", ".jpeg"}
var dirThumbs = fmt.Sprintf("%s%s", os.Getenv("HOME"), "/.cache/lk")
var dirPath = "."
var gitVersion string
var showVersionFlag = flag.Bool("version", false, "Show version")
var port = flag.Int("port", 0, "listen port")

func hostname() string {
	hostname, _ := os.Hostname()
	// If hostname does not have dots (i.e. not fully qualified), then return zeroconf address for LAN browsing
	if strings.Split(hostname, ".")[0] == hostname {
		return hostname + ".local"
	}
	return hostname
}

func main() {

	flag.Parse()

	if *showVersionFlag {
		log.Println("lk", gitVersion, "https://github.com/kaihendry/lk")
		os.Exit(0)
	}

	directory := flag.Arg(0)
	dirPath, _ = filepath.Abs(directory)
	
	// Getting rid of /../ etc
	dirPath = path.Clean(dirPath)

	// Don't allow path under dirPath to be viewed
	http.Handle("/o/", http.StripPrefix(path.Join("/o", dirPath), http.FileServer(http.Dir(dirPath))))
	http.HandleFunc("/favicon.ico", http.NotFound)

	http.HandleFunc("/", lk)
	http.HandleFunc("/t/", thumb)

	// http://stackoverflow.com/a/33985208/4534
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Panic(err)
	}

	if a, ok := ln.Addr().(*net.TCPAddr); ok {
		host := fmt.Sprintf("http://%s:%d", hostname(), a.Port)
		log.Println("Serving from", host)
		open.Start(host)
	}
	if err := http.Serve(ln, nil); err != nil {
		log.Panic(err)
	}

}

func thumb(w http.ResponseWriter, r *http.Request) {

	// Path cleaning
	requestedPath := path.Clean(r.URL.Path[2:])

	// Make sure you can't go under the dirPath
	if !strings.HasPrefix(requestedPath, dirPath) {
		http.NotFound(w, r)
		return
	}

	thumbPath := filepath.Join(dirThumbs, requestedPath)
	if _, err := os.Stat(thumbPath); err != nil {
		log.Println("THUMB:", thumbPath, "does not exist")
		srcPath := requestedPath
		if _, err := os.Stat(srcPath); err != nil {
			log.Println("ORIGINAL", srcPath, "does not exist")
			http.NotFound(w, r)
			return
		} else {
			log.Println("Must generate thumb for", srcPath)
			err := genthumb(srcPath, thumbPath)
			if err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			log.Println("Created thumb", thumbPath)
		}
	}
	log.Println("Serving thumb", thumbPath)
	http.ServeFile(w, r, thumbPath)
}

func lk(w http.ResponseWriter, r *http.Request) {

	// log.Println("dirPath", dirPath, "Web path", r.URL.Path)
	srcPath := filepath.Join(dirPath, r.URL.Path)
	// log.Println("Combined", srcPath)

	files, err := ioutil.ReadDir(srcPath)
	// log.Println(files)

	if err != nil {
		panic(err)
	}

	dirs := []string{}
	images := []string{}

	for _, f := range files {

		if strings.HasPrefix(filepath.Base(f.Name()), ".") {
			continue
		}

		// log.Println("Filename", f.Name())
		if f.IsDir() {
			// log.Println(f.Name(), "is a directory")
			// TODO check they have a JPG in them?
			dirs = append(dirs, filepath.Join(r.URL.Path, f.Name()))
		}
		// Only append jpg images
		if in(acceptedImageExt, strings.ToLower(path.Ext(f.Name()))) {
			log.Printf("Appending %s", f.Name())
			images = append(images, filepath.Join(srcPath, f.Name()))
		}
	}

	t, err := template.New("foo").Parse(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8" />
<style>
body { padding: 5px; font-size: 120%; }
ol { display: flex-inline; padding: 0; }
li { flex: 1; display: flex; padding-bottom: 0.4em; }
li a { flex: 1; border: thin dotted black; text-decoration: none; padding: 0.3em; color: white; background-color: #0b5578; }
</style>
</head>
<body>
<ol>
{{ range .Dirs }}<li><a href="{{ . }}">{{ . }}</a></li>
{{ end }}
</ol>
{{ range .Images }}<a title="{{ . }}" href="/o{{ . }}"><img alt="" width=230 height=230 src="/t{{ . }}"></a>
{{ end }}
<p>By <a href=https://github.com/kaihendry/lk>lk {{ .Version }}</a></p>
</body>
</html>`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Images  []string
		Dirs    []string
		Version string
	}{
		images,
		dirs,
		gitVersion,
	}

	t.Execute(w, data)

	log.Printf("%s %s %s %s\n", r.RemoteAddr, r.Method, r.URL, r.UserAgent())

}
