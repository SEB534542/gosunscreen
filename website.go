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
		io.WriteString(w, "Set mode to auto")
		s1.Mode = autom
	case manual + "/" + up:
		io.WriteString(w, "Set mode to manual and move sunscreen up")
		s1.Mode = manual
		s1.Position = up
	case manual + "/" + down:
		io.WriteString(w, "Set mode to manual and move sunscreen down")
		s1.Mode = manual
		s1.Position = down
	default:
		io.WriteString(w, r.URL.Path)
		io.WriteString(w, "\nTest\n")
		io.WriteString(w, mode + "\n")
		fmt.Println(mode == "autom")
	}
	http.Redirect(w, r, "/", http.StatusFound)
}
