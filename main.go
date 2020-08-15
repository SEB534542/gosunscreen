// Package gosunscreen monitors light and moves the Sunscreen accordingly through GPIO
package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/stianeikeland/go-rpio"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"strconv"
	"sync"
	"time"
)

// Sunscreen represents a physical Sunscreen that can be controlled through 2 GPIO pins: one for moving it up, and one for moving it down.
type Sunscreen struct {
	Mode     string   // Mode of Sunscreen auto or manual
	Position string   // Current position of Sunscreen
	secDown  int      // Seconds to move Sunscreen down
	secUp    int      // Seconds to move Sunscreen up<<<<<<< HEAD
	pinDown  rpio.Pin // GPIO pin for moving sunscreen down
	pinUp    rpio.Pin // GPIO pin for moving sunscreen up
}

// LightSensor represents a physical lightsensor for which data can be collected through the corresponding GPIO pin.
type lightSensor struct {
	pinLight rpio.Pin // pin for retrieving light value
	data     []int    // collected light values
}

var config = struct {
	Sunrise               time.Time // Time after which Sunscreen can shine on the Sunscreen area
	Sunset                time.Time // Time after which Sunscreen no can shine on the Sunscreen area
	SunsetThreshold       int       // Minutes before sunset that Sunscreen no longer should go down
	Interval              int       // Interval for checking current light in seconds
	LightGoodValue        int       // Max measured light value that counts as "good weather"
	LightGoodThreshold    int       // Number of times light should be below lightGoodValue
	LightNeutralValue     int       // Max measured light value that counts as "neutral weather"
	LightNeutralThreshold int       // Number of times light should be above lightNeutralValue
	LightBadValue         int       // max measured light value that counts as "bad weather"
	LightBadThreshold     int       // number of times light should be above lightBadValue
	AllowedOutliers       int       // Number of outliers accepted in the measurement
	RefreshRate           int       // Number of seconds the main page should refresh
	EnableMail            bool      // Enable mail functionality
	MoveHistory           int       // Number of sunscreen movements to be shown
}{}

const up string = "up"
const down string = "down"
const unknown string = "unknown"
const auto string = "auto"
const manual string = "manual"
const configFile string = "config.json"
const csvFile string = "sunscreen_stats.csv"

var tpl *template.Template
var mu sync.Mutex
var fm = template.FuncMap{
	"fdateHM": hourMinute,
}

var ls1 = &lightSensor{
	pinLight: rpio.Pin(23),
	data:     []int{},
}
var s1 = &Sunscreen{
	Mode:     manual,
	Position: up,
	secDown:  17,
	secUp:    20,
	pinDown:  rpio.Pin(21),
	pinUp:    rpio.Pin(20),
}

// Move moves the suncreen up or down based on the Sunscreen.Position. It updates the position accordingly.
func (s *Sunscreen) Move() {
	old := s.Position
	mu.Lock()
	if s.Position != up {
		log.Printf("Sunscreen position is %v, moving sunscreen up...\n", s.Position)
		s.pinUp.Low()
		for i := 0; i <= s.secUp; i++ {
			time.Sleep(time.Second)
		}
		s.pinUp.High()
		s.Position = up
	} else {
		log.Printf("Sunscreen position is %v, moving sunscreen down...\n", s.Position)
		s.pinDown.Low()
		for i := 0; i <= s.secDown; i++ {
			time.Sleep(time.Second)
		}
		s.pinDown.High()
		s.Position = down
	}
	new := s.Position
	mode := s.Mode
	mu.Unlock()
	sendMail("Moved sunscreen "+s.Position, fmt.Sprint("Sunscreen moved from %s to %s", old, new))
	appendCSV(csvFile, [][]string{{time.Now().Format("02-01-2006 15:04:05 MST"), mode, old, new}})
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

// ReviewPosition reviews the position of the Sunscreen against the lightData and moves the Sunscreen up or down if it meets the criteria
func (s *Sunscreen) evalPosition(lightData []int) {
	counter := 0
	mu.Lock()
	switch s.Position {
	case up:
		log.Printf("Sunscreen is %v. Check if weather is good to go down\n", s.Position)
		for _, v := range lightData[:(config.LightGoodThreshold + config.AllowedOutliers)] {
			if v <= config.LightGoodValue {
				counter++
			}
		}
		if counter >= config.LightGoodThreshold {
			mu.Unlock()
			s.Move()
			return
		}
	case down:
		log.Printf("Sunscreen is %v. Check if it should go up\n", s.Position)
		for _, v := range lightData[:(config.LightBadThreshold + config.AllowedOutliers)] {
			if v >= config.LightBadValue {
				counter++
			}
		}
		if counter >= config.LightBadThreshold {
			mu.Unlock()
			s.Move()
			return
		}
		counter = 0
		for _, v := range lightData[:(config.LightNeutralThreshold + config.AllowedOutliers)] {
			if v >= config.LightNeutralValue {
				counter++
			}
		}
		if counter >= config.LightNeutralThreshold {
			mu.Unlock()
			s.Move()
			return
		}
	mu.Unlock()
	}
}

// GetCurrentLight collects the average input from the light sensor ls and returns the value as a slice of int
func (ls *lightSensor) GetCurrentLight() []int {
	lightValues := []int{}
	for i := 0; i < 10; i++ {
		lightValues = append(lightValues, ls.getLightValue())
	}
	return []int{calcAverage(lightValues...) / 31}
}

func (ls *lightSensor) getLightValue() int {
	count := 0
	// Output on the pin for 0.1 seconds
	ls.pinLight.Output()
	ls.pinLight.Low()
	time.Sleep(100 * time.Millisecond)

	// Change the pin back to input
	ls.pinLight.Input()

	// Count until the pin goes high
	for ls.pinLight.Read() == rpio.Low {
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

func (s *Sunscreen) autoSunscreen(ls *lightSensor) {
	for {
		mu.Lock()
		switch {
		case config.Sunset.Sub(time.Now()).Minutes() <= float64(config.SunsetThreshold) && config.Sunset.Sub(time.Now()).Minutes() > 0 && s.Position == up:
			log.Printf("Sun will set in (less then) %v min and Sunscreen is %v. Snoozing until sunset for %v seconds...\n", config.SunsetThreshold, s.Position, int(config.Sunset.Sub(time.Now()).Seconds()))
			for config.Sunset.Sub(time.Now()).Minutes() <= float64(config.SunsetThreshold) && config.Sunset.Sub(time.Now()).Minutes() > 0 {
				if s.Mode != auto {
					log.Println("Mode is no longer auto, closing auto func")
					return
				}
				time.Sleep(time.Second)
			}
			continue
		case time.Now().After(config.Sunset):
			log.Printf("Sun is down (%v), adjusting Sunrise/set", config.Sunset.Format("2 Jan 15:04 MST"))
			s.Up()
			config.Sunrise = config.Sunrise.AddDate(0, 0, 1)
			config.Sunset = config.Sunset.AddDate(0, 0, 1)
			continue
		case time.Now().Before(config.Sunrise):
			log.Printf("Sun is not yet up, snoozing until %v for %v seconds...\n", config.Sunrise.Format("2 Jan 15:04 MST"), int(config.Sunrise.Sub(time.Now()).Seconds()))
			s.Up()
			for i := 0; float64(i) <= config.Sunrise.Sub(time.Now()).Seconds(); i++ {
				if s.Mode != auto {
					log.Println("Mode is no longer auto, closing auto func")
					return
				}
				time.Sleep(time.Second)
			}
		case time.Now().After(config.Sunrise) && time.Now().Before(config.Sunset):
			//if there is enough light gathered in ls.data, evaluate position
			if maxLen := MaxIntSlice(config.LightGoodThreshold, config.LightBadThreshold, config.LightNeutralThreshold) + config.AllowedOutliers; len(ls.data) >= maxLen {
				mu.Unlock()
				s.evalPosition(ls.data)
				mu.Lock()
			} else {
				log.Println("Not enough light gathered...")
			}
		}
		interval := config.Interval
		mu.Unlock()
		log.Printf("Completed cycle, sleeping for %v second(s)...\n", interval)
		for i := 0; i < interval; i++ {
			mu.Lock()
			if s.Mode != auto {
				log.Println("Mode is no longer auto, closing auto func")
				return
			}
			mu.Unlock()
			time.Sleep(time.Second)
		}
	}
}

func (ls *lightSensor) monitorLight() {
	for {
		mu.Lock()
		if time.Now().After(config.Sunrise) && time.Now().Before(config.Sunset) {
			ls.data = append(ls.GetCurrentLight(), ls.data...)
			//ensure ls.data doesnt get too long
			if maxLen := MaxIntSlice(config.LightGoodThreshold, config.LightBadThreshold, config.LightNeutralThreshold) + config.AllowedOutliers; len(ls.data) > maxLen {
				ls.data = ls.data[:maxLen]
			}
		} else {
			// Sun is not up
			ls.data = []int{}
		}
		interval := config.Interval
		mu.Unlock()
		for i := 0; i < interval; i++ {
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
	rpio.Open()
	defer rpio.Close()
	for _, pin := range []rpio.Pin{s1.pinDown, s1.pinUp} {
		pin.Output()
		pin.High()
	}
	defer func() {
		log.Println("Closing down...")
		s1.Up()
	}()
	log.Printf("Sunrise: %v, Sunset: %v\n", config.Sunrise.Format("2 Jan 15:04 MST"), config.Sunset.Format("2 Jan 15:04 MST"))
	go ls1.monitorLight()
	log.Println("Launching website...")
	http.HandleFunc("/", mainHandler)
	http.HandleFunc("/mode/", modeHandler)
	http.HandleFunc("/config/", configHandler)
	log.Fatal(http.ListenAndServe("0.0.0.0:8081", nil))
}

func mainHandler(w http.ResponseWriter, req *http.Request) {
	stats := readCSV(csvFile)
	if len(stats) != 0 {
		stats = stats[MaxIntSlice(0, len(stats)-config.MoveHistory):]
	}
	mu.Lock()
	data := struct {
		*Sunscreen
		Time        string
		RefreshRate int
		Light       []int
		Stats       [][]string
		MoveHistory int
	}{
		s1,
		time.Now().Format("_2 Jan 06 15:04:05"),
		config.RefreshRate,
		ls1.data,
		stats,
		config.MoveHistory,
	}
	mu.Unlock()
	err := tpl.ExecuteTemplate(w, "index.gohtml", data)
	if err != nil {
		log.Fatalln(err)
	}
}

func modeHandler(w http.ResponseWriter, req *http.Request) {
	mode := req.URL.Path[len("/mode/"):]
	switch mode {
	case auto:
		mu.Lock()
		if s1.Mode == manual {
			go s1.autoSunscreen(ls1)
			s1.Mode = auto
			log.Printf("Set mode to auto (%v)\n", s1.Mode)
		} else {
			log.Printf("Mode is already auto (%v)\n", s1.Mode)
		}
		mu.Unlock()
	case manual + "/" + up:
		mu.Lock()
		s1.Mode = manual
		mu.Unlock()
		s1.Up()
	case manual + "/" + down:
		mu.Lock()
		s1.Mode = manual
		mu.Unlock()
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
		mu.Lock()
		config.Sunrise, err = StoTime(req.PostForm["Sunrise"][0], 0)
		if err != nil {
			log.Fatalln(err)
		}
		config.Sunset, err = StoTime(req.PostForm["Sunset"][0], 0)
		if err != nil {
			log.Fatalln(err)
		}
		config.SunsetThreshold, err = strToInt(req.PostForm["SunsetThreshold"][0])
		if err != nil {
			log.Fatalln(err)
		}
		config.Interval, err = strToInt(req.PostForm["Interval"][0])
		if err != nil {
			log.Fatalln(err)
		}
		config.LightGoodValue, err = strToInt(req.PostForm["LightGoodValue"][0])
		if err != nil {
			log.Fatalln(err)
		}
		config.LightGoodThreshold, err = strToInt(req.PostForm["LightGoodThreshold"][0])
		if err != nil {
			log.Fatalln(err)
		}
		config.LightNeutralValue, err = strToInt(req.PostForm["LightNeutralValue"][0])
		if err != nil {
			log.Fatalln(err)
		}
		config.LightNeutralThreshold, err = strToInt(req.PostForm["LightNeutralThreshold"][0])
		if err != nil {
			log.Fatalln(err)
		}
		config.LightBadValue, err = strToInt(req.PostForm["LightBadValue"][0])
		if err != nil {
			log.Fatalln(err)
		}
		config.LightBadThreshold, err = strToInt(req.PostForm["LightBadThreshold"][0])
		if err != nil {
			log.Fatalln(err)
		}
		config.AllowedOutliers, err = strToInt(req.PostForm["AllowedOutliers"][0])
		if err != nil {
			log.Fatalln(err)
		}
		config.RefreshRate, err = strToInt(req.PostForm["RefreshRate"][0])
		if err != nil {
			log.Fatalln(err)
		}
		if req.PostForm["EnableMail"] == nil {
			config.EnableMail = false
		} else {
			config.EnableMail, err = strconv.ParseBool(req.PostForm["EnableMail"][0])
			if err != nil {
				log.Fatalln(err)
			}
		}
		config.MoveHistory, err = strToInt(req.PostForm["MoveHistory"][0])
		if err != nil {
			log.Fatalln(err)
		}
		SaveToJson(config, configFile)
		mu.Unlock()
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

func hourMinute(t time.Time) string {
	return t.Format("15:04")
}

// SendMail sends mail to
func sendMail(subj, body string) {
	if config.EnableMail {
		to := []string{"raspberrych57@gmail.com"}

		//Format message
		var msgTo string
		for i, s := range to {
			if i != 0 {
				msgTo = msgTo + ","
			}
			msgTo = msgTo + s
		}

		msg := []byte("To:" + msgTo + "\r\n" +
			"Subject:" + subj + "\r\n" +
			"\r\n" + body + "\r\n")

		// Set up authentication information.
		auth := smtp.PlainAuth("", "raspberrych57@gmail.com", "Raspberrych4851", "smtp.gmail.com")

		// Connect to the server, authenticate, set the sender and recipient,
		// and send the email all in one step.
		err := smtp.SendMail("smtp.gmail.com:587", auth, "raspberrych57@gmail.com", to, msg)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func readCSV(file string) [][]string {
	// Read the file
	f, err := os.Open(file)
	if err != nil {
		f, err := os.Create(file)
		if err != nil {
			log.Fatal("Unable to create csv", err)
		}
		f.Close()
		return [][]string{}
	}
	defer f.Close()
	r := csv.NewReader(f)
	lines, err := r.ReadAll()
	if err != nil {
		log.Fatal(err)
	}
	return lines
}

func appendCSV(file string, newLines [][]string) {

	// Get current data
	lines := readCSV(file)

	// Add new lines
	lines = append(lines, newLines...)

	// Write the file
	f, err := os.Create(file)
	if err != nil {
		log.Fatal(err)
	}
	w := csv.NewWriter(f)
	if err = w.WriteAll(lines); err != nil {
		log.Fatal(err)
	}
}

// strToInt transforms string to an int and returns a positive int or zero
func strToInt(s string) (int, error) {
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	if i < 0 {
		return 0, err
	}
	return i, err
}
