package main

import (
	"log"
	"time"

	"github.com/stianeikeland/go-rpio/v4"
)

const (
	lightFactor = 12
	freq        = 10
	lightSensor = rpio.Pin(23)
)

func main() {
	rpio.Open()
	defer rpio.Close()
	for {
		t := time.Now()
		if h := t.Hour(); h < 22 {
			value, err := getAvgLight(lightSensor, freq)
			if err != nil {
				log.Println(err)
			} else {
				log.Println("Light gathered:", value/lightFactor)
			}
			time.Sleep(60 * time.Second)
		} else {
			log.Printf("Current time %v is outside period", t.Format("15:04"))
			log.Println("About to exit")
			return
		}
	}
}
