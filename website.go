package website

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"io"
	//"time"
)

type sunscreen struct {
	Mode     string
	Position string
}

const autom string = "autom"
const manual string = "manual"
const up string = "Up"
const down string = "Down"

var s1 *sunscreen = &sunscreen{Mode: autom, Position: up}

func main() {
	http.HandleFunc("/", mainHandler)
	http.HandleFunc("/mode/", modeHandler)
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles("index.gohtml")
	t.Execute(w, s1)
}

func modeHandler(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Path[len("/mode/"):]
	// fmt.Println(mode)
	switch mode {
	case autom:
		fmt.Println("Set mode to auto")
		s1.Mode = autom
	case manual + "/" + up:
		fmt.Println("Set mode to manual and move sunscreen up")
		s1.Mode = manual
		s1.Position = up
	case manual + "/" + down:
		fmt.Println("Set mode to manual and move sunscreen down")
		s1.Mode = manual
		s1.Position = down
	default:
		fmt.Println(r.URL.Path)
		fmt.Println("\nTest\n")
		fmt.Println(mode + "\n")
	}
	http.Redirect(w, r, "/", http.StatusFound)
}
