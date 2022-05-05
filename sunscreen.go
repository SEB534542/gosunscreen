package main

import (
	"fmt"
	"log"
	"time"

	"github.com/stianeikeland/go-rpio/v4"
)

// Constants for sunscreen position
const (
	unknown = "unknown"
	up      = "up"
	down    = "down"
	moving  = "moving"
)

// Constants for suncreen mode
const (
	auto   = "auto"
	manual = "manual"
)

// Sunscreen represents a physical Sunscreen that can be controlled through 2 GPIO pins: one for moving it up, and one for moving it down.
type Sunscreen struct {
	// TODO: remove ID and name?
	Id        int           // Autogenerated ID for sunscreen
	Name      string        // Name of sunscreen
	Mode      string        // Mode of Sunscreen auto or manual
	Position  string        // Current position of Sunscreen
	DurDown   time.Duration // Duration to move Sunscreen down
	DurUp     time.Duration // Duration to move Sunscreen up
	PinDown   rpio.Pin      // GPIO pin for moving sunscreen down
	PinUp     rpio.Pin      // GPIO pin for moving sunscreen up
	AutoStart bool          // If true, Start is calculated based on config.Location.GetSunriseSunset() and SunStart
	AutoStop  bool          // If true, Stop is calculated based on config.Location.GetSunriseSunset() and SunStop
	SunStart  time.Duration // Duration after Sunrise to determine Start
	SunStop   time.Duration // Duration after before Sunset to determine Stop
	Start     time.Time     // Time after which Sunscreen can shine on the Sunscreen area
	Stop      time.Time     // Time after which Sunscreen no can shine on the Sunscreen area
	StopLimit time.Duration // Duration before Stop that Sunscreen no longer should go down
}

func move(pin rpio.Pin, dur time.Duration) {
	pin.Low()
	n := time.Now()
	for time.Now().Before(n.Add(dur)) {
		time.Sleep(time.Second)
	}
	pin.High()
}

// Init initiates the sunscreen
func (s *Sunscreen) initPins() {
	s.PinDown.Output()
	s.PinDown.High()
	s.PinUp.Output()
	s.PinUp.High()
	updateStartStop(s, ls, 0)
}

// Move moves the suncreen up or down based on the Sunscreen.Position. It updates the position accordingly.
func (s *Sunscreen) Move() {
	old := s.Position
	switch s.Position {
	case unknown, down:
		new := up
		log.Printf("Moving sunscreen from %v to %v", old, new)
		s.Position = moving
		move(s.PinUp, s.DurUp)
		s.Position = new
	case up:
		new := down
		log.Printf("Moving sunscreen from %v to %v", old, new)
		s.Position = moving
		move(s.PinDown, s.DurDown)
		s.Position = new
	case moving:
		log.Printf("Sunscreen is moving already, do nothing")
	default:
		log.Fatalf("Unknown sunscreen position: '%v'", s.Position)
	}
	// TODO: Configure send mail
	// new := s.Position
	// mode := s.Mode
	// sendMail("Moved sunscreen "+new, fmt.Sprintf("Sunscreen moved from %s to %s.", old, new))
	// TODO: Store data in csv?
	// appendCSV(csvFile, [][]string{{time.Now().Format("02-01-2006 15:04:05"), mode, new, fmt.Sprint(site.LightSensor.Data)}})
	// SaveToJson(site, siteFile)
}

// Up checks if the suncreen's position is up. If not, it moves the suncreen up through method move().
func (s *Sunscreen) Up() {
	if s.Position != up {
		s.Move()
	}
}

// Down checks if s suncreen position is down. If not, it moves s suncreen down through method move().
func (s *Sunscreen) Down() {
	if s.Position != down {
		s.Move()
	}
}

func (s *Sunscreen) resetStartStop(d int) (err error) {
	resetDate := func(h, m, d int) time.Time {
		return time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), h, m, 0, 0, time.Now().Location()).AddDate(0, 0, d)
	}
	if s.AutoStart || s.AutoStop {
		err = s.resetAutoTime(d)
	}
	if !s.AutoStart {
		s.Start = resetDate(s.Start.Hour(), s.Start.Minute(), d)
	}
	if !s.AutoStop {
		s.Stop = resetDate(s.Stop.Hour(), s.Stop.Minute(), d)
	}
	return
}

func (s *Sunscreen) resetAutoTime(d int) (err error) {
	var start, stop time.Time
	if s.AutoStart || s.AutoStop {
		config.Location.Date = time.Now().AddDate(0, 0, d)
		start, stop, err = config.Location.GetSunriseSunset()
		if err != nil {
			err = fmt.Errorf("Could not determine sunrise ('%v') and sunset ('%v') for location '%v'. Please try again or set start/stop manually. Error:\n%v", start, stop, config.Location, err)
			return err
		}
	}
	if s.AutoStart {
		s.Start = start.Add(s.SunStart)
	}
	if s.AutoStop {
		s.Stop = stop.Add(-s.SunStop)
	}
	return nil
}

/* Evaluate checks the position of the Sunscreen against the gathered light and
parameters from the ligth sensor and moves the Sunscreen up or down if it
meets the criteria.*/
func (s *Sunscreen) evaluate(data []int, good, neutral, bad, timesGood, timesNeutral, timesBad, outliers int) {
	counter := 0
	switch s.Position {
	case up:
		for _, v := range data[:(timesGood + outliers)] {
			if v <= good {
				counter++
			}
		}
		if counter >= timesGood {
			s.Down()
			return
		}
	case down:
		for _, v := range data[:(timesBad + outliers)] {
			if v >= bad {
				counter++
			}
		}
		if counter >= timesBad {
			s.Up()
			return
		}
		counter = 0
		for _, v := range data[:(timesNeutral + outliers)] {
			if v >= neutral {
				counter++
			}
		}
		if counter >= timesNeutral {
			s.Up()
			return
		}
	}
}
