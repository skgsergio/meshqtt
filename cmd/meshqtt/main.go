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
	flag.Var(&keys, "channel-key", "Channel keys in format ChannelName:Base64Key (can be specified multiple times)")
	flag.Parse()

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
