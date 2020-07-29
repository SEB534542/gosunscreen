package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	//"io"
	//"time"
)

type sunscreen struct {
	Mode     string
	Position string
}

const autom string = "autom"
const manual string = "manual"
const up string = "up"
const down string = "down"

var s1 *sunscreen = &sunscreen{Mode: autom, Position: up}

func main() {
	http.HandleFunc("/", mainHandler)
	http.HandleFunc("/mode/", modeHandler)
	http.HandleFunc("/config/", configHandler)
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}

func mainHandler(w http.ResponseWriter, res *http.Request) {
	t, _ := template.ParseFiles("index.gohtml")
	t.Execute(w, s1)
}

func modeHandler(w http.ResponseWriter, res *http.Request) {
	mode := res.URL.Path[len("/mode/"):]
	// fmt.Println(mode)
	switch mode {
	case autom:
		s1.Mode = autom
		fmt.Println("New Mode:", s1.Mode)
		fmt.Println("New Position:", s1.Position)
	case manual + "/" + up:
		s1.Mode = manual
		s1.Position = up
		fmt.Println("New Mode:", s1.Mode)
		fmt.Println("New Position:", s1.Position)
	case manual + "/" + down:
		s1.Mode = manual
		s1.Position = down
		fmt.Println("New Mode:", s1.Mode)
		fmt.Println("New Position:", s1.Position)
	default:
		fmt.Println(res.URL.Path)
		fmt.Println("Current Mode:", s1.Mode)
		fmt.Println("Current Position:", s1.Position)
	}
	http.Redirect(w, res, "/", http.StatusFound)
}

func configHandler(w http.ResponseWriter, res *http.Request) {
	t, _ := template.ParseFiles("config.gohtml")
	t.Execute(w, s1)
}