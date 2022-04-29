package main

import (
	"log"
	"time"

	"github.com/stianeikeland/go-rpio/v4"
)

func main() {
	rpio.Open()
	defer rpio.Close()
	ls := &LightSensor{
		Pin:         rpio.Pin(23),
		Interval:    time.Duration(time.Minute),
		LightFactor: 12,
		Start:       time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 8, 0, 0, 0, time.Local),
		Stop:        time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 21, 0, 0, 0, time.Local),
		Data:        []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
	log.Println("Starting monitor") // TODO: remove(?)
	ls.Monitor()
}

// ReviewPosition reviews the position of the Sunscreen against the lightData and moves the Sunscreen up or down if it meets the criteria
func (s *Sunscreen) evalPosition(ls *LightSensor) {
	counter := 0
	switch s.Position {
	case up:
		//log.Printf("Sunscreen is %v. Check if weather is good to go down\n", s.Position)
		for _, v := range ls.Data[:(ls.LightGoodThreshold + ls.AllowedOutliers)] {
			if v <= ls.LightGoodValue {
				counter++
			}
		}
		if counter >= ls.LightGoodThreshold {
			s.Move()
			return
		}
	case down:
		//log.Printf("Sunscreen is %v. Check if it should go up\n", s.Position)
		for _, v := range ls.Data[:(ls.LightBadThreshold + ls.AllowedOutliers)] {
			if v >= ls.LightBadValue {
				counter++
			}
		}
		if counter >= ls.LightBadThreshold {
			s.Move()
			return
		}
		counter = 0
		for _, v := range ls.Data[:(ls.LightNeutralThreshold + ls.AllowedOutliers)] {
			if v >= ls.LightNeutralValue {
				counter++
			}
		}
		if counter >= ls.LightNeutralThreshold {
			s.Move()
			return
		}
	}
}
