// Package gosunscreen monitors light and moves the Sunscreen accordingly through GPIO
package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SEB534542/seb"
	"github.com/satori/go.uuid"
	"github.com/stianeikeland/go-rpio/v4"
	"golang.org/x/crypto/bcrypt"
)

// TODO: 1 lightsensor met meerdere zonneschermen, waarbij een zonnescherm zn eigen begin en eindtijd heeft (en de lichtgegevens meet)

// LightSensor represents a physical lightsensor for which data can be collected through the corresponding GPIO pin.
type LightSensor struct {
	pinLight              rpio.Pin      // pin for retrieving light value
	Interval              time.Duration // Interval for checking current light in seconds
	LightGoodValue        int           // Max measured light value that counts as "good weather"
	LightGoodThreshold    int           // Number of times light should be below lightGoodValue
	LightNeutralValue     int           // Max measured light value that counts as "neutral weather"
	LightNeutralThreshold int           // Number of times light should be above lightNeutralValue
	LightBadValue         int           // max measured light value that counts as "bad weather"
	LightBadThreshold     int           // number of times light should be above lightBadValue
	AllowedOutliers       int           // Number of outliers accepted in the measurement
	data                  []int         // collected light values
}

// Sunscreen represents a physical Sunscreen that can be controlled through 2 GPIO pins: one for moving it up, and one for moving it down.
type Sunscreen struct {
	ID            string        // Name and/or ID of sunscreen
	Mode          string        // Mode of Sunscreen auto or manual
	Position      string        // Current position of Sunscreen
	timeDown      time.Duration // Seconds to move Sunscreen down
	timeUp        time.Duration // Seconds to move Sunscreen up
	pinDown       rpio.Pin      // GPIO pin for moving sunscreen down
	pinUp         rpio.Pin      // GPIO pin for moving sunscreen up
	Start         time.Time     // Time after which Sunscreen can shine on the Sunscreen area
	Stop          time.Time     // Time after which Sunscreen no can shine on the Sunscreen area
	StopThreshold time.Duration // Duration before Stop that Sunscreen no longer should go down
}

type Site struct {
	Sunscreens []*Sunscreen
	*LightSensor
}

type config struct {
	RefreshRate time.Duration // Number of seconds the main page should refresh
	EnableMail  bool          // Enable mail functionality
	MoveHistory int           // Number of sunscreen movements to be shown
	LogRecords  int           // Number of log records that are shown
	Notes       string        // Field to store comments/notes
	Username    string        // Username for logging in
	Password    []byte        // Password for logging in
	IpWhitelist []string      // Whitelisted IPs
	LightFactor int           // Factor for correcting the measured analog light value
	port        int
}

// General constants
const (
	up       = "up"
	down     = "down"
	unknown  = "unknown"
	auto     = "auto"
	manual   = "manual"
	maxCount = 9999999
)

// Constants for log folder and files
const (
	logFolder = "logs"
	logFile   = "logfile.log"
	csvFile   = "sunscreen_stats.csv"
	lightFile = "light_stats.csv"
)

// Constants for config folder and files
const (
	configFolder  = "config"
	configFile    = "config.json"
	sunscreenFile = "sunscreen.json"
)

var (
	tpl        *template.Template
	mu         sync.Mutex
	fm         = template.FuncMap{"fdateHM": hourMinute, "fsliceString": SliceToString}
	dbSessions = map[string]string{}
	site       Site
	config     Config
)

// // TODO: Remove ls1 en s1
// var s = &LightSensor{
// 	pinLight: rpio.Pin(23),
// 	data:     []int{},
// }
// var s1 = &Sunscreen{
// 	Mode:     manual,
// 	Position: up,
// 	secDown:  17,
// 	secUp:    20,
// 	pinDown:  rpio.Pin(21),
// 	pinUp:    rpio.Pin(20),
// }

func init() {
	//Loading gohtml templates
	tpl = template.Must(template.New("").Funcs(fm).ParseGlob("./templates/*"))

	// Check if log folder exists, else create
	if _, err := os.Stat(logFolder); os.IsNotExist(err) {
		os.Mkdir(logFolder, 4096)
	}
}

func main() {
	// Open logfile or create if not exists
	f, err := os.OpenFile("./"+logFolder+"/"+logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Panic("Error opening log file:", err)
	}
	defer f.Close()
	log.SetOutput(f)

	log.Println("--------Start of program--------")

	// Load general config
	err := seb.LoadConfig("./"+configFolder+"/"+configFile, &c)
	checkErr(err)
	if config.Port == 0 {
		config.Port = 8080
		log.Print("Unable to load port from %v, set port to %v", config.Port)
	}

	// Load site
	err = seb.LoadConfig("./"+configFolder+"/"+sunscreenFile, &site)

	//Resetting Start and Stop to today
	for _, s := range site.Sunscreens {
		s.Start = time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), s.Start.Hour(), s.Start.Minute(), 0, 0, time.Now().Location())
		s.Stop = time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), s.Stop.Hour(), s.Stop.Minute(), 0, 0, time.Now().Location())
		log.Printf("Start: %v, Stop: %v\n", s.Start.Format("2 Jan 15:04 MST"), s.Stop.Format("2 Jan 15:04 MST"))
	}

	// Connecting to rpio Pins
	rpio.Open()
	defer rpio.Close()

	// TODO: remove below(?) / rewrite for all
	for _, pin := range []rpio.Pin{s1.pinDown, s1.pinUp} {
		pin.Output()
		pin.High()
	}
	defer func() {
		log.Println("Closing down...")
		mu.Lock()
		s.Up()
		mu.Unlock()
	}()

	// Monitor light
	go site.Lightsensor.monitorLight()

	log.Printf("Launching website at localhost%v...", port)
	http.HandleFunc("/", mainHandler)
	http.Handle("/favicon.ico", http.NotFoundHandler())
	http.HandleFunc("/mode/", modeHandler)
	http.HandleFunc("/config/", configHandler)
	http.HandleFunc("/log/", logHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.HandleFunc("/light", lightHandler)
	err = http.ListenAndServeTLS(port, "cert.pem", "key.pem", nil)
	if err != nil {
		log.Println("ERROR: Unable to launch TLS, launching without TLS...")
		log.Fatal(http.ListenAndServe(port, nil))
	}
}

// Move moves the suncreen up or down based on the Sunscreen.Position. It updates the position accordingly.
func (s *Sunscreen) Move() {
	old := s.Position
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
	sendMail("Moved sunscreen "+new, fmt.Sprintf("Sunscreen moved from %s to %s. Light: %v", old, new, ls1.data))
	appendCSV(csvFile, [][]string{{time.Now().Format("02-01-2006 15:04:05"), mode, new, fmt.Sprint(ls1.data)}})
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
	switch s.Position {
	case up:
		//log.Printf("Sunscreen is %v. Check if weather is good to go down\n", s.Position)
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
		//log.Printf("Sunscreen is %v. Check if it should go up\n", s.Position)
		for _, v := range lightData[:(config.LightBadThreshold + config.AllowedOutliers)] {
			if v >= config.LightBadValue {
				counter++
			}
		}
		if counter >= config.LightBadThreshold {
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
			s.Move()
			return
		}
	}
}

// getCurrentLight collects the average input from the light sensor ls and returns the value as a slice of int
func (ls *LightSensor) getCurrentLight() (int, error) {
	freq := 10
	lightValues := make([]int, freq, freq)
	i := 0
	for i < len(lightValues) {
		lightValue, err := ls.getLightValue()
		if err != nil {
			log.Printf("Error retrieving light (%v/%v): %v", freq-len(lightValues)+i+1, freq, err)
			// Remove record from slice and continue loop
			lightValues = append(lightValues[:i], lightValues[i+1:]...)
			continue
		}
		lightValues[i] = lightValue
		i++
	}
	if len(lightValues) == 0 {
		return 0, fmt.Errorf("All of the %v attemps failed from pin %v", freq, ls.pinLight)
	}
	x := calcAverage(lightValues...) / config.LightFactor
	if x == 0 {
		return x, fmt.Errorf("Average light from pin %v is zero", ls.pinLight)
	}
	return x, nil
}

func (ls *LightSensor) getLightValue() (int, error) {
	count := 0
	// Output on the pin for 0.1 seconds
	ls.pinLight.Output()
	ls.pinLight.Low()
	time.Sleep(100 * time.Millisecond)

	// Change the pin back to input
	ls.pinLight.Input()

	// Count until the pin goes high
	mu.Lock()
	for ls.pinLight.Read() == rpio.Low {
		count++
		if count > maxCount {
			mu.Unlock()
			return count, fmt.Errorf("Count is getting too high (%v)", count)
		}
	}
	mu.Unlock()
	if count == 0 {
		return count, fmt.Errorf("Count is zero (%v)", count)
	}
	return count, nil
}

func calcAverage(xi ...int) int {
	total := 0
	if len(xi) == 0 {
		log.Panic("No values to calculate average from")
	}
	for _, v := range xi {
		total = total + v
	}
	return total / len(xi)
}

func (s *Sunscreen) autoSunscreen(ls *LightSensor) {
	for {
		mu.Lock()
		if s.Mode != auto {
			log.Println("Mode is no longer auto, closing auto func")
			mu.Unlock()
			return
		}
		switch {
		case config.Sunset.Sub(time.Now()).Minutes() <= float64(config.SunsetThreshold) && config.Sunset.Sub(time.Now()).Minutes() > 0 && s.Position == up:
			log.Printf("Sun will set in (less then) %v min and Sunscreen is %v. Snoozing until sunset for %v seconds...\n", config.SunsetThreshold, s.Position, int(config.Sunset.Sub(time.Now()).Seconds()))
			for config.Sunset.Sub(time.Now()).Minutes() <= float64(config.SunsetThreshold) && config.Sunset.Sub(time.Now()).Minutes() > 0 {
				if s.Mode != auto {
					log.Println("Mode is no longer auto, closing auto func")
					mu.Unlock()
					return
				}
				mu.Unlock()
				time.Sleep(time.Second)
				mu.Lock()
			}
			mu.Unlock()
			continue
		case time.Now().After(config.Sunset):
			// Sun is down, moving sunscreen up (if not already up)
			s.Up()
			mu.Unlock()
			continue
		case time.Now().Before(config.Sunrise):
			log.Printf("Sun is not yet up, snoozing until %v for %v seconds...\n", config.Sunrise.Format("2 Jan 15:04 MST"), int(config.Sunrise.Sub(time.Now()).Seconds()))
			s.Up()
			for i := 0; float64(i) <= config.Sunrise.Sub(time.Now()).Seconds(); i++ {
				if s.Mode != auto {
					log.Println("Mode is no longer auto, closing auto func")
					mu.Unlock()
					return
				}
				mu.Unlock()
				time.Sleep(time.Second)
				mu.Lock()
			}
		case time.Now().After(config.Sunrise) && time.Now().Before(config.Sunset):
			//if there is enough light gathered in ls.data, evaluate position
			if maxLen := MaxIntSlice(config.LightGoodThreshold, config.LightBadThreshold, config.LightNeutralThreshold) + config.AllowedOutliers; len(ls.data) >= maxLen {
				s.evalPosition(ls.data)
			} else if len(ls.data) <= 2 {
				log.Println("Not enough light gathered...", len(ls.data))
			}
		}
		//log.Printf("Completed cycle, sleeping for %v second(s)...\n", config.Interval)
		for i := 0; i < config.Interval; i++ {
			if s.Mode != auto {
				log.Println("Mode is no longer auto, closing auto func")
				mu.Unlock()
				return
			}
			mu.Unlock()
			time.Sleep(time.Second)
			mu.Lock()
		}
		mu.Unlock()
	}
}

func (ls *LightSensor) monitorLight() {
	for {
		mu.Lock()
		if time.Now().After(config.Sunrise) && time.Now().Before(config.Sunset) {
			// Sun is up, monitor light
			mu.Unlock()
			currentLight, err := ls.getCurrentLight()
			mu.Lock()
			if err != nil {
				log.Println("Error retrieving light:", err)
				goto End
			}
			ls.data = append([]int{currentLight}, ls.data...)
			appendCSV(lightFile, [][]string{{time.Now().Format("02-01-2006 15:04:05"), fmt.Sprint(ls.data[0])}})
			//ensure ls.data doesnt get too long
			if maxLen := MaxIntSlice(config.LightGoodThreshold, config.LightBadThreshold, config.LightNeutralThreshold) + config.AllowedOutliers; len(ls.data) > maxLen {
				ls.data = ls.data[:maxLen]
			}
		} else if time.Now().After(config.Sunset) {
			log.Printf("Sun is down (%v), adjusting Sunrise/set", config.Sunset.Format("2 Jan 15:04 MST"))
			config.Sunrise = config.Sunrise.AddDate(0, 0, 1)
			config.Sunset = config.Sunset.AddDate(0, 0, 1)
		} else {
			// Sun is not up yet, ensure data is empty
			ls.data = []int{}
		}
	End:
		start = time.Now()
		for time.Until(start.Add(ls.Interval)) > 0 {
			mu.Unlock()
			time.Sleep(time.Second)
			mu.Lock()
		}
		mu.Unlock()
	}
}

func loginHandler(w http.ResponseWriter, req *http.Request) {
	if alreadyLoggedIn(req) {
		http.Redirect(w, req, "/", http.StatusSeeOther)
		return
	}

	ip := GetIP(req)

	// Check if IP is on whitelist (true)
	knownIp := func(ip string) bool {
		for i, v := range ip {
			if v == 58 {
				ip = ip[:i]
				break
			}
		}
		for _, v := range config.IpWhitelist {
			if ip == v {
				return true
			}
		}
		return false
	}

	createSession := func() {
		// create session
		log.Printf("User (%v) logged in...", ip)
		sID := uuid.NewV4()
		c := &http.Cookie{
			Name:  "session",
			Value: sID.String(),
		}
		http.SetCookie(w, c)
		dbSessions[c.Value] = config.Username
		http.Redirect(w, req, "/", http.StatusSeeOther)
	}

	if knownIp(ip) {
		createSession()
		return
	}

	// process form submission
	if req.Method == http.MethodPost {
		u := req.FormValue("Username")
		p := req.FormValue("Password")

		if u != config.Username {
			log.Printf("%v entered incorrect username...", ip)
			http.Error(w, "Username and/or password do not match", http.StatusForbidden)
			return
		}
		// does the entered password match the stored password?
		err := bcrypt.CompareHashAndPassword(config.Password, []byte(p))
		if err != nil {
			log.Printf("%v entered incorrect password...", ip)
			http.Error(w, "Username and/or password do not match", http.StatusForbidden)
			return
		}
		createSession()
		return
	}

	err := tpl.ExecuteTemplate(w, "login.gohtml", nil)
	if err != nil {
		log.Fatalln(err)
	}
}

func logoutHandler(w http.ResponseWriter, req *http.Request) {
	if !alreadyLoggedIn(req) {
		http.Redirect(w, req, "/", http.StatusSeeOther)
		return
	}
	c, _ := req.Cookie("session")
	// delete the session
	delete(dbSessions, c.Value)
	// remove the cookie
	c = &http.Cookie{
		Name:   "session",
		Value:  "",
		MaxAge: -1,
	}
	http.SetCookie(w, c)

	http.Redirect(w, req, "/login", http.StatusSeeOther)
}

func mainHandler(w http.ResponseWriter, req *http.Request) {
	if !alreadyLoggedIn(req) {
		http.Redirect(w, req, "/login", http.StatusSeeOther)
		return
	}

	stats := readCSV(csvFile)
	mu.Lock()
	if len(stats) != 0 {
		stats = stats[MaxIntSlice(0, len(stats)-config.MoveHistory):]
	}
	data := struct {
		*Sunscreen
		Time         string
		RefreshRate  int
		Light        []int
		Stats        [][]string
		MoveHistory  int
		LightHistory int
	}{
		s1,
		time.Now().Format("_2 Jan 06 15:04:05"),
		config.RefreshRate,
		ls1.data,
		reverseXSS(stats),
		config.MoveHistory,
		len(ls1.data),
	}
	mu.Unlock()
	err := tpl.ExecuteTemplate(w, "index.gohtml", data)
	if err != nil {
		log.Fatalln(err)
	}
}

func lightHandler(w http.ResponseWriter, req *http.Request) {
	if !alreadyLoggedIn(req) {
		http.Redirect(w, req, "/login", http.StatusSeeOther)
		return
	}
	mu.Lock()
	stats := readCSV(lightFile)
	if len(stats) != 0 {
		stats = stats[MaxIntSlice(0, len(stats)-config.LogRecords):]
	}
	data := struct {
		Stats [][]string
	}{
		reverseXSS(stats),
	}
	mu.Unlock()
	err := tpl.ExecuteTemplate(w, "light.gohtml", data)
	if err != nil {
		log.Fatalln(err)
	}
}

func modeHandler(w http.ResponseWriter, req *http.Request) {
	if !alreadyLoggedIn(req) {
		http.Redirect(w, req, "/login", http.StatusSeeOther)
		return
	}

	mode := req.URL.Path[len("/mode/"):]
	mu.Lock()
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
	mu.Unlock()
	http.Redirect(w, req, "/", http.StatusFound)
}

func configHandler(w http.ResponseWriter, req *http.Request) {
	if !alreadyLoggedIn(req) {
		http.Redirect(w, req, "/login", http.StatusSeeOther)
		return
	}

	var err error
	mu.Lock()
	defer mu.Unlock()
	if req.Method == http.MethodPost {
		config.Sunrise, err = StoTime(req.PostFormValue("Sunrise"), 0)
		if err != nil {
			log.Fatalln(err)
		}
		config.Sunset, err = StoTime(req.PostFormValue("Sunset"), 0)
		if err != nil {
			log.Fatalln(err)
		}
		config.SunsetThreshold, err = strToInt(req.PostFormValue("SunsetThreshold"))
		if err != nil {
			log.Fatalln(err)
		}
		config.Interval, err = strToInt(req.PostFormValue("Interval"))
		if err != nil {
			log.Fatalln(err)
		}
		config.LightGoodValue, err = strToInt(req.PostFormValue("LightGoodValue"))
		if err != nil {
			log.Fatalln(err)
		}
		config.LightGoodThreshold, err = strToInt(req.PostFormValue("LightGoodThreshold"))
		if err != nil {
			log.Fatalln(err)
		}
		config.LightNeutralValue, err = strToInt(req.PostFormValue("LightNeutralValue"))
		if err != nil {
			log.Fatalln(err)
		}
		config.LightNeutralThreshold, err = strToInt(req.PostFormValue("LightNeutralThreshold"))
		if err != nil {
			log.Fatalln(err)
		}
		config.LightBadValue, err = strToInt(req.PostFormValue("LightBadValue"))
		if err != nil {
			log.Fatalln(err)
		}
		config.LightBadThreshold, err = strToInt(req.PostFormValue("LightBadThreshold"))
		if err != nil {
			log.Fatalln(err)
		}
		config.AllowedOutliers, err = strToInt(req.PostFormValue("AllowedOutliers"))
		if err != nil {
			log.Fatalln(err)
		}
		config.RefreshRate, err = strToInt(req.PostFormValue("RefreshRate"))
		if err != nil {
			log.Fatalln(err)
		}
		if req.PostFormValue("EnableMail") == "" {
			config.EnableMail = false
		} else {
			config.EnableMail, err = strconv.ParseBool(req.PostFormValue("EnableMail"))
			if err != nil {
				log.Fatalln(err)
			}
		}
		config.MoveHistory, err = strToInt(req.PostFormValue("MoveHistory"))
		if err != nil {
			log.Fatalln(err)
		}
		config.LogRecords, err = strToInt(req.PostFormValue("LogRecords"))
		if err != nil {
			log.Fatalln(err)
		}
		config.Notes = req.PostFormValue("Notes")
		config.Username = req.PostFormValue("Username")
		if req.PostFormValue("Password") != "" {
			err = bcrypt.CompareHashAndPassword(config.Password, []byte(req.PostFormValue("CurrentPassword")))
			if err != nil {
				http.Error(w, "Current password is incorrect, password has not been changed", http.StatusForbidden)
				SaveToJson(config, configFile)
				log.Println("Updated variables (except for password)")
				return
			} else {
				config.Password, _ = bcrypt.GenerateFromPassword([]byte(req.PostFormValue("Password")), bcrypt.MinCost)
			}
		}
		config.IpWhitelist = func(s string) []string {
			xs := strings.Split(s, ",")
			for i, v := range xs {
				xs[i] = strings.Trim(v, " ")
			}
			return xs
		}(req.PostFormValue("IpWhitelist"))

		config.LightFactor, err = strToInt(req.PostFormValue("LightFactor"))
		if err != nil {
			log.Fatalln(err)
		}
		SaveToJson(config, configFile)
		log.Println("Updated variables")
	}
	err = tpl.ExecuteTemplate(w, "config.gohtml", config)
	if err != nil {
		log.Panic(err)
	}
}

func logHandler(w http.ResponseWriter, req *http.Request) {
	if !alreadyLoggedIn(req) {
		http.Redirect(w, req, "/login", http.StatusSeeOther)
		return
	}

	f, err := ioutil.ReadFile("./logs/" + logFile)
	if err != nil {
		fmt.Println("File reading error", err)
		return
	}
	lines := strings.Split(string(f), "\n")
	var max = config.LogRecords
	if len(lines) < max {
		max = len(lines)
	}
	data := struct {
		FileName  string
		LogOutput []string
	}{
		logFile,
		reverseXS(lines)[:max],
	}
	err = tpl.ExecuteTemplate(w, "log.gohtml", data)
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

func SliceToString(xs []string) string {
	return strings.Join(xs, ",")
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

func reverseXSS(xxs [][]string) [][]string {
	r := [][]string{}
	for i, _ := range xxs {
		r = append(r, xxs[len(xxs)-1-i])
	}
	return r
}

func reverseXS(xs []string) []string {
	r := []string{}
	for i, _ := range xs {
		r = append(r, xs[len(xs)-1-i])
	}
	return r
}

func alreadyLoggedIn(req *http.Request) bool {
	c, err := req.Cookie("session")
	if err != nil {
		// Error retrieving cookie
		return false
	}
	un := dbSessions[c.Value]
	if un != config.Username {
		// Unknown cookie
		return false
	}
	return true
}

// GetIP gets a requests IP address by reading off the forwarded-for
// header (for proxies) and falls back to use the remote address.
func GetIP(req *http.Request) string {
	forwarded := req.Header.Get("X-FORWARDED-FOR")
	if forwarded != "" {
		return forwarded
	}
	return req.RemoteAddr
}
