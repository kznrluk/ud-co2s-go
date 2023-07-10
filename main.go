package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/tarm/serial"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"
)

type UDCO2S struct {
	Dev string
	sync.Mutex
	Logs []LogData
}

type Status struct {
	Co2ppm      string `json:"co2ppm"`
	Humidity    string `json:"humidity"`
	Temperature string `json:"temperature"`
}

type LogData struct {
	Time   int64  `json:"time"`
	Status Status `json:"status"`
}

type GraphiteMetric struct {
	Name     string   `json:"name"`
	Interval int      `json:"interval"`
	Value    float64  `json:"value"`
	Tags     []string `json:"tags"`
	Time     int64    `json:"time"`
}

const (
	LockFilePath = "/tmp/udco2s.lock"
	ApiKey       = "1081715:glc_"
	ApiUrl       = "https://graphite-prod-10-prod-us-central-0.grafana.net/graphite/metrics"
)

func main() {
	if checkInstanceRunning() {
		log.Fatal("An instance is already running")
	}

	createLockFile()

	u := &UDCO2S{Dev: "/dev/ttyACM0"}
	u.StartLogging()

	removeLockFile()
}

func checkInstanceRunning() bool {
	if _, err := os.Stat(LockFilePath); os.IsNotExist(err) {
		return false
	}
	return true
}

func createLockFile() {
	_, err := os.Create(LockFilePath)
	if err != nil {
		log.Fatal(err)
	}
}

func removeLockFile() {
	err := os.Remove(LockFilePath)
	if err != nil {
		log.Fatal(err)
	}
}

func (u *UDCO2S) StartLogging() {
	re := regexp.MustCompile(`CO2=(?P<co2>\d+),HUM=(?P<hum>\d+\.\d+),TMP=(?P<tmp>-?\d+\.\d+)`)
	c := &serial.Config{Name: u.Dev, Baud: 115200}
	s, err := serial.OpenPort(c)
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
				Status: Status{
					Co2ppm:      matches[1],
					Humidity:    matches[2],
					Temperature: matches[3],
				},
			}

			u.Lock()
			u.Logs = append(u.Logs, *obj)
			if len(u.Logs) >= 10 {
				go u.postToGrafana()
			}
			u.Unlock()

			objJson, err := json.Marshal(obj)
			if err != nil {
				log.Fatal(err)
			}

			fmt.Println(string(objJson))
		}
		time.Sleep(1 * time.Second)
	}
}

func (u *UDCO2S) postToGrafana() {
	u.Lock()
	defer u.Unlock()

	metrics := make([]GraphiteMetric, 0, len(u.Logs)*3)

	for _, log := range u.Logs {
		co2ppm, _ := strconv.ParseFloat(log.Status.Co2ppm, 64)
		humidity, _ := strconv.ParseFloat(log.Status.Humidity, 64)
		temperature, _ := strconv.ParseFloat(log.Status.Temperature, 64)

		metrics = append(metrics,
			GraphiteMetric{"co2.metric", 10, co2ppm, []string{"source=udco2s"}, log.Time},
			GraphiteMetric{"humidity.metric", 10, humidity, []string{"source=udco2s"}, log.Time},
			GraphiteMetric{"temperature.metric", 10, temperature, []string{"source=udco2s"}, log.Time},
		)
	}

	jsonValue, _ := json.Marshal(metrics)

	var bearer = "Bearer " + ApiKey
	req, err := http.NewRequest("POST", ApiUrl, bytes.NewBuffer(jsonValue))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", bearer)
	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		log.Println("Failed to post metrics to Grafana", err)
		return
	}

	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	log.Println("Response Body from Grafana:", string(body))

	// Clear the logs after successful post
	u.Logs = nil
}
