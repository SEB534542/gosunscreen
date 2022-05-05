package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/smtp"
	"os"
	"time"

	"github.com/stianeikeland/go-rpio/v4"
	"golang.org/x/crypto/bcrypt"
)

// Constants for config folder and files
const (
	folderConfig    = "config"
	fileConfig      = "./config/config.json"
	fileSunscrn     = "./config/sunscreen.json"
	fileLightsensor = "./config/lightsensor.json"
	folderLog       = "logs"
	fileLog         = "./logs/logfile.log"
	fileStats       = "./logs/sunscreen_stats.csv"
	fileLight       = "./logs/light_stats.csv"
)

var ls = &LightSensor{}

// Pin:          rpio.Pin(23),
// Interval:     time.Duration(time.Minute),
// LightFactor:  12,
// Start:        time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 8, 0, 0, 0, time.Local),
// Stop:         time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 21, 0, 0, 0, time.Local),
// Good:         10,
// Neutral:      15,
// Bad:          30,
// TimesGood:    17,
// TimesNeutral: 20,
// TimesBad:     6,
// Outliers:     2,
// Data:         []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
// }
var s = &Sunscreen{}

// Id:        1000,
// Name:      "Woonkamer",
// Mode:      auto,
// Position:  unknown,
// DurDown:   time.Duration(16 * time.Second),
// DurUp:     time.Duration(19 * time.Second),
// PinDown:   rpio.Pin(21),
// PinUp:     rpio.Pin(20),
// AutoStart: true,
// AutoStop:  true,
// SunStart:  time.Duration(9720000000000),
// SunStop:   time.Duration(5400000000000),
// Start:     time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 8, 0, 0, 0, time.Local),
// Stop:      time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 21, 0, 0, 0, time.Local),
// StopLimit: time.Duration(1800000000000),
// }

var config Config

func init() {
	// Check if log folder exists, else create
	if _, err := os.Stat(folderLog); os.IsNotExist(err) {
		os.Mkdir(folderLog, 4096)
	}
	// Check if log folder exists, else create
	if _, err := os.Stat(folderConfig); os.IsNotExist(err) {
		os.Mkdir(folderConfig, 4096)
	}
}

func main() {
	// TODO: check if below can be stored in a separate func
	// Open logfile or create if not exists
	f, err := os.OpenFile(fileLog, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Panic("Error opening log file:", err)
		//  TODO: remove log file and create new
	}
	defer f.Close()
	log.SetOutput(f)
	log.Println("--------Start of program--------")

	// Open connection RPIO pins
	rpio.Open()
	defer rpio.Close()

	loadConfig()
	s.initPins()
	updateStartStop(s, ls, 0)
	// TODO: Include a stop option with below (like greenhouse)
	defer func() {
		log.Println("Closing down...")
		s.Up()
	}()
	log.Println("Starting monitor")
	if ls != nil {
		go ls.MonitorMove(s)
	}
	startServer()
}

// UpdateStartStop resets all start/stop to today + d (e.g. d=0 resets it to today.
func updateStartStop(s *Sunscreen, ls *LightSensor, d int) {
	s.resetStartStop(d)
	// Light sensor should start in time so at sunscreen start enough light has been gathered
	dur := time.Duration((max(ls.TimesGood, ls.TimesNeutral, ls.TimesBad)+ls.Outliers)/int(ls.Interval.Minutes())) * time.Minute
	ls.Start = s.Start.Add(-dur)
	ls.Stop = s.Stop.Add(time.Duration(30 * time.Minute))
}

// Max takes multiple int and returns the highest value. It always returns a minimum of zero.
func max(xi ...int) int {
	var x int
	for _, v := range xi {
		if v > x {
			x = v
		}
	}
	return x
}

// ReadJSON reads from the given json file location and returns any error.
// into i interface.
func readJSON(fname string, i interface{}) error {
	if _, err := os.Stat(fname); os.IsNotExist(err) {
		log.Printf("File '%v' does not exist, creating blank", fname)
		SaveToJSON(i, fname)
	} else {
		data, err := ioutil.ReadFile(fname)
		// TODO: remove file and create new
		if err != nil {
			return fmt.Errorf("%s is corrupt. Please delete the file (%v)", fname, err)
		}
		err = json.Unmarshal(data, i)
		if err != nil {
			return fmt.Errorf("%s is corrupt. Please delete the file (%v)", fname, err)
		}
	}
	return nil
}

// LoadConfig reads the JSON file from fname and does some initial checks.
func loadConfig() {
	// Load config
	err := readJSON(fileConfig, &config)
	if err != nil {
		log.Printf("Error while reading JSON '%v', please manually set config and save", fileConfig)
	}
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
		log.Fatal("Error setting default refreshrate:", err)
	}

	// Load sunscreen
	err = readJSON(fileSunscrn, &s)
	if err != nil {
		log.Fatal(err)
	}

	// Load lightsensor
	err = readJSON(fileLightsensor, &ls)
	if err != nil {
		log.Fatal(err)
	}
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

// SaveToJson takes an interface and stores it into the filename
func SaveToJSON(i interface{}, fileName string) {
	bs, err := json.Marshal(i)
	if err != nil {
		log.Fatal(err)
	}
	err = ioutil.WriteFile(fileName, bs, 0644)
	if err != nil {
		log.Fatal("Error", err)
	}
}
