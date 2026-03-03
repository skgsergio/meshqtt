package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	broker := flag.String("broker", "mqtt.meshtastic.org:1883", "MQTT broker URL")
	user := flag.String("user", "meshdev", "MQTT user name")
	password := flag.String("password", "large4cats", "MQTT password")
	topic := flag.String("topic", "msh/EU_868/2/e/#", "MQTT topic to subscribe to")
	clientID := flag.String("client-id", fmt.Sprintf("meshqtt-%d", time.Now().Unix()), "MQTT client ID")
	portFilter := flag.String("filter-port", "", "Comma-separated list of port names or numbers (e.g. TEXT_MESSAGE_APP,POSITION_APP,1,3). Supports negation with ! (e.g. !TELEMETRY_APP)")
	hopFilter := flag.String("filter-hop", "", "Hop filter expression, e.g. \">0\", \"<=3\", \"==1\"")
	nodeFilter := flag.String("filter-node", "", "Comma-separated list of hex node IDs (without ! or 0x). Matches if From or To equals any of them. Supports negation with ! (e.g. !9e7734d4 to exclude)")
	filterEmpty := flag.Bool("filter-empty", false, "Filter out packets with no payload")
	filterEncrypted := flag.Bool("filter-encrypted", false, "Filter out packets that are still encrypted")
	flag.Var(&keys, "channel-key", "Channel keys in format ChannelName:Base64Key (can be specified multiple times)")
	flag.Parse()

	// Configure filtering based on CLI flags.
	if f, err := parseFilters(*portFilter, *hopFilter, *nodeFilter, *filterEmpty, *filterEncrypted); err != nil {
		log.Fatalf("invalid filters: %v", err)
	} else {
		activeFilters = f
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(*broker)
	opts.SetUsername(*user)
	opts.SetPassword(*password)
	opts.SetClientID(*clientID)
	opts.SetAutoReconnect(true)

	opts.OnConnect = func(c mqtt.Client) {
		fmt.Printf("Connected to %s\n", *broker)
		if token := c.Subscribe(*topic, 0, onMessage); token.Wait() && token.Error() != nil {
			fmt.Printf("Warning: Error subscribing to %s: %v\n", *topic, token.Error())
		} else {
			fmt.Printf("Subscribed to %s\n", *topic)
		}
	}

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("Error connecting to MQTT: %v", token.Error())
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	client.Disconnect(250)
	fmt.Println("Disconnected")
}
