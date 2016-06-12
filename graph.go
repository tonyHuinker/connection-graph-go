package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
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

func main() {
	//kasdjf;laksj
	days := askForInput("How many days of lookback?")
	daysINT, _ := strconv.Atoi(days)
	lookback := daysINT * 86400000
	myhop := ehop.NewEDAfromKey("keys")
	resp, _ := ehop.CreateEhopRequest("GET", "devices", "null", myhop)
	defer resp.Body.Close()
	var devices []ehop.Device
	temp, _ := ioutil.ReadAll(resp.Body)
	error := json.NewDecoder(bytes.NewReader([]byte(temp))).Decode(&devices)
	if error != nil {
		fmt.Println(error.Error())
	} else {
		var GraphData = make(map[int]source)
		deviceStringArray := "["
		for _, device := range devices {
			if device.IsL3 {
				var hold source
				hold.IP = device.Ipaddr4
				if device.DNSName != "" {
					hold.Hostname = device.DNSName
				}
				//hold.Connect = make([]connection, 1)
				GraphData[device.ID] = hold
				deviceStringArray += strconv.Itoa(device.ID) + ","
			}
		}
		fmt.Println("made it this far")
		deviceStringArray = deviceStringArray[0 : len(deviceStringArray)-1]
		deviceStringArray += "]"
		query := `{ "cycle": "1hr", "from": ` + strconv.Itoa(lookback) + `, "metric_category": "app_detail", "metric_specs": [ { "name": "bytes_in" }, { "name": "bytes_out" } ], "object_ids":` + deviceStringArray + `, "object_type": "device", "until": 0 } } }`
		resp, _ = ehop.CreateEhopRequest("POST", "metrics/totalbyobject", query, myhop)
		temp, _ = ioutil.ReadAll(resp.Body)
		var metrics ehop.MetricsTotalByGroup
		error = json.NewDecoder(bytes.NewReader([]byte(temp))).Decode(&metrics)
		if error != nil {
			fmt.Println(error.Error())
		} else {
			for _, stat := range metrics.Stats {
				for _, second := range stat.Values {
					for _, value := range second {
						for _, peer := range value.Value {
							var hold2 connection
							//			port					ipaddr4				Host						bytes
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
							//fmt.Println(value.Key.Str + "," + peer.Key.Addr + "," + peer.Key.Host + ",bytes," + strconv.Itoa(peer.Value) + "\n")
						}
					}
				}
			}

			for id := range GraphData {
				for _, nextion := range GraphData[id].Connect {
					fmt.Println("Source IP= " + GraphData[id].IP + " Hostname=" + GraphData[id].Hostname + " Protocol=" + nextion.Port + " Peer IP=" + nextion.IP + " Peer Hostname=" + nextion.Host + " Bytes=" + strconv.Itoa(nextion.Bytes))
				}
			}
		}
	}
}
