package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/tonyHuinker/ehop"
)

type source struct {
	Hostname string
	IP       string
	Connect  []connection
}

type connection struct {
	Port  string
	Host  string
	IP    string
	Bytes int
}

func askForInput(prompt string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println(prompt)
	response, _ := reader.ReadString('\n')
	fmt.Println("\nThank You")
	return strings.TrimSpace(response)
}

func longRequest(query string, out chan *http.Response, myhop *ehop.EDA) {
	resp, _ := ehop.CreateEhopRequest("POST", "metrics/totalbyobject", query, myhop)
	out <- resp
}

func main() {
	//Get number of days (to * by ms) to add to
	days := askForInput("How many days of lookback?")
	daysINT, _ := strconv.Atoi(days)
	lookback := daysINT * 86400000

	//Specify Key File
	keyFile := askForInput("What is the name of your keyFile?")
	myhop := ehop.NewEDAfromKey(keyFile)

	//Get all devices from the system
	resp, _ := ehop.CreateEhopRequest("GET", "devices", "null", myhop)
	defer resp.Body.Close()
	var devices []ehop.Device

	//Put into struct
	error := json.NewDecoder(resp.Body).Decode(&devices)
	if error != nil {
		fmt.Println(error.Error())
		os.Exit(-1)
	}
	fmt.Println("Grabbed Devices Successfully")

	//GraphData is Data Structure to store info
	var GraphData = make(map[int]source)

	//Graph all L3 devices... put into an array for the req.body
	deviceStringArray := "["
	for _, device := range devices {
		if device.IsL3 {
			var hold source
			hold.IP = device.Ipaddr4
			if device.DNSName != "" {
				hold.Hostname = device.DNSName
			}
			GraphData[device.ID] = hold
			deviceStringArray += strconv.Itoa(device.ID) + ","
		}
	}

	//Remove last comma and add closing bracket
	deviceStringArray = deviceStringArray[0 : len(deviceStringArray)-1]
	deviceStringArray += "]"

	//Create req body for metrics/totalbyobject call
	query := `{ "cycle": "1hr", "from": ` + strconv.Itoa(lookback) + `, "metric_category": "app_detail", "metric_specs": [ { "name": "bytes_in" }, { "name": "bytes_out" } ], "object_ids":` + deviceStringArray + `, "object_type": "device", "until": 0 } } }`

	//Make call
	resp, error = ehop.CreateEhopRequest("POST", "metrics/totalbyobject", query, myhop)
	defer resp.Body.Close()
	if error != nil {
		fmt.Println("Bummer.... big long request didn't make it back..." + error.Error())
		os.Exit(-1)
	}

	//Store into Structs
	var metrics ehop.MetricsTotalByGroup
	error = json.NewDecoder(resp.Body).Decode(&metrics)
	if error != nil {
		fmt.Println("Hmm... problem with the json decoding... " + error.Error())
		htmlData, _ := ioutil.ReadAll(resp.Body)
		w, _ := os.Create("bad_input.json")
		io.WriteString(w, string(htmlData))
		w.Close()
		os.Exit(-1)
	}
	fmt.Println("Made metrics call and saved metrics successfully")

	//Go through response struct... and move values into our GraphData obect
	for _, stat := range metrics.Stats {
		for _, second := range stat.Values {
			for _, value := range second {
				for _, peer := range value.Value {
					var hold2 connection
					hold2.Port = value.Key.Str
					if peer.Key.Host != "" {
						hold2.Host = peer.Key.Host
					} else {
						hold2.Host = "No Host Saved"
					}
					hold2.Bytes = peer.Value
					hold2.IP = peer.Key.Addr
					hold3 := GraphData[stat.OID]
					hold3.Connect = append(hold3.Connect, hold2)
					GraphData[stat.OID] = hold3
				}
			}
		}
	}

	f, _ := os.Create("graphCSV.csv")
	//Go through GraphData object.. and print stuff to screen and output to CSV
	io.WriteString(f, "Machine 1 IP, Machine 1 Hostname, Protocol, Machine 2 IP, Machine 2 Hostname, Bytes\n")
	for id := range GraphData {
		for _, nextion := range GraphData[id].Connect {
			io.WriteString(f, GraphData[id].IP+","+GraphData[id].Hostname+","+nextion.Port+","+nextion.IP+","+nextion.Host+","+strconv.Itoa(nextion.Bytes)+"\n")
		}
	}
	f.Close()
}
