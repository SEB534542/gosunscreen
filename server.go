package main

import (
	"encoding/csv"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kelvins/sunrisesunset"
	"github.com/satori/go.uuid"
	"github.com/stianeikeland/go-rpio/v4"
	"golang.org/x/crypto/bcrypt"
)

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
	Cert        string                   // location and name of cert.pem for HTTPS connection
	Key         string                   // location and name of cert.pem for HTTPS connection
	Location    sunrisesunset.Parameters // Contains Latiude, longitude, UtcOffset and Date for calculation when sun rises and sets
}

var (
	tpl        *template.Template
	fm         = template.FuncMap{"fdateHM": hourMinute, "fsliceString": sliceToString, "fminutes": minutes, "fseconds": seconds}
	dbSessions = map[string]string{}
)

func init() {
	//Loading gohtml templates
	tpl = template.Must(template.New("").Funcs(fm).ParseGlob("./templates/*"))
}

func startServer() {
	muConf.Lock()
	if config.Port == 0 {
		config.Port = 8081
		log.Printf("No port configured, using port %v", config.Port)
	}
	port := config.Port
	cert := config.Cert
	key := config.Key
	muConf.Unlock()
	log.Printf("Launching website at localhost:%v...", port)
	http.HandleFunc("/", handlerMain)
	http.Handle("/favicon.ico", http.NotFoundHandler())
	http.HandleFunc("/mode/", handlerMode)
	http.HandleFunc("/config/", handlerConfig)
	http.HandleFunc("/log/", handlerLog)
	http.HandleFunc("/login", handlerLogin)
	http.HandleFunc("/logout", handlerLogout)
	http.HandleFunc("/light", handlerLight)
	http.HandleFunc("/stop", handlerStop)
	err := http.ListenAndServeTLS(":"+fmt.Sprint(port), cert, key, nil)
	if err != nil {
		log.Println("ERROR: Unable to launch TLS, launching without TLS...", err)
		log.Fatal(http.ListenAndServe(":"+fmt.Sprint(port), nil))
	}
}

func handlerLog(w http.ResponseWriter, req *http.Request) {
	if !alreadyLoggedIn(req) {
		http.Redirect(w, req, "/login", http.StatusSeeOther)
		return
	}

	f, err := ioutil.ReadFile(fileLog)
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
		fileLog,
		reverseXS(lines)[:max],
	}
	err = tpl.ExecuteTemplate(w, "log.gohtml", data)
	if err != nil {
		log.Fatalln(err)
	}
}

func handlerConfig(w http.ResponseWriter, req *http.Request) {
	if !alreadyLoggedIn(req) {
		http.Redirect(w, req, "/login", http.StatusSeeOther)
		return
	}
	var err error
	var msgs []string
	//mu.Lock()
	// defer mu.Unlock()
	if req.Method == http.MethodPost {
		// Store lightsensor config
		msgsNew := updateLightsensor(req)
		if len(msgsNew) == 0 {
			SaveToJSON(s, fileLightsensor)
			log.Println("Saved lightsensor")
		} else {
			msg := "Unable to save lightsensor, please correct errors"
			log.Println(msg)
			msgsNew = append(msgsNew, msg)
		}
		msgs = append(msgs, msgsNew...)

		// Store sunscreen config
		msgsNew = updateSunscreen(req)
		if len(msgsNew) == 0 {
			SaveToJSON(s, fileSunscrn)
			log.Println("Saved sunscreen")
		} else {
			msg := "Unable to save Sunscreen, please correct errors"
			log.Println(msg)
			msgsNew = append(msgsNew, msg)
		}
		msgs = append(msgs, msgsNew...)

		//Store general config
		msgsNew = updateConfig(req)
		if len(msgsNew) == 0 {
			SaveToJSON(config, fileConfig)
			log.Println("Saved general config")
		} else {
			msg := "Unable to save general config, please correct errors"
			log.Println(msg)
			msgsNew = append(msgsNew, msg)
		}
		msgs = append(msgs, msgsNew...)

		log.Println("Updated configuration")
	}

	data := struct {
		Sunscreen
		LightSensor
		Config
		Msgs []string
	}{
		*s,
		*ls,
		config,
		msgs,
	}

	err = tpl.ExecuteTemplate(w, "config.gohtml", data)
	if err != nil {
		log.Panic(err)
	}
}

func handlerLogin(w http.ResponseWriter, req *http.Request) {
	if alreadyLoggedIn(req) {
		http.Redirect(w, req, "/", http.StatusSeeOther)
		return
	}

	ip := getIP(req)

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

	stats := readCSV(fileStats)
	muConf.Lock()
	if len(stats) != 0 {
		stats = stats[MaxIntSlice(0, len(stats)-config.MoveHistory):]
	}
	var lighHistory int
	muLS.Lock()
	muSunscrn.Lock()
	if ls != nil {
		lighHistory = len(ls.Data)
	}
	data := struct {
		S            Sunscreen
		LS           LightSensor
		Time         string
		RefreshRate  time.Duration
		Stats        [][]string
		MoveHistory  int
		LightHistory int
	}{
		*s,
		*ls,
		time.Now().Format("_2 Jan 06 15:04:05"),
		config.RefreshRate, //int(config.RefreshRate.Seconds()),
		reverseXSS(stats),
		config.MoveHistory,
		lighHistory,
	}
	muSunscrn.Unlock()
	muLS.Unlock()
	muConf.Unlock()
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
	stats := readCSV(fileLight)
	muConf.Lock()
	if len(stats) != 0 {
		stats = stats[MaxIntSlice(0, len(stats)-config.LogRecords):]
	}
	data := struct {
		Stats [][]string
	}{
		reverseXSS(stats),
	}
	muConf.Unlock()
	err := tpl.ExecuteTemplate(w, "light.gohtml", data)
	if err != nil {
		log.Fatalln(err)
	}
}

// HandlerMode sets the mode for the sunscreen, i.e. auto, or manual
func handlerMode(w http.ResponseWriter, req *http.Request) {
	if !alreadyLoggedIn(req) {
		http.Redirect(w, req, "/login", http.StatusSeeOther)
		return
	}
	// Url options: '/mode/auto" or '/mode/manual/up' or '/mode/manual/down'
	idMode := req.URL.Path[len("/mode/"):]
	mode := idMode[strings.Index(idMode, "/")+1:]
	muSunscrn.Lock()
	switch mode {
	case auto:
		if s.Mode != auto {
			s.Mode = auto
			SaveToJSON(s, fileSunscrn)
			log.Printf("Set mode to auto (%v)\n", s.Mode)
		} else {
			log.Printf("Mode is already auto (%v)\n", s.Mode)
		}
	case manual + "/" + up:
		if s.Mode != manual {
			s.Mode = manual
			SaveToJSON(s, fileSunscrn)
		}
		go s.Up()
	case manual + "/" + down:
		if s.Mode != manual {
			s.Mode = manual
			SaveToJSON(s, fileSunscrn)
		}
		go s.Down()
	default:
		log.Println("Unknown mode:", req.URL.Path)
	}
	muSunscrn.Unlock()
	// TODO: remove this log? log.Println("Mode=", site.Sunscreens[i].Mode, "and Position=", site.Sunscreens[i].Position)
	http.Redirect(w, req, "/", http.StatusFound)
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

func sliceToString(xs []string) string {
	return strings.Join(xs, ",")
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
	muConf.Lock()
	username := config.Username
	muConf.Unlock()
	if un != username {
		// Unknown cookie
		return false
	}
	return true
}

// StoTime receives a string of time (format hh:mm) and a day offset, and returns a type time with today's and the supplied hours and minutes + the offset in days
func stoTime(t string, days int) (time.Time, error) {
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

func updateSunscreen(req *http.Request) []string {
	muSunscrn.Lock()
	if s == nil {
		s = &Sunscreen{}
	}
	var msgs []string
	appendMsgs := func(msg string) {
		msgs = append(msgs, msg)
		log.Println(msg)
	}
	if req.PostFormValue("AutoStart") == "" {
		s.AutoStart = false
		start, err := stoTime(req.PostFormValue("Start"), 0)
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
		stop, err := stoTime(req.PostFormValue("Stop"), 0)
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
	stopLimit, err := time.ParseDuration(req.PostFormValue("StopLimit") + "m")
	if err != nil {
		appendMsgs(fmt.Sprintf("Unable to save StopLimit '%v' (%v)", stopLimit, err))
	} else {
		s.StopLimit = stopLimit
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
	muSunscrn.Unlock()
	return msgs
}

// UpdateLightsensor reads, validates and stores light sensor
func updateLightsensor(req *http.Request) []string {
	var msgs []string
	appendMsgs := func(msg string) {
		msgs = append(msgs, msg)
		log.Println(msg)
	}
	muLS.Lock()
	good, err := strToInt(req.PostFormValue("Good"))
	if err != nil {
		appendMsgs(fmt.Sprintf("Unable to save Light Good Value: %v", err))
	}
	neutral, err := strToInt(req.PostFormValue("Neutral"))
	if err != nil {
		appendMsgs(fmt.Sprintf("Unable to save Light Neutral Value: %v", err))
	}
	bad, err := strToInt(req.PostFormValue("Bad"))
	if err != nil {
		appendMsgs(fmt.Sprintf("Unable to save Light Bad Value: %v", err))
	}
	if (good < neutral && neutral < bad) && err == nil {
		ls.Good = good
		ls.Neutral = neutral
		ls.Bad = bad
	} else {
		if err != nil {
			appendMsgs(fmt.Sprintf("Error while reading light values: %v", err))
		} else {
			appendMsgs(fmt.Sprintf("Light values incorrect, (good<neutral<bad): %v<%v<%v", good, neutral, bad))
		}
	}
	// Light Threshold
	{
		LightMin := 5
		timesGood, err := strToInt(req.PostFormValue("TimesGood"))
		if err != nil {
			appendMsgs(fmt.Sprintf("Error reading Light Times Good: %v", err))
		} else {
			if timesGood < LightMin {
				appendMsgs(fmt.Sprintf("Light Times Good should be minimum %v (was %v)", LightMin, timesGood))
				timesGood = LightMin
			}
			ls.TimesGood = timesGood
		}
		timesNeutral, err := strToInt(req.PostFormValue("TimesNeutral"))
		if err != nil {
			appendMsgs(fmt.Sprintf("Error reading Light Times Neutral: %v", err))
		} else {
			if timesNeutral < LightMin {
				appendMsgs(fmt.Sprint("Light Times Neutral should be minimum %v (was %v)", LightMin, timesNeutral))
				timesNeutral = LightMin
			}
			ls.TimesNeutral = timesNeutral
		}
		timesBad, err := strToInt(req.PostFormValue("TimesBad"))
		if err != nil {
			appendMsgs(fmt.Sprintf("Error reading Light Times Bad: %v", err))
		} else {
			if timesBad < LightMin {
				appendMsgs(fmt.Sprint("Light Times Bad should be minimum %v (was %v)", LightMin, timesBad))
				timesBad = LightMin
			}
			ls.TimesBad = timesBad
		}
	}
	outliers, err := strToInt(req.PostFormValue("Outliers"))
	if err != nil {
		appendMsgs(fmt.Sprintf("Error reading Outliers ('%v'): %v", outliers, err))
	} else {
		ls.Outliers = outliers
	}
	lightFactor, err := strToInt(req.PostFormValue("LightFactor"))
	if err != nil || lightFactor == 0 {
		appendMsgs(fmt.Sprintf("LightFactor (%v) should a number greater than zero: %v", lightFactor, err))
	} else {
		ls.LightFactor = lightFactor
	}
	pin, err := strToInt(req.PostFormValue("PinLight"))
	if !(pin > 0 && pin < 28) || err != nil {
		appendMsgs(fmt.Sprintf("Unable to save Led Pin '%v' (%v)", pin, err))
	} else {
		ls.Pin = rpio.Pin(pin)
	}
	interval, err := time.ParseDuration(req.PostFormValue("Interval") + "s")
	if err != nil || interval < IntervalMin {
		appendMsgs(fmt.Sprintf("Unable to save Interval '%v', should be minimal %v seconds (%v)", interval, IntervalMin, err))
	} else {
		ls.Interval = interval
	}
	muLS.Unlock()
	return msgs
}

// UpdateConfig reads, validates and stores config
func updateConfig(req *http.Request) []string {
	var msgs []string
	appendMsgs := func(msg string) {
		msgs = append(msgs, msg)
		log.Println(msg)
	}
	muConf.Lock()
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
	config.Cert = req.PostFormValue("Cert")
	config.Key = req.PostFormValue("Key")
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
	muConf.Unlock()
	return msgs
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

// Append CSV takes a filename and adds the new lines to the corresponding CSV file
func appendCSV(file string, newLines [][]string) {
	lines := readCSV(file)
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

// GetIP gets a requests IP address by reading off the forwarded-for
// header (for proxies) and falls back to use the remote address.
func getIP(req *http.Request) string {
	forwarded := req.Header.Get("X-FORWARDED-FOR")
	if forwarded != "" {
		return forwarded
	}
	return req.RemoteAddr
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

func stringToSlice(s string) []string {
	xs := strings.Split(s, ",")
	for i, v := range xs {
		xs[i] = strings.Trim(v, " ")
	}
	return xs
}

func handlerStop(w http.ResponseWriter, req *http.Request) {
	if !alreadyLoggedIn(req) {
		http.Redirect(w, req, "/login", http.StatusSeeOther)
		return
	}

	log.Println("Closing down...")
	s.Up()
	rpio.Close()
	log.Println("Shutting down")
	os.Exit(3)
}
