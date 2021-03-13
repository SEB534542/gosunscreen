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
	"github.com/kelvins/sunrisesunset"
	"github.com/satori/go.uuid"
	"github.com/stianeikeland/go-rpio/v4"
	"golang.org/x/crypto/bcrypt"
)

// LightSensor represents a physical lightsensor for which data can be collected through the corresponding GPIO pin.
type LightSensor struct {
	PinLight              rpio.Pin      // pin for retrieving light value
	Interval              time.Duration // Interval for checking current light in seconds
	LightGoodValue        int           // Max measured light value that counts as "good weather"
	LightGoodThreshold    int           // Number of times light should be below lightGoodValue
	LightNeutralValue     int           // Max measured light value that counts as "neutral weather"
	LightNeutralThreshold int           // Number of times light should be above lightNeutralValue
	LightBadValue         int           // max measured light value that counts as "bad weather"
	LightBadThreshold     int           // number of times light should be above lightBadValue
	AllowedOutliers       int           // Number of outliers accepted in the measurement
	Data                  []int         // collected light values
	LightFactor           int           // Factor for correcting the measured analog light value
}

// Sunscreen represents a physical Sunscreen that can be controlled through 2 GPIO pins: one for moving it up, and one for moving it down.
type Sunscreen struct {
	Id            int           // Autogenerated ID for sunscreen
	Name          string        // Name of sunscreen
	Mode          string        // Mode of Sunscreen auto or manual
	Position      string        // Current position of Sunscreen
	DurDown       time.Duration // Duration to move Sunscreen down
	DurUp         time.Duration // Duration to move Sunscreen up
	PinDown       rpio.Pin      // GPIO pin for moving sunscreen down
	PinUp         rpio.Pin      // GPIO pin for moving sunscreen up
	AutoStart     bool          // If true, Start is calculated based on config.Location.GetSunriseSunset() and SunStart
	AutoStop      bool          // If true, Stop is calculated based on config.Location.GetSunriseSunset() and SunStop
	SunStart      time.Duration // Duration after Sunrise to determine Start
	SunStop       time.Duration // Duration after before Sunset to determine Stop
	Start         time.Time     // Time after which Sunscreen can shine on the Sunscreen area
	Stop          time.Time     // Time after which Sunscreen no can shine on the Sunscreen area
	StopThreshold time.Duration // Duration before Stop that Sunscreen no longer should go down
}

type Site struct {
	Sunscreens  []*Sunscreen
	LightSensor *LightSensor
}

type Config struct {
	RefreshRate time.Duration            // Number of seconds the main page should refresh
	MoveHistory int                      // Number of sunscreen movements to be shown
	LogRecords  int                      // Number of log records that are shown
	Username    string                   // Username for logging in
	Password    []byte                   // Password for logging in
	IpWhitelist []string                 // Whitelisted IPs
	Port        int                      // Port of the localhost
	EnableMail  bool                     // Enable mail functionality
	MailFrom    string                   // E-mail address from, often same as username
	MailUser    string                   // E-mail Username
	MailPass    string                   // E-mail Password
	MailTo      []string                 // E-mail to
	MailHost    string                   // E-mail host
	MailPort    int                      // E-mail host port
	Location    sunrisesunset.Parameters // Contains Latiude, longitude, UtcOffset and Date
	Sunrise     time.Time                // Date and time of sunrise for Location
	Sunset      time.Time                // Date and time of sunset for Location
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
	logFile   = "./logs/logfile.log"
	csvFile   = "./logs/sunscreen_stats.csv"
	lightFile = "./logs/light_stats.csv"
)

// Constants for config folder and files
const (
	configFolder = "config"
	configFile   = "./config/config.json"
	siteFile     = "./config/site.json"
)

var (
	tpl        *template.Template
	mu         sync.Mutex
	fm         = template.FuncMap{"fdateHM": hourMinute, "fsliceString": sliceToString, "fminutes": minutes, "fseconds": seconds}
	dbSessions = map[string]string{}
	site       = &Site{}
	config     Config
)

func init() {
	//Loading gohtml templates
	tpl = template.Must(template.New("").Funcs(fm).ParseGlob("./templates/*"))

	// Check if log folder exists, else create
	if _, err := os.Stat(logFolder); os.IsNotExist(err) {
		os.Mkdir(logFolder, 4096)
	}

	// Check if log folder exists, else create
	if _, err := os.Stat(configFolder); os.IsNotExist(err) {
		os.Mkdir(configFolder, 4096)
	}
}

func main() {
	// Open logfile or create if not exists
	f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Panic("Error opening log file:", err)
	}
	defer f.Close()
	log.SetOutput(f)

	log.Println("--------Start of program--------")

	// Load general config
	err = seb.LoadConfig(configFile, &config)
	checkErr(err)
	if config.Port == 0 {
		config.Port = 8081
		log.Printf("Unable to load port, set port to default (%v)", config.Port)
	}
	if config.Username == "" {
		config.Username = "admin"
		pw, err := bcrypt.GenerateFromPassword([]byte("today"), bcrypt.MinCost)
		if err != nil {
			log.Fatal("Error setting default password:", err)
		}
		config.Password = pw
	}
	if config.RefreshRate == time.Duration(0) {
		config.RefreshRate, err = time.ParseDuration("1h")
		checkErr(err)
	}

	// Load site config (sunscreens and lightsensor)
	err = seb.LoadConfig(siteFile, &site)

	//Resetting Start and Stop to today
	site.resetStartStop(0)

	// Connecting to rpio Pins
	rpio.Open()
	defer rpio.Close()

	// Set Sunscreen pins
	for _, s := range site.Sunscreens {
		s.PinDown.Output()
		s.PinDown.High()
		s.PinUp.Output()
		s.PinUp.High()

		defer func() {
			log.Println("Closing down...")
			mu.Lock()
			s.Up()
			mu.Unlock()
		}()
	}

	// Monitor light
	if site.LightSensor != nil {
		go site.monitorLight()
	}
	log.Printf("Launching website at localhost:%v...", config.Port)
	http.HandleFunc("/", handlerMain)
	http.Handle("/favicon.ico", http.NotFoundHandler())
	http.HandleFunc("/mode/", handlerMode)
	http.HandleFunc("/config/add/", handlerAddSunscreen)
	http.HandleFunc("/config/edit/", handlerEditSunscreen)
	http.HandleFunc("/config/delete/", handlerDeleteSunscreen)
	http.HandleFunc("/config/", handlerConfig)
	http.HandleFunc("/log/", handlerLog)
	http.HandleFunc("/login", handlerLogin)
	http.HandleFunc("/logout", handlerLogout)
	http.HandleFunc("/light", handlerLight)
	err = http.ListenAndServeTLS(":"+fmt.Sprint(config.Port), "cert.pem", "key.pem", nil)
	if err != nil {
		log.Println("ERROR: Unable to launch TLS, launching without TLS...")
		log.Fatal(http.ListenAndServe(":"+fmt.Sprint(config.Port), nil))
	}
}

// Move moves the suncreen up or down based on the Sunscreen.Position. It updates the position accordingly.
func (s *Sunscreen) Move() {
	old := s.Position
	if s.Position != up {
		log.Printf("Sunscreen position is %v, moving sunscreen up...\n", s.Position)
		s.PinUp.Low()
		n := time.Now()
		for time.Now().Before(n.Add(s.DurUp)) {
			time.Sleep(time.Second)
		}
		s.PinUp.High()
		s.Position = up
	} else {
		log.Printf("Sunscreen position is %v, moving sunscreen down...\n", s.Position)
		s.PinDown.Low()
		n := time.Now()
		for time.Now().Before(n.Add(s.DurDown)) {
			time.Sleep(time.Second)
		}
		s.PinDown.High()
		s.Position = down
	}
	new := s.Position
	mode := s.Mode
	sendMail("Moved sunscreen "+new, fmt.Sprintf("Sunscreen moved from %s to %s. Light: %v", old, new, site.LightSensor.Data))
	appendCSV(csvFile, [][]string{{time.Now().Format("02-01-2006 15:04:05"), mode, new, fmt.Sprint(site.LightSensor.Data)}})
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

func (site *Site) resetStartStop(d int) {
	for _, s := range site.Sunscreens {
		if s.AutoStart || s.AutoStop {
			s.resetAutoTime(d)
		}
		if !s.AutoStart {
			s.Start = time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), s.Start.Hour(), s.Start.Minute(), 0, 0, time.Now().Location()).AddDate(0, 0, d)
		}
		if !s.AutoStop {
			s.Stop = time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), s.Stop.Hour(), s.Stop.Minute(), 0, 0, time.Now().Location()).AddDate(0, 0, d)
		}
	}
}

func (s *Sunscreen) resetAutoTime(d int) {
	var start, stop time.Time
	var err error
	if s.AutoStart || s.AutoStop {
		config.Location.Date = time.Now().AddDate(0, 0, d)
		start, stop, err = config.Location.GetSunriseSunset()
		if err != nil {
			log.Fatal("Error during determining sunrise and sunset:", err)
		}
	}
	if s.AutoStart {
		s.Start = start.Add(s.SunStart)
	}
	if s.AutoStop {
		s.Stop = stop.Add(-s.SunStop)
	}
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
		return 0, fmt.Errorf("All of the %v attemps failed from pin %v", freq, ls.PinLight)
	}
	x := calcAverage(lightValues...) / ls.LightFactor
	if x == 0 {
		return x, fmt.Errorf("Average light from pin %v is zero", ls.PinLight)
	}
	return x, nil
}

func (ls *LightSensor) getLightValue() (int, error) {
	count := 0
	// Output on the pin for 0.1 seconds
	ls.PinLight.Output()
	ls.PinLight.Low()
	time.Sleep(100 * time.Millisecond)

	// Change the pin back to input
	ls.PinLight.Input()

	// Count until the pin goes high
	mu.Lock()
	for ls.PinLight.Read() == rpio.Low {
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
		case time.Now().After(s.Stop.Add(-s.StopThreshold)) && time.Now().Before(s.Stop) && s.Position == up:
			log.Printf("Sun will set in (less then) %v min and Sunscreen is %v. Snoozing until sunset for %v...\n", s.StopThreshold, s.Position, time.Now().Sub(s.Stop))
			for time.Now().After(s.Stop.Add(-s.StopThreshold)) && time.Now().Before(s.Stop) {
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
		case time.Now().After(s.Stop):
			// Sun is down, moving sunscreen up (if not already up)
			s.Up()
			mu.Unlock()
			continue
		case time.Now().Before(s.Start):
			log.Printf("Sun is not yet up, snoozing until %v for %v...\n", s.Start.Format("2 Jan 15:04 MST"), time.Now().Sub(s.Start))
			s.Up()
			for time.Now().Before(s.Start) {
				if s.Mode != auto {
					log.Println("Mode is no longer auto, closing auto func")
					mu.Unlock()
					return
				}
				mu.Unlock()
				time.Sleep(time.Second)
				mu.Lock()
			}
		case time.Now().After(s.Start) && time.Now().Before(s.Stop):
			//if there is enough light gathered in ls.Data, evaluate position
			if maxLen := MaxIntSlice(ls.LightGoodThreshold, ls.LightBadThreshold, ls.LightNeutralThreshold) + ls.AllowedOutliers; len(ls.Data) >= maxLen {
				s.evalPosition(ls)
			} else if len(ls.Data) <= 2 {
				log.Println("Not enough light gathered...", len(ls.Data))
			}
		}
		//log.Printf("Completed cycle, sleeping for %v second(s)...\n", config.Interval)
		n := time.Now()
		for time.Now().Before(n.Add(ls.Interval)) {
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

func (site *Site) monitorLight() {
	for {
		mu.Lock()
		var minStart, maxStop time.Time
		for _, s := range site.Sunscreens {
			if s.Start.Before(minStart) || minStart.IsZero() {
				minStart = s.Start
			}
			if s.Stop.After(maxStop) || maxStop.IsZero() {
				maxStop = s.Stop
			}
		}
		if time.Now().After(minStart) && time.Now().Before(maxStop) {
			// "Sun" is up, monitor light
			mu.Unlock()
			currentLight, err := site.LightSensor.getCurrentLight()
			mu.Lock()
			if err != nil {
				log.Println("Error retrieving light:", err)
				goto End
			}
			site.LightSensor.Data = append([]int{currentLight}, site.LightSensor.Data...)
			appendCSV(lightFile, [][]string{{time.Now().Format("02-01-2006 15:04:05"), fmt.Sprint(site.LightSensor.Data[0])}})
			//ensure ls.Data doesnt get too long
			if maxLen := MaxIntSlice(site.LightSensor.LightGoodThreshold, site.LightSensor.LightBadThreshold, site.LightSensor.LightNeutralThreshold) + site.LightSensor.AllowedOutliers; len(site.LightSensor.Data) > maxLen {
				site.LightSensor.Data = site.LightSensor.Data[:maxLen]
			}
		} else if time.Now().After(maxStop) {
			log.Printf("Sun is down (%v), adjusting Sunrise/set", maxStop.Format("2 Jan 15:04 MST"))
			site.resetStartStop(1)
		} else {
			// Sun is not up yet, ensure Data is empty
			site.LightSensor.Data = []int{}
		}
	End:
		n := time.Now()
		for time.Until(n.Add(site.LightSensor.Interval)) > 0 {
			mu.Unlock()
			time.Sleep(time.Second)
			mu.Lock()
		}
		mu.Unlock()
	}
}

func handlerLogin(w http.ResponseWriter, req *http.Request) {
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

func handlerLogout(w http.ResponseWriter, req *http.Request) {
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

func handlerMain(w http.ResponseWriter, req *http.Request) {
	if !alreadyLoggedIn(req) {
		http.Redirect(w, req, "/login", http.StatusSeeOther)
		return
	}

	stats := readCSV(csvFile)
	mu.Lock()
	if len(stats) != 0 {
		stats = stats[MaxIntSlice(0, len(stats)-config.MoveHistory):]
	}
	var l int
	if site.LightSensor != nil {
		l = len(site.LightSensor.Data)
	}
	data := struct {
		*Site
		Time         string
		RefreshRate  time.Duration
		Stats        [][]string
		MoveHistory  int
		LightHistory int
	}{
		site,
		time.Now().Format("_2 Jan 06 15:04:05"),
		config.RefreshRate, //int(config.RefreshRate.Seconds()),
		reverseXSS(stats),
		config.MoveHistory,
		l,
	}
	mu.Unlock()
	err := tpl.ExecuteTemplate(w, "index.gohtml", data)
	if err != nil {
		log.Fatalln(err)
	}
}

func handlerLight(w http.ResponseWriter, req *http.Request) {
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

func handlerMode(w http.ResponseWriter, req *http.Request) {
	if !alreadyLoggedIn(req) {
		http.Redirect(w, req, "/login", http.StatusSeeOther)
		return
	}

	// Url logic is: '/mode/xxxx/auto" where xxxx = Sunscreen.Id followed by the mode (e.g. auto, up or down)
	idMode := req.URL.Path[len("/mode/"):]
	id, err := strconv.Atoi(idMode[:strings.Index(idMode, "/")])
	if err != nil {
		http.Error(w, "Unknown Sunscreen Id", http.StatusForbidden)
		return
	}
	mode := idMode[strings.Index(idMode, "/")+1:]
	i, err := site.sIndex(id)
	if err != nil {
		http.Error(w, "Unknown Sunscreen Id", http.StatusForbidden)
		return
	}
	mu.Lock()
	switch mode {
	case auto:
		if site.Sunscreens[i].Mode == manual {
			go site.Sunscreens[i].autoSunscreen(site.LightSensor)
			site.Sunscreens[i].Mode = auto
			log.Printf("Set mode to auto (%v)\n", site.Sunscreens[i].Mode)
		} else {
			log.Printf("Mode is already auto (%v)\n", site.Sunscreens[i].Mode)
		}
	case manual + "/" + up:
		site.Sunscreens[i].Mode = manual
		site.Sunscreens[i].Up()
	case manual + "/" + down:
		site.Sunscreens[i].Mode = manual
		site.Sunscreens[i].Down()
	default:
		log.Println("Unknown mode:", req.URL.Path)
	}
	log.Println("Mode=", site.Sunscreens[i].Mode, "and Position=", site.Sunscreens[i].Position)
	mu.Unlock()
	http.Redirect(w, req, "/", http.StatusFound)
}

func handlerEditSunscreen(w http.ResponseWriter, req *http.Request) {
	if !alreadyLoggedIn(req) {
		http.Redirect(w, req, "/login", http.StatusSeeOther)
		return
	}

	// Check if there is anything after "/config/edit/", i.e. the sunscreen ID
	id, err := strconv.Atoi(req.URL.Path[len("/config/edit/"):])
	if err != nil {
		http.Error(w, fmt.Sprintf("Unknown sunscreen (%v)", id), http.StatusForbidden)
		return
	}
	mu.Lock()
	i, err := site.sIndex(id)
	if err != nil {
		mu.Unlock()
		http.Error(w, fmt.Sprintf("Unknown sunscreen (%v)", id), http.StatusForbidden)
		return
	}
	msgs := site.Sunscreens[i].processReq(req)
	if len(msgs) == 0 {
		SaveToJson(site, siteFile)
		log.Println("Saved sunscreen")
		mu.Unlock()
		http.Redirect(w, req, "/config", http.StatusSeeOther)
		return
	} else {
		msg := "Unable to save Sunscreen, please correct errors"
		msgs = append(msgs, msg)
		log.Println(msg)
		http.Error(w, strings.Join(msgs, "\n"), http.StatusForbidden)
		return
	}
	mu.Unlock()
}

func handlerDeleteSunscreen(w http.ResponseWriter, req *http.Request) {
	if !alreadyLoggedIn(req) {
		http.Redirect(w, req, "/login", http.StatusSeeOther)
		return
	}
	id, err := strconv.Atoi(req.URL.Path[len("/config/delete/"):])
	if err != nil {
		http.Error(w, fmt.Sprintf("Unknown sunscreen (%v)", id), http.StatusForbidden)
		return
	}
	mu.Lock()
	i, err := site.sIndex(id)
	if err != nil {
		mu.Unlock()
		http.Error(w, fmt.Sprintf("Unknown sunscreen (%v)", id), http.StatusForbidden)
		return
	}
	site.Sunscreens = append(site.Sunscreens[:i], site.Sunscreens[i+1:]...)
	mu.Unlock()
	http.Redirect(w, req, "/config", http.StatusSeeOther)
	return
}

func handlerConfig(w http.ResponseWriter, req *http.Request) {
	if !alreadyLoggedIn(req) {
		http.Redirect(w, req, "/login", http.StatusSeeOther)
		return
	}

	var err error
	var msgs []string
	appendMsgs := func(msg string) {
		msgs = append(msgs, msg)
		log.Println(msg)
	}
	stringToSlice := func(s string) []string {
		xs := strings.Split(s, ",")
		for i, v := range xs {
			xs[i] = strings.Trim(v, " ")
		}
		return xs
	}

	mu.Lock()
	defer mu.Unlock()
	if req.Method == http.MethodPost {
		if site.LightSensor == nil {
			site.LightSensor = &LightSensor{}
		}
		// Read, validate and store light sensor
		lightGoodValue, err := strToInt(req.PostFormValue("LightGoodValue"))
		if err != nil {
			appendMsgs(fmt.Sprintf("Unable to save LightGoodValue: %v", err))
		}
		lightNeutralValue, err := strToInt(req.PostFormValue("LightNeutralValue"))
		if err != nil {
			appendMsgs(fmt.Sprintf("Unable to save LightNeutralValue: %v", err))
		}
		lightBadValue, err := strToInt(req.PostFormValue("LightBadValue"))
		if err != nil {
			appendMsgs(fmt.Sprintf("Unable to save LightBadValue: %v", err))
		}
		if (lightGoodValue < lightNeutralValue && lightNeutralValue < lightBadValue) && err == nil {
			site.LightSensor.LightGoodValue = lightGoodValue
			site.LightSensor.LightNeutralValue = lightNeutralValue
			site.LightSensor.LightBadValue = lightBadValue
		} else {
			if err != nil {
				appendMsgs(fmt.Sprintf("Error while reading light values: %v", err))
			} else {
				appendMsgs(fmt.Sprintf("Light values incorrect, (good<neutral<bad): %v<%v<%v", lightGoodValue, lightNeutralValue, lightBadValue))
			}
		}
		// Light Threshold
		{
			min := 5
			lightGoodThreshold, err := strToInt(req.PostFormValue("LightGoodThreshold"))
			if err != nil {
				appendMsgs(fmt.Sprintf("Error reading LightGoodThreshold: %v", err))
			} else {
				if lightGoodThreshold < min {
					appendMsgs(fmt.Sprintf("lightGoodThreshold should be minimum %v (was %v)", min, lightGoodThreshold))
					lightGoodThreshold = min
				}
				site.LightSensor.LightGoodThreshold = lightGoodThreshold
			}
			lightNeutralThreshold, err := strToInt(req.PostFormValue("LightNeutralThreshold"))
			if err != nil {
				appendMsgs(fmt.Sprintf("Error reading LightNeutralThreshold: %v", err))
			} else {
				if lightNeutralThreshold < min {
					appendMsgs(fmt.Sprint("lightNeutralThreshold should be minimum %v (was %v)", min, lightNeutralThreshold))
					lightNeutralThreshold = min
				}
				site.LightSensor.LightNeutralThreshold = lightNeutralThreshold
			}
			lightBadThreshold, err := strToInt(req.PostFormValue("LightBadThreshold"))
			if err != nil {
				appendMsgs(fmt.Sprintf("Error reading LightBadThreshold: %v", err))
			} else {
				if lightBadThreshold < min {
					appendMsgs(fmt.Sprint("lightBadThreshold should be minimum %v (was %v)", min, lightBadThreshold))
					lightBadThreshold = min
				}
				site.LightSensor.LightBadThreshold = lightBadThreshold
			}
		}
		site.LightSensor.AllowedOutliers, err = strToInt(req.PostFormValue("AllowedOutliers"))
		if err != nil {
			appendMsgs(fmt.Sprintf("Error reading AllowedOutliers: %v", err))
		}
		lightFactor, err := strToInt(req.PostFormValue("LightFactor"))
		if err != nil || lightFactor == 0 {
			appendMsgs(fmt.Sprintf("LightFactor (%v) should a number greater than zero: %v", lightFactor, err))
		} else {
			site.LightSensor.LightFactor = lightFactor
		}
		pin, err := strToInt(req.PostFormValue("PinLight"))
		if !(pin > 0 && pin < 28) || err != nil {
			appendMsgs(fmt.Sprintf("Unable to save Led Pin '%v' (%v)", pin, err))
		} else {
			site.LightSensor.PinLight = rpio.Pin(pin)
		}
		interval, err := time.ParseDuration(req.PostFormValue("Interval") + "s")
		var min float64 = 10.0
		if err != nil || interval.Seconds() < min {
			appendMsgs(fmt.Sprintf("Unable to save Interval '%v', should be minimal %v seconds (%v)", interval, min, err))
		} else {
			site.LightSensor.Interval = interval
		}
		// Read, validate and store config
		refreshRate, err := time.ParseDuration(req.PostFormValue("RefreshRate") + "m")
		if err != nil {
			appendMsgs(fmt.Sprintf("Unable to save RefreshRate '%v' (%v)", refreshRate, err))
		} else {
			config.RefreshRate = refreshRate
		}
		moveHistory, err := strToInt(req.PostFormValue("MoveHistory"))
		if err != nil {
			appendMsgs(fmt.Sprintf("Unable to save MoveHistory (%v)", err))
		} else {
			config.MoveHistory = moveHistory
		}
		logRecords, err := strToInt(req.PostFormValue("LogRecords"))
		if err != nil {
			appendMsgs(fmt.Sprintf("Unable to save LogRecords (%v)", err))
		} else {
			config.LogRecords = logRecords
		}
		config.IpWhitelist = stringToSlice(req.PostFormValue("IpWhitelist"))
		port, err := strToInt(req.PostFormValue("Port"))
		if err != nil || !(port >= 1000 && port <= 9999) {
			appendMsgs(fmt.Sprintf("Unable to save port '%v', should be within range 1000-9999 (%v)", port, err))
		} else {
			config.Port = port
		}
		if req.PostFormValue("Username") != "" && req.PostFormValue("Username") != config.Username {
			err = bcrypt.CompareHashAndPassword(config.Password, []byte(req.PostFormValue("CurrentPassword")))
			if err != nil {
				appendMsgs(fmt.Sprintf("Current password is incorrect, username has not been updated"))
			} else {
				config.Username = req.PostFormValue("Username")
				appendMsgs(fmt.Sprintf("New username saved"))
			}
		}
		if req.PostFormValue("Password") != "" {
			err = bcrypt.CompareHashAndPassword(config.Password, []byte(req.PostFormValue("CurrentPassword")))
			if err != nil {
				appendMsgs(fmt.Sprintf("Current password is incorrect, password has not been updated"))
			} else {
				config.Password, _ = bcrypt.GenerateFromPassword([]byte(req.PostFormValue("Password")), bcrypt.MinCost)
				appendMsgs(fmt.Sprintf("New password saved"))
			}
		}
		// Mail config
		if req.PostFormValue("EnableMail") == "" {
			config.EnableMail = false
		} else {
			//enableMail, err = strconv.ParseBool(req.PostFormValue("EnableMail"))
			config.EnableMail = true
		}
		config.MailFrom = req.PostFormValue("MailFrom")
		config.MailUser = req.PostFormValue("MailUser")
		if req.PostFormValue("MailPass") != "" {
			config.MailPass = req.PostFormValue("MailPass")
		}
		config.MailTo = stringToSlice(req.PostFormValue("MailTo"))
		config.MailHost = req.PostFormValue("MailHost")
		mailPort, err := strToInt(req.PostFormValue("MailPort"))
		if err != nil {
			appendMsgs(fmt.Sprintf("Unable to save mail port: %v", err))
		} else {
			config.MailPort = mailPort
		}
		lat, err := strconv.ParseFloat(req.PostFormValue("Latitude"), 64)
		if err != nil {
			appendMsgs(fmt.Sprintf("Unable to save location latitude ('%v'): %v", lat, err))
		} else {
			config.Location.Latitude = lat
		}
		long, err := strconv.ParseFloat(req.PostFormValue("Longitude"), 64)
		if err != nil {
			appendMsgs(fmt.Sprintf("Unable to save location longitude ('%v'): %v", long, err))
		} else {
			config.Location.Longitude = long
		}
		utcOffset, err := strconv.ParseFloat(req.PostFormValue("UtcOffset"), 64)
		if err != nil {
			appendMsgs(fmt.Sprintf("Unable to save location UtcOffset ('%v'): %v", utcOffset, err))
		} else {
			config.Location.UtcOffset = utcOffset
		}

		var msg string
		if len(msgs) == 0 {
			msg = "Saved configuration"
		} else {
			msg = "Saved the rest"
		}
		appendMsgs(msg)

		SaveToJson(config, configFile)
		SaveToJson(site, siteFile)
		log.Println("Updated configuration")
	}

	s := *site
	if site.LightSensor == nil {
		s.LightSensor = &LightSensor{}
	}

	data := struct {
		Site
		Config
		Msgs []string
	}{
		s,
		config,
		msgs,
	}

	err = tpl.ExecuteTemplate(w, "config.gohtml", data)
	if err != nil {
		log.Panic(err)
	}
}

func handlerAddSunscreen(w http.ResponseWriter, req *http.Request) {
	if !alreadyLoggedIn(req) {
		http.Redirect(w, req, "/login", http.StatusSeeOther)
		return
	}

	var s = &Sunscreen{}
	var msgs []string
	appendMsgs := func(msg string) {
		msgs = append(msgs, msg)
		log.Println(msg)
	}
	if req.Method == http.MethodPost {
		// TODO: include validations
		// Check for duplicates in Name and pins
		// Durations > 0
		// Start > Stop
		id := 1000
		mu.Lock()
		for _, v := range site.Sunscreens {
			if v.Id >= id {
				id = v.Id + 1
			}
		}
		s.Id = id
		s.processReq(req)
		if len(msgs) == 0 {
			s.Mode = manual
			s.Position = up
			site.Sunscreens = append(site.Sunscreens, s)
			SaveToJson(site, siteFile)
			log.Println("Added new sunscreen")
			mu.Unlock()
			http.Redirect(w, req, "/config", http.StatusSeeOther)
			return
		} else {
			appendMsgs("Unable to save Sunscreen, please correct errors")
		}
		mu.Unlock()
	}
	data := struct {
		*Sunscreen
		Msgs []string
	}{
		s,
		msgs,
	}
	err := tpl.ExecuteTemplate(w, "add.gohtml", data)
	if err != nil {
		log.Panic(err)
	}
}

func handlerLog(w http.ResponseWriter, req *http.Request) {
	if !alreadyLoggedIn(req) {
		http.Redirect(w, req, "/login", http.StatusSeeOther)
		return
	}

	f, err := ioutil.ReadFile(logFile)
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

func sliceToString(xs []string) string {
	return strings.Join(xs, ",")
}

func hourMinute(t time.Time) string {
	return t.Format("15:04")
}

func minutes(d time.Duration) string {
	return fmt.Sprint(d.Minutes())
}

func seconds(d time.Duration) string {
	return fmt.Sprint(d.Seconds())
}

// SendMail sends mail to
func sendMail(subj, body string) {
	if config.EnableMail {
		//Format message
		var msgTo string
		for i, s := range config.MailTo {
			if i != 0 {
				msgTo = msgTo + ","
			}
			msgTo = msgTo + s
		}

		msg := []byte("To:" + msgTo + "\r\n" +
			"Subject:" + subj + "\r\n" +
			"\r\n" + body + "\r\n")

		// Set up authentication information
		auth := smtp.PlainAuth("", config.MailUser, config.MailPass, config.MailHost)

		// Connect to the server, authenticate, set the sender and recipient,
		// and send the email all in one step.
		err := smtp.SendMail(fmt.Sprintf("%v:%v", config.MailHost, config.MailPort), auth, config.MailFrom, config.MailTo, msg)
		if err != nil {
			log.Println("Unable to send mail:", err)
			return
		}
		log.Println("Send mail to", config.MailTo)
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

// CheckErr evaluates err for errors (not nil)
// and triggers a log.Panic containing the error.
func checkErr(err error) {
	if err != nil {
		log.Panic("Error:", err)
	}
	return
}

// sIndex retrieves the index of the corresponding Sunscreen Id within the site
func (site *Site) sIndex(id int) (int, error) {
	for i, s := range site.Sunscreens {
		if s.Id == id {
			return i, nil
		}
	}
	return 0, fmt.Errorf("Id %v not found", id)
}

func (s *Sunscreen) processReq(req *http.Request) []string {
	var msgs []string
	appendMsgs := func(msg string) {
		msgs = append(msgs, msg)
		log.Println(msg)
	}
	s.Name = req.PostFormValue("Name")

	if req.PostFormValue("AutoStart") == "" {
		s.AutoStart = false
		start, err := seb.StoTime(req.PostFormValue("Start"), 0)
		if err != nil {
			appendMsgs(fmt.Sprintf("Unable to save Start time '%v' (%v)", start, err))
		} else {
			s.Start = start
		}
	} else {
		s.AutoStart = true
		sunStart, err := time.ParseDuration(req.PostFormValue("SunStart") + "m")
		if err != nil {
			appendMsgs(fmt.Sprintf("Unable to save SunStart '%v' (%v)", sunStart, err))
		} else {
			s.SunStart = sunStart
		}
	}
	if req.PostFormValue("AutoStop") == "" {
		s.AutoStop = false
		stop, err := seb.StoTime(req.PostFormValue("Stop"), 0)
		if err != nil {
			appendMsgs(fmt.Sprintf("Unable to save Stop time '%v' (%v)", stop, err))
		} else {
			s.Stop = stop
		}
	} else {
		s.AutoStop = true
		sunStop, err := time.ParseDuration(req.PostFormValue("SunStop") + "m")
		if err != nil {
			appendMsgs(fmt.Sprintf("Unable to save SunStart '%v' (%v)", sunStop, err))
		} else {
			s.SunStop = sunStop
		}
	}
	s.resetAutoTime(0)
	stopThreshold, err := time.ParseDuration(req.PostFormValue("StopThreshold") + "m")
	if err != nil {
		appendMsgs(fmt.Sprintf("Unable to save StopThreshold '%v' (%v)", stopThreshold, err))
	} else {
		s.StopThreshold = stopThreshold
	}
	durDown, err := time.ParseDuration(req.PostFormValue("DurDown") + "s")
	if err != nil {
		appendMsgs(fmt.Sprintf("Unable to save DurDown '%v' (%v)", durDown, err))
	} else {
		s.DurDown = durDown
	}
	durUp, err := time.ParseDuration(req.PostFormValue("DurUp") + "s")
	if err != nil {
		appendMsgs(fmt.Sprintf("Unable to save DurUp '%v' (%v)", durUp, err))
	} else {
		s.DurUp = durUp
	}
	pinDown, err := strToInt(req.PostFormValue("PinDown"))
	if !(pinDown > 0 && pinDown < 28) || err != nil {
		appendMsgs(fmt.Sprintf("Unable to save Led Pin '%v' (%v)", pinDown, err))
	} else {
		s.PinDown = rpio.Pin(pinDown)
	}
	pinUp, err := strToInt(req.PostFormValue("PinUp"))
	if !(pinUp > 0 && pinUp < 28) || err != nil {
		appendMsgs(fmt.Sprintf("Unable to save Led Pin '%v' (%v)", pinUp, err))
	} else {
		s.PinUp = rpio.Pin(pinUp)
	}
	return msgs
}
