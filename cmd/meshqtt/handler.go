package main

import (
	"fmt"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"google.golang.org/protobuf/proto"

	pb "github.com/skgsergio/meshqtt/internal/protobufs"
)

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

