package main

import (
	"fmt"
	"github.com/stianeikeland/go-rpio"
	"log"
	"time"
)

// lowest measured value (neutral?): 150

var pin rpio.Pin = rpio.Pin(23)

func main() {
	fmt.Println("Testing GPIO")
	rpio.Open()
	defer rpio.Close()

	for {
		getLight()
		time.Sleep(time.Second)
	}
}

func getLight() {
	lightValues := []int{}
	for i := 0; i < 10; i++ {
		lightValues = append(lightValues, getLightValue())
	}
	log.Println("Current light value is:", calcAverage(lightValues...))
}

func getLightValue() int {
	count := 0
	// Output on the pin for 0.1 seconds
	pin.Output()
	pin.Low()
	time.Sleep(100 * time.Millisecond)

	// Change the pin back to input
	pin.Input()

	// Count until the pin goes high
	for pin.Read() == rpio.Low {
		count++
	}
	// log.Println("Current light value is:", count)
	return count
}

func calcAverage(xi ...int) int {
	total := 0
	for _, v := range xi {
		total = total + v
	}
	return total / len(xi)
}
