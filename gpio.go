package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/stianeikeland/go-rpio"
)

var pin rpio.Pin = rpio.Pin(23)

func main() {

	f, err := os.Create("go-log.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	log.SetOutput(f)

	fmt.Println("Testing GPIO")
	rpio.Open()
	defer rpio.Close()

	for {
		getLight()
		time.Sleep(time.Second)
	}
}
