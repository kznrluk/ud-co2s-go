package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/tarm/serial"
	"log"
	"regexp"
	"time"
)

type UDCO2S struct {
	Dev string
}

type Status struct {
	Co2ppm      string `json:"co2ppm"`
	Humidity    string `json:"humidity"`
	Temperature string `json:"temperature"`
}

type LogData struct {
	Time int64  `json:"time"`
	Stat Status `json:"stat"`
}

func (u *UDCO2S) StartLogging() {
	re := regexp.MustCompile(`CO2=(?P<co2>\d+),HUM=(?P<hum>\d+\.\d+),TMP=(?P<tmp>-?\d+\.\d+)`)
	c := &serial.Config{Name: u.Dev, Baud: 115200}
	s, err := serial.OpenPort(c)
	if err != nil {
		log.Fatal(err)
	}
	_, err = s.Write([]byte("STA\r\n"))
	if err != nil {
		log.Fatal(err)
	}
	_, err = s.Write([]byte("STA\r\n"))
	if err != nil {
		log.Fatal(err)
	}
	reader := bufio.NewReader(s)
	for {
		line, _, err := reader.ReadLine()
		if err != nil {
			log.Fatal(err)
		}
		matches := re.FindStringSubmatch(string(line))
		if matches != nil {
			obj := &LogData{
				Time: time.Now().Unix(),
				Stat: Status{
					Co2ppm:      matches[1],
					Humidity:    matches[2],
					Temperature: matches[3],
				},
			}
			objJson, err := json.Marshal(obj)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(string(objJson))
		}
		time.Sleep(1 * time.Second)
	}
}

func main() {
	u := &UDCO2S{Dev: "/dev/ttyACM0"}
	u.StartLogging()
}
