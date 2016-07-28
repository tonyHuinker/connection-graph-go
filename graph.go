package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
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

func addToGraphData(GraphData map[int]source, metrics ehop.MetricsTotalByGroup) map[int]source {
	//Go through response struct... and move values into our GraphData obect
	for _, stat := range metrics.Stats {
		for _, second := range stat.Values {
			for _, value := range second {
				for _, peer := range value.Value {
					if value.Key.Str == "HostedServices" {
						var hold2 connection
						hold2.Port = peer.Key.Str
						hold2.Bytes = peer.Value
						hold3 := GraphData[stat.OID]
						hold3.Connect = append(hold3.Connect, hold2)
						GraphData[stat.OID] = hold3

					}
				}
			}
		}
	}

	return GraphData
}

func main() {
	//Get number of days (to * by ms) to add to
	days := askForInput("How many days of lookback?")
	daysINT, _ := strconv.Atoi(days)
	lookback := daysINT * -86400000

	//Specify Key File
	keyFile := askForInput("What is the name of your keyFile?")
	myhop := ehop.NewEDAfromKey(keyFile)

	//Get all devices from the system
	resp, _ := ehop.CreateEhopRequest("GET", "devices?active_from="+strconv.Itoa(lookback), "null", myhop)
	defer resp.Body.Close()
	var devices []ehop.Device

	//Put into struct
	error := json.NewDecoder(resp.Body).Decode(&devices)
	if error != nil {
		fmt.Println(error.Error())
		os.Exit(-1)
	}

	//GraphData is Data Structure to store info
	var GraphData = make(map[int]source)

	//Grab all L3 devices... put into an array for the req.body
	var deviceIDArray []int
	for _, device := range devices {
		if device.IsL3 {
			var hold source
			hold.IP = device.Ipaddr4
			if device.DNSName != "" {
				hold.Hostname = device.DNSName
			}
			GraphData[device.ID] = hold
			deviceIDArray = append(deviceIDArray, device.ID)
		}
	}
	fmt.Println("Grabbed " + strconv.Itoa(len(deviceIDArray)) + " L3 Devices Successfully")

	//loop through devices.. make queries in batches of 500
	deviceStringArray := "["
	x := 0
	for i, dID := range deviceIDArray {
		deviceStringArray += strconv.Itoa(dID) + ","
		x++
		if x > 499 {
			//Remove last comma and add closing bracket
			deviceStringArray = deviceStringArray[0 : len(deviceStringArray)-1]
			deviceStringArray += "]"

			//Create req body for metrics/totalbyobject call
			fmt.Println("Making request for 500 devices, there are " + strconv.Itoa(len(deviceIDArray)-i) + " devices left")
			query := `{ "cycle": "auto", "from": -86400000 , "metric_category": "custom_detail", "metric_specs": [ { "name": "HostedServices" }], "object_ids":` + deviceStringArray + `, "object_type": "device", "until": 0 } } }`
			//query := `{ "cycle": "auto", "from": ` + strconv.Itoa(lookback) + `, "metric_category": "custom_detail", "metric_specs": [ { "name": "HostedServices" }], "object_ids":` + deviceStringArray + `, "object_type": "device", "until": 0 } } }`
			resp, error = ehop.CreateEhopRequest("POST", "metrics/totalbyobject", query, myhop)
			//Make call
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
				os.Exit(-1)
			}
			fmt.Println("Made metrics call and saved metrics successfully")
			GraphData = addToGraphData(GraphData, metrics)
			x = 0
			deviceStringArray = "["
		}
	}
	deviceStringArray = deviceStringArray[0 : len(deviceStringArray)-1]
	deviceStringArray += "]"

	fmt.Println("Making last request for devices")
	//Create req body for metrics/totalbyobject call
	query := `{ "cycle": "auto", "from": -86400000 , "metric_category": "custom_detail", "metric_specs": [ { "name": "HostedServices" }], "object_ids":` + deviceStringArray + `, "object_type": "device", "until": 0 } } }`
	//query := `{ "cycle": "auto", "from": ` + strconv.Itoa(lookback) + `, "metric_category": "custom_detail", "metric_specs": [ { "name": "HostedServices" }], "object_ids":` + deviceStringArray + `, "object_type": "device", "until": 0 } } }`
	//query := `{ "cycle": "1hr", "from": ` + strconv.Itoa(lookback) + `, "metric_category": "app_detail", "metric_specs": [ { "name": "bytes_in" }, { "name": "bytes_out" } ], "object_ids":` + deviceStringArray + `, "object_type": "device", "until": 0 } } }`
	resp, error = ehop.CreateEhopRequest("POST", "metrics/totalbyobject", query, myhop)
	//Make call
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
		os.Exit(-1)
	}
	fmt.Println("Made metrics call and saved metrics successfully")
	GraphData = addToGraphData(GraphData, metrics)

	//htmlData, _ := ioutil.ReadAll(resp.Body)
	//	fmt.Println(string(htmlData))
	//w, _ := os.Create("bad_input.json")
	//io.WriteString(w, string(htmlData))
	//w.Close()

	f, _ := os.Create("graphCSV.csv")
	//Go through GraphData object.. and print stuff to screen and output to CSV
	io.WriteString(f, "Machine 1 IP, Machine 1 Hostname, Port, Connection Count, Port, Connection Count, Port, Connection Count, Port, Connection Count\n")
	for id := range GraphData {
		io.WriteString(f, GraphData[id].IP+","+GraphData[id].Hostname)
		for _, nextion := range GraphData[id].Connect {
			io.WriteString(f, ","+nextion.Port+","+strconv.Itoa(nextion.Bytes))
		}
		io.WriteString(f, "\n")
	}
	f.Close()
}
