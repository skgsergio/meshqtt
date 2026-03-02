package main

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"google.golang.org/protobuf/proto"

	pb "github.com/skgsergio/meshqtt/internal/protobufs"
)

type channelKeys map[string][]byte

func (i *channelKeys) String() string {
	return fmt.Sprint(*i)
}

func (i *channelKeys) Set(value string) error {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid format, use ChannelName:Base64Key")
	}
	key, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("invalid base64 key for channel %s: %v", parts[0], err)
	}
	(*i)[parts[0]] = key
	return nil
}

var keys = make(channelKeys)
var defaultKey = []byte{0xd4, 0xf1, 0xbb, 0x3a, 0x20, 0x29, 0x07, 0x59, 0xf0, 0xbc, 0xff, 0xab, 0xcf, 0x4e, 0x69, 0x01}

func main() {
	broker := flag.String("broker", "tcp://mqtt.meshtastic.org:1883", "MQTT broker URL")
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

func decrypt(packet *pb.MeshPacket, key []byte) ([]byte, error) {
	if len(key) == 0 {
		return nil, fmt.Errorf("empty key")
	}

	realKey := key
	// Expansion of single-byte keys per firmware convention
	if len(key) == 1 {
		pskIndex := key[0]
		if pskIndex == 0 {
			return nil, fmt.Errorf("encryption disabled (PSK 0)")
		}
		realKey = make([]byte, 16)
		copy(realKey, defaultKey)
		realKey[15] = realKey[15] + pskIndex - 1
	} else if len(key) < 16 {
		realKey = make([]byte, 16)
		copy(realKey, key)
	} else if len(key) > 16 && len(key) < 32 {
		realKey = make([]byte, 32)
		copy(realKey, key)
	}

	block, err := aes.NewCipher(realKey)
	if err != nil {
		return nil, err
	}

	// IV Construction matching Meshtastic CryptoEngine::initNonce
	// 16 bytes total:
	// [0:8]   Packet ID (64-bit uint, Little Endian)
	// [8:12]  From Node ID (32-bit uint, Little Endian)
	// [12:16] 0x00000000 (Go's cipher.NewCTR treats this as the counter)
	iv := make([]byte, 16)

	// memcpy(nonce, &packetId, sizeof(uint64_t));
	iv[0] = byte(packet.Id)
	iv[1] = byte(packet.Id >> 8)
	iv[2] = byte(packet.Id >> 16)
	iv[3] = byte(packet.Id >> 24)
	iv[4] = 0 // High 32 bits of packetId are zero in MeshPacket.id (fixed32)
	iv[5] = 0
	iv[6] = 0
	iv[7] = 0

	// memcpy(nonce + sizeof(uint64_t), &fromNode, sizeof(uint32_t));
	iv[8] = byte(packet.From)
	iv[9] = byte(packet.From >> 8)
	iv[10] = byte(packet.From >> 16)
	iv[11] = byte(packet.From >> 24)

	// iv[12:16] is already 0 (counter starts at 0)

	stream := cipher.NewCTR(block, iv)
	plaintext := make([]byte, len(packet.GetEncrypted()))
	stream.XORKeyStream(plaintext, packet.GetEncrypted())

	return plaintext, nil
}

func onMessage(client mqtt.Client, msg mqtt.Message) {
	var envelope pb.ServiceEnvelope
	if err := proto.Unmarshal(msg.Payload(), &envelope); err != nil {
		return
	}

	packet := envelope.GetPacket()
	if packet == nil {
		return
	}

	parts := strings.Split(msg.Topic(), "/")
	channelName := ""
	if len(parts) >= 5 {
		channelName = parts[4]
	}

	from := packet.GetFrom()
	to := packet.GetTo()

	fmt.Printf("[%s] Topic: %s (Channel: %s)\n", time.Now().Format(time.RFC3339), msg.Topic(), channelName)
	fmt.Printf("  From: !%08x, To: !%08x, ID: %d, HopLimit: %d\n", from, to, packet.GetId(), packet.GetHopLimit())

	decoded := packet.GetDecoded()
	if decoded == nil && packet.GetEncrypted() != nil {
		var keyToTry []byte
		if k, ok := keys[channelName]; ok {
			keyToTry = k
		} else {
			keyToTry = defaultKey
		}

		if plaintext, err := decrypt(packet, keyToTry); err == nil {
			var d pb.Data
			if err := proto.Unmarshal(plaintext, &d); err == nil {
				decoded = &d
			}
		}
	}

	if decoded != nil {
		portnum := decoded.GetPortnum()
		payload := decoded.GetPayload()
		fmt.Printf("  Packet Type: %s (%d)\n", portnum.String(), portnum)

		switch portnum {
		case pb.PortNum_TEXT_MESSAGE_APP:
			fmt.Printf("  Text: %s\n", string(payload))
		case pb.PortNum_POSITION_APP:
			var pos pb.Position
			if err := proto.Unmarshal(payload, &pos); err == nil {
				fmt.Printf("  Position: Lat=%d, Lon=%d, Alt=%d\n", pos.GetLatitudeI(), pos.GetLongitudeI(), pos.GetAltitude())
			}
		case pb.PortNum_NODEINFO_APP:
			var user pb.User
			if err := proto.Unmarshal(payload, &user); err == nil {
				fmt.Printf("  User: LongName=%s, ShortName=%s, ID=%s\n", user.GetLongName(), user.GetShortName(), user.GetId())
			}
		case pb.PortNum_TELEMETRY_APP:
			var tel pb.Telemetry
			if err := proto.Unmarshal(payload, &tel); err == nil {
				fmt.Printf("  Telemetry: %s\n", tel.String())
			}
		case pb.PortNum_ROUTING_APP:
			var routing pb.Routing
			if err := proto.Unmarshal(payload, &routing); err == nil {
				fmt.Printf("  Routing: %s\n", routing.String())
			}
		case pb.PortNum_ADMIN_APP:
			var admin pb.AdminMessage
			if err := proto.Unmarshal(payload, &admin); err == nil {
				fmt.Printf("  Admin: %s\n", admin.String())
			}
		case pb.PortNum_WAYPOINT_APP:
			var wp pb.Waypoint
			if err := proto.Unmarshal(payload, &wp); err == nil {
				fmt.Printf("  Waypoint: %s\n", wp.String())
			}
		}
	} else if packet.GetEncrypted() != nil {
		fmt.Printf("  Payload: <encrypted> (%d bytes)\n", len(packet.GetEncrypted()))
	}
	fmt.Println()
}
