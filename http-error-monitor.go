package main

//
//  http-error-monitor.go  --  A Thunder Cloud Agent (TCA) that watches Syslog records coming in from an A10 Thunder device
//    for logs from aFleX script that is looking for HTTP errors and recording them.  If it sees an HTTP Error, it will
//    report it out via MQTT
//
//  aFlex script that should be applied to the HTTP or WEB API SLB that you want to monitor:
//
// when HTTP_REQUEST {
//     set u [HTTP::uri]
//     set n [IP::client_addr]
// }
//
// when HTTP_RESPONSE {
//     if { [HTTP::status] > 399 & [HTTP::status] < 600 } {
//         log "HTTP Error: $n - [HTTP::status] - $u"
//         if { [HTTP::status] == 404 } {
//             drop
//         }
//     }
// }
//   This aFlex script will catch all 4xx and 5xx HTTP Error codes and log them.  To enable Syslog output, you will
//   need the following lines in your A10 Thunder configuration:
//
// !
// logging syslog information
// ! logging host {IP Addr where program is running} use-mgmt-port port {port to use}
// logging host 10.1.1.12 use-mgmt-port port 5514
// !
//
//   The 'use-mgmt-port' is optional and will send the Syslog messages out via the MGMT interface.
//   If you use the defaults in this program, you will need the 'port 5514' part.
//
//  John D. Allen
//  Global Solutions Architect - Cloud, A10 Networks
//  Apache 2.0 License Applies
//  April, 2021
//

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"

	"os"
	"strconv"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"gopkg.in/mcuadros/go-syslog.v2"
)

// Configuration holds config structure
type Configuration struct {
	Debug        int    `json:"debug"`
	MQTT_Broker  string `json:"mqtt_broker"`
	Client_ID    string `json:"client_id"`
	Syslog_port  int    `json:"syslog_port"`
	MQTT_port    int    `json:"mqtt_port"`
	Notify_Topic string `json:"notify_topic"`
	Username     string `json:"username"`
	Password     string `json:"password"`
}

var config Configuration

var connHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	fmt.Println("MQTT Broker Connected...")
}

func getConfig(fn string) (Configuration, error) {
	jsonFile, err := os.Open(fn)
	if err != nil {
		return Configuration{}, errors.New("Unable to open Config File!")
	}
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)

	var c Configuration
	json.Unmarshal(byteValue, &c)

	return c, nil
}

//
//---------------------------------------------------------------------------------------------------
func main() {
	//
	// Get Config info
	config, err := getConfig("./config.json")
	//fmt.Println(config)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if config.Syslog_port == 0 {
		config.Syslog_port = 514
	}

	//------------------[  MQTT Setup Stuff  ]-----------------------
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("mqtt://%s:%d", config.MQTT_Broker, config.MQTT_port))
	opts.SetClientID(config.Client_ID) // If running multiple clients, this needs to be unique, or remove for defaults
	// -- This code defaults to no Auth being used on the MQTT Broker. Uncomment these two lines for Username/Password Auth
	// opts.SetUsername(config.Username)
	// opts.SetPassword(config.Password)
	// -- TLS Auth requires much more code. See https://github.com/eclipse/paho.mqtt.golang/blob/master/cmd/ssl/main.go for example.
	opts.SetKeepAlive(30) // 30 second keepalive PING for MQTT Broker connection.
	opts.SetOnConnectHandler(connHandler)
	opts.SetAutoReconnect(true)
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	//------------------[  Syslog Setup Stuff  ]---------------------
	channel := make(syslog.LogPartsChannel)
	handler := syslog.NewChannelHandler(channel)
	server := syslog.NewServer()
	server.SetFormat(syslog.RFC3164) // Thunder uses RFC 3164 format for its Syslog records.
	server.SetHandler(handler)
	server.ListenUDP("0.0.0.0:" + strconv.Itoa(config.Syslog_port))
	server.Boot()
	if config.Debug > 5 {
		fmt.Println("HTTP Error Montor running on port " + strconv.Itoa(config.Syslog_port) + "...")
	}

	//------------------[  MAIN  ]-----------------------------
	go func(channel syslog.LogPartsChannel) {
		for logParts := range channel {
			//
			// Log records ("logParts") come in from Thunder looking like this:
			// map[client:10.1.11.44:5456 content:[ACOS]<4> Virtual server ws-vip connection rate limit 10 exceeded facility:16
			//   hostname:Testing1 priority:132 severity:4 tag:a10logd timestamp:2021-05-18 22:03:04 +0000 UTC tls_peer:]
			// map[client:10.1.11.44:5456 content:[AFLEX]<6> http-error-status-log:HTTP Error: 10.147.95.128 - 404 - /blatt
			//   facility:16 hostname:Testing1 priority:134 severity:6 tag:a10logd timestamp:2021-05-18 22:05:41 +0000 UTC tls_peer:]
			if config.Debug > 9 { // Output all incoming Syslog records.
				fmt.Print(".")
				fmt.Println(logParts)
			}
			m := fmt.Sprintf("%s", logParts["content"])
			host := fmt.Sprintf("%s", logParts["hostname"])
			if strings.HasPrefix(m, "[AFLEX]") { // -- Only log lines from AFLEX
				//  Full 'content' field looks like: "[AFLEX]<6> http-error-status-log:HTTP Error: 10.147.95.128 - 404 - /vafc""
				if strings.Contains(m, "http-error-status-log") { // Only process records from our aflex script
					msg := m[33:] // Cut off the prefix and just show the error text.
					text := "A10 Thunder node = " + host + "::" + msg
					if config.Debug > 5 {
						fmt.Println(text)
					}
					token := client.Publish(config.Notify_Topic, 0, false, text)
					token.Wait()
					// Check for Error on Publish
					if token.Error() != nil {
						if config.Debug > 3 {
							fmt.Print(">>> MQTT Publish Error: ")
							fmt.Println(token.Error())
						}
					}
				}
			}
			//fmt.Println(logParts)
		}
	}(channel)

	server.Wait()
}
