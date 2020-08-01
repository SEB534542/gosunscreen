// Package gosunscreen monitors light and moves the Sunscreen accordingly through GPIO
package main

import (
	"encoding/json"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// https://pkg.go.dev/github.com/stianeikeland/go-rpio/v4?tab=doc
// import "github.com/stianeikeland/go-rpio/v4"

var config = struct {
	Sunrise               time.Time // Time after which Sunscreen can shine on the Sunscreen area
	Sunset                time.Time // Time after which Sunscreen no can shine on the Sunscreen area
	SunsetThreshold       int       // minutes before sunset that Sunscreen no longer should go down
	Interval              int       // interval for checking current light in seconds
	LightGoodValue        int       // max measured light value that counts as "good weather"
	LightGoodThreshold    int       // number of times light should be below lightGoodValue
	LightNeutralValue     int       // max measured light value that counts as "neutral weather"
	LightNeutralThreshold int       // number of times light should be above lightNeutralValue
	LightBadValue         int       // max measured light value that counts as "bad weather"
	LightBadThreshold     int       // number of times light should be above lightBadValue
	AllowedOutliers       int       // number of outliers accepted in the measurement
}{}

const up string = "up"
const down string = "down"
const unknown string = "unknown"
const auto string = "auto"
const manual string = "manual"
const configFile string = "config.json"

var tpl *template.Template
var mu sync.Mutex

var fm = template.FuncMap{
	"fdateHM": hourMinute,
}

var ls1 = &lightSensor{
	pinLight: 16,
	data:     []int{},
}

var s1 = &Sunscreen{
	Mode:     auto,
	Position: unknown,
	secDown:  17,
	secUp:    20,
	pinDown:  40,
	pinUp:    38,
}

func hourMinute(t time.Time) string {
	return t.Format("15:04")
}

// A Sunscreen represents a physical Sunscreen that can be controlled through 2 GPIO pins: one for moving it up, and one for moving it down.
type Sunscreen struct {
	Mode     string // Mode of Sunscreen auto or manual
	Position string // Current position of Sunscreen
	secDown  int    // Seconds to move Sunscreen down
	secUp    int    // Seconds to move Sunscreen up
	pinDown  int    // GPIO pin for moving Sunscreen down
	pinUp    int    // GPIO pin for moving Sunscreen up
}

// A LightSensor represents a physical lightsensor for which data can be collected through the corresponding GPIO pin.
type lightSensor struct {
	pinLight int   // pin for retrieving light value
	data     []int // collected light values
}

// Move moves the suncreen up or down based on the Sunscreen.Position. It updates the position accordingly.
func (s *Sunscreen) Move() {
	if s.Position != up {
		log.Printf("Sunscreen position is %v, moving Sunscreen up", s.Position)
		// TODO: move Sunscreen up
		// TODO: lock s.Position
		s.Position = up
	} else {
		log.Printf("Sunscreen position is %v, moving Sunscreen down", s.Position)
		// TODO: move Sunscreen down
		// TODO: lock s.Position
		s.Position = down
	}
}

// Up checks if the suncreen's position is up. If not, it moves the suncreen up through method move().
func (s *Sunscreen) Up() {
	if s.Position != up {
		s.Move()
	} else {
		log.Println("Sunscreen is already up...")
	}
}

// Down checks if s suncreen position is down. If not, it moves s suncreen down through method move().
func (s *Sunscreen) Down() {
	if s.Position != down {
		s.Move()
	} else {
		log.Println("Sunscreen is already down...")
	}
}

// ReviewPosition reviews the position of the Sunscreen against the lightData and moves the Sunscreen up or down if it meets the criteria
func (s *Sunscreen) reviewPosition(lightData []int) {
	counter := 0
	switch s.Position {
	case up:
		log.Printf("Sunscreen is %v. Check if weather is good to go down\n", s.Position)
		for _, v := range lightData[:(config.LightGoodThreshold + config.AllowedOutliers)] {
			if v <= config.LightGoodValue {
				counter++
			}
		}
		if counter >= config.LightGoodThreshold {
			s.Move()
			return
		}
	case down:
		log.Printf("Sunscreen is %v. Check if it should go up\n", s.Position)

		for _, v := range lightData[:(config.LightNeutralThreshold + config.AllowedOutliers)] {
			if v >= config.LightNeutralValue {
				counter++
			}
		}
		if counter >= config.LightNeutralThreshold {
			s.Move()
			return
		}

		for _, v := range lightData[:(config.LightBadThreshold + config.AllowedOutliers)] {
			if v >= config.LightBadValue {
				counter++
			}
		}
		if counter >= config.LightBadThreshold {
			s.Move()
			return
		}
	}
}

// GetCurrentLight collects the input from the light sensor ls and returns the value as a slice of int
func (ls *lightSensor) GetCurrentLight() []int {
	// TODO: measure light
	return []int{5}
}

func (s *Sunscreen) autoSunscreen(ls *lightSensor) {
	for {
		switch {
		// TODO: replace sub with after/before in all cases
		case config.Sunset.Sub(time.Now()).Minutes() <= float64(config.SunsetThreshold) && config.Sunset.Sub(time.Now()).Minutes() > 0 && s.Position == up:
			log.Printf("Sun will set in (less then) %v min and Sunscreen is %v. Snoozing until sunset for %v seconds...\n", config.SunsetThreshold, s.Position, int(config.Sunset.Sub(time.Now()).Seconds()))
			for i := 0; float64(i) <= config.Sunset.Sub(time.Now()).Seconds(); i++ {
				if s.Mode != auto {
					log.Println("Mode is no longer auto, closing auto func")
					return
				}
				time.Sleep(time.Second)
			}			
			fallthrough
		case config.Sunset.Sub(time.Now()) <= 0:
			log.Printf("Sun is down (%v), adjusting Sunrise/set to tomorrow", config.Sunset.Format("2 Jan 15:04 MST"))
			s.Up()
			config.Sunrise = config.Sunrise.AddDate(0, 0, 1)
			config.Sunset = config.Sunset.AddDate(0, 0, 1)
			fallthrough
		case config.Sunrise.Sub(time.Now()) > 0:
			log.Printf("Sun is not yet up, snoozing until %v for %v seconds...\n", config.Sunrise.Format("2 Jan 15:04 MST"), int(config.Sunrise.Sub(time.Now()).Seconds()))
			s.Up()			
			for i := 0; float64(i) <= config.Sunrise.Sub(time.Now()).Seconds(); i++ {
				if s.Mode != auto {
					log.Println("Mode is no longer auto, closing auto func")
					return
				}
				time.Sleep(time.Second)
			}			
			log.Printf("Sun is up")
			mu.Lock()
			ls.data = []int{}
			mu.Unlock()
			go ls.monitorLight()
		}
		maxLen := MaxIntSlice(config.LightGoodThreshold, config.LightBadThreshold, config.LightNeutralThreshold) + config.AllowedOutliers
		mu.Lock()
		if len(ls.data) > maxLen {
			ls.data = ls.data[:maxLen]
			s.reviewPosition(ls.data)
		}
		mu.Unlock()
		log.Printf("Completed cycle, sleeping for %v second(s)...\n", config.Interval)
		for i := 0; i < config.Interval; i++ {
			if s.Mode != auto {
				log.Println("Mode is no longer auto, closing auto func")
				return
			}
			time.Sleep(time.Second)
		}
	}
}

func (ls *lightSensor) monitorLight() {
	for {
		//TODO: rewrite that if within sunrise - sunset (using Before or After): add data, else ls.data = []int{}
		mu.Lock()
		ls.data = append(ls.GetCurrentLight(), ls.data...)
		mu.Unlock()
		if config.Sunset.Sub(time.Now()) <= 0 {
			log.Printf("Sun is down (%v), shutting down light", config.Sunset)
			return
		}
		for i := 0; i < config.Interval; i++ {
			time.Sleep(time.Second)
		}
	}
}

func init() {
	//Loading gohtml templates
	tpl = template.Must(template.New("").Funcs(fm).ParseGlob("templates/*"))

	//Loading config
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Fatal(err)
	}
	err = json.Unmarshal(data, &config)
	if err != nil {
		log.Fatal(err)
	}

	//Resetting Sunrise and Sunset to today
	config.Sunrise = time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), config.Sunrise.Hour(), config.Sunrise.Minute(), 0, 0, time.Now().Location())
	config.Sunset = time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), config.Sunset.Hour(), config.Sunset.Minute(), 0, 0, time.Now().Location())
}

func main() {
	log.Println("--------Start of program--------")
	log.Printf("Sunrise: %v, Sunset: %v\n", config.Sunrise.Format("2 Jan 15:04 MST"), config.Sunset.Format("2 Jan 15:04 MST"))
	go s1.Move()
	defer func() {
		log.Println("Closing down...")
		s1.Up()
	}()
	go ls1.monitorLight()
	go s1.autoSunscreen(ls1)

	http.HandleFunc("/", mainHandler)
	http.HandleFunc("/mode/", modeHandler)
	http.HandleFunc("/config/", configHandler)
	log.Fatal(http.ListenAndServe("0.0.0.0:8081", nil))
}

func mainHandler(w http.ResponseWriter, req *http.Request) {
	data := struct {
		*Sunscreen
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
	switch mode {
	case auto:
		if s1.Mode == manual {
			go s1.autoSunscreen(ls1)
			s1.Mode = auto
			log.Printf("Set mode to auto (%v)\n", s1.Mode) 
		} else {
			log.Printf("Mode is already auto (%v)\n", s1.Mode)
		}
	case manual + "/" + up:
		s1.Mode = manual
		s1.Up()
	case manual + "/" + down:
		s1.Mode = manual
		s1.Down()
	default:
		log.Println("Unknown mode:", req.URL.Path)
	}
	log.Println("Mode=", s1.Mode, "and Position=", s1.Position)
	http.Redirect(w, req, "/", http.StatusFound)
}

func configHandler(w http.ResponseWriter, req *http.Request) {
	err := req.ParseForm()
	if err != nil {
		log.Fatalln(err)
	}
	if len(req.PostForm) != 0 {
		config.Sunrise, err = StoTime(req.PostForm["Sunrise"][0], 0)
		if err != nil {
			log.Fatalln(err)
		}
		config.Sunset, err = StoTime(req.PostForm["Sunset"][0], 0)
		if err != nil {
			log.Fatalln(err)
		}
		config.SunsetThreshold, err = strconv.Atoi(req.PostForm["SunsetThreshold"][0])
		if err != nil {
			log.Fatalln(err)
		}
		config.Interval, err = strconv.Atoi(req.PostForm["Interval"][0])
		if err != nil {
			log.Fatalln(err)
		}
		config.LightGoodValue, err = strconv.Atoi(req.PostForm["LightGoodValue"][0])
		if err != nil {
			log.Fatalln(err)
		}
		config.LightGoodThreshold, err = strconv.Atoi(req.PostForm["LightGoodThreshold"][0])
		if err != nil {
			log.Fatalln(err)
		}
		config.LightNeutralValue, err = strconv.Atoi(req.PostForm["LightNeutralValue"][0])
		if err != nil {
			log.Fatalln(err)
		}
		config.LightNeutralThreshold, err = strconv.Atoi(req.PostForm["LightNeutralThreshold"][0])
		if err != nil {
			log.Fatalln(err)
		}
		config.LightBadValue, err = strconv.Atoi(req.PostForm["LightBadValue"][0])
		if err != nil {
			log.Fatalln(err)
		}
		config.LightBadThreshold, err = strconv.Atoi(req.PostForm["LightBadThreshold"][0])
		if err != nil {
			log.Fatalln(err)
		}
		config.AllowedOutliers, err = strconv.Atoi(req.PostForm["AllowedOutliers"][0])
		if err != nil {
			log.Fatalln(err)
		}
		SaveToJson(config, configFile)
		log.Println("Updated variables")
	}

	err = tpl.ExecuteTemplate(w, "config.gohtml", config)
	if err != nil {
		log.Fatalln(err)
	}
}

// StoTime receives a string of time (format hh:mm) and a day offset, and returns a type time with today's and the supplied hours and minutes + the offset in days
func StoTime(t string, days int) (time.Time, error) {
	timeNow := time.Now()

	timeHour, err := strconv.Atoi(t[:2])
	if err != nil {
		return time.Time{}, err
	}

	timeMinute, err := strconv.Atoi(t[3:])
	if err != nil {
		return time.Time{}, err
	}

	return time.Date(timeNow.Year(), timeNow.Month(), timeNow.Day()+days, int(timeHour), int(timeMinute), 0, 0, time.Local), nil
}

// MaxIntSlice receives variadic parameter of integers and return the highest integer
func MaxIntSlice(xi ...int) int {
	var max int
	for i, v := range xi {
		if i == 0 || v > max {
			max = v
		}
	}
	return max
}

// SaveToJson takes an interface and stores it into the filename
func SaveToJson(i interface{}, fileName string) {
	bs, err := json.Marshal(i)
	if err != nil {
		log.Fatal(err)
	}
	err = ioutil.WriteFile(fileName, bs, 0644)
	if err != nil {
		log.Fatal("Error", err)
	}
}
