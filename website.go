package main

import (
	"html/template"
	"log"
	"net/http"
	"time"
	//"fmt"
	//"io"
)

type sunscreen struct {
	Mode     string
	Position string
}

const autom string = "autom"
const manual string = "manual"
const up string = "up"
const down string = "down"

var Sunrise = "10:00"
var Sunset = "18:45"

var tpl *template.Template
var s1 *sunscreen = &sunscreen{Mode: autom, Position: up}

func init() {
	tpl = template.Must(template.ParseGlob("templates/*"))
}

func main() {
	http.HandleFunc("/", mainHandler)
	http.HandleFunc("/mode/", modeHandler)
	http.HandleFunc("/config/", configHandler)
	log.Fatal(http.ListenAndServe("0.0.0.0:8081", nil))
}

func mainHandler(w http.ResponseWriter, req *http.Request) {
	data := struct {
		*sunscreen
		Time string
	}{
		s1,
		time.Now().Format("_2 Jan 06 15:04:05"),
	}

	err := tpl.ExecuteTemplate(w, "index.gohtml", data)
	if err != nil {
		log.Fatalln(err)
	}
}

func modeHandler(w http.ResponseWriter, req *http.Request) {
	mode := req.URL.Path[len("/mode/"):]
	// fmt.Println(mode)
	switch mode {
	case autom:
		s1.Mode = autom
		log.Println("Updated Mode:", s1.Mode, "and Position:", s1.Position)
	case manual + "/" + up:
		s1.Mode = manual
		s1.Position = up
		log.Println("Updated Mode:", s1.Mode, "and Position:", s1.Position)
	case manual + "/" + down:
		s1.Mode = manual
		s1.Position = down
		log.Println("Updated Mode:", s1.Mode, "and Position:", s1.Position)
	default:
		log.Println("Unknown mode:", req.URL.Path)
		log.Println("Current Mode:", s1.Mode, "// Current Position:", s1.Position)
	}
	http.Redirect(w, req, "/", http.StatusFound)
}

func configHandler(w http.ResponseWriter, req *http.Request) {
	err := req.ParseForm()
	if err != nil {
		log.Fatalln(err)
	}

	if len(req.PostForm) != 0 {
		Sunrise = req.PostForm["Sunrise"][0]
		Sunset = req.PostForm["Sunset"][0]
		log.Println("Updated Sunrise:", Sunrise, "and Sunset:", Sunset)
	}

	data := struct {
		Sunrise string
		Sunset  string
	}{
		Sunrise,
		Sunset,
	}
	err = tpl.ExecuteTemplate(w, "config.gohtml", data)
	if err != nil {
		log.Fatalln(err)
	}
}
