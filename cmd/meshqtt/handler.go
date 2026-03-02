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
	channelName := parts[len(parts)-2]

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

	if !activeFilters.match(packet, decoded) {
		return
	}

	fmt.Printf("%s %s %s (Channel: %s)\n",
		dim("["+time.Now().Format(time.RFC3339)+"]"),
		bold(cyan("Topic:")),
		cyan(msg.Topic()),
		channelName,
	)
	fmt.Printf("  %s !%08x -> !%08x\n", bold("Route:"), packet.GetFrom(), packet.GetTo())
	fmt.Printf("  %s %d\n", bold("ID:"), packet.GetId())
	fmt.Printf("  %s %d\n", bold("Hop:"), packet.GetHopLimit())

	if decoded != nil {
		portnum := decoded.GetPortnum()
		payload := decoded.GetPayload()
		fmt.Printf("  %s %s (%d)\n", bold("Packet:"), magenta(portnum.String()), portnum)

		switch portnum {
		case pb.PortNum_TEXT_MESSAGE_APP:
			fmt.Printf("  %s %s\n", bold(green("Text:")), string(payload))

		case pb.PortNum_POSITION_APP:
			var pos pb.Position
			if err := proto.Unmarshal(payload, &pos); err == nil {
				fmt.Printf("  %s Lat=%d, Lon=%d, Alt=%d\n",
					bold(green("Position:")),
					pos.GetLatitudeI(), pos.GetLongitudeI(), pos.GetAltitude(),
				)
			}

		case pb.PortNum_NODEINFO_APP:
			var user pb.User
			if err := proto.Unmarshal(payload, &user); err == nil {
				fmt.Printf("  %s LongName=%s, ShortName=%s, ID=%s\n",
					bold(green("User:")),
					user.GetLongName(), user.GetShortName(), user.GetId(),
				)
			}

		case pb.PortNum_TELEMETRY_APP:
			var tel pb.Telemetry
			if err := proto.Unmarshal(payload, &tel); err == nil {
				fmt.Printf("  %s %s\n", bold(green("Telemetry:")), tel.String())
			}

		case pb.PortNum_TRACEROUTE_APP:
			fmt.Printf("  %s %s\n", bold(green("Traceroute:")), decoded.String())

		case pb.PortNum_ROUTING_APP:
			var routing pb.Routing
			if err := proto.Unmarshal(payload, &routing); err == nil {
				fmt.Printf("  %s %s\n", bold(green("Routing:")), routing.String())
			}

		case pb.PortNum_ADMIN_APP:
			var admin pb.AdminMessage
			if err := proto.Unmarshal(payload, &admin); err == nil {
				fmt.Printf("  %s %s\n", bold(green("Admin:")), admin.String())
			}

		case pb.PortNum_WAYPOINT_APP:
			var wp pb.Waypoint
			if err := proto.Unmarshal(payload, &wp); err == nil {
				fmt.Printf("  %s %s\n", bold(green("Waypoint:")), wp.String())
			}

		default:
			// Unknown / unhandled port number but we did manage to decode a Data message.
			// Show that something is there even if we don't have a specific formatter yet.
			fmt.Printf("  %s %s\n", bold(yellow("Decoded (raw):")), decoded.String())
		}
	} else if packet.GetEncrypted() != nil {
		fmt.Printf("  %s <encrypted> (%d bytes)\n", bold(yellow("Payload:")), len(packet.GetEncrypted()))
	} else {
		// Neither decoded data nor encrypted bytes: a header-only / control packet.
		fmt.Printf("  %s (no payload)\n", bold(yellow("Payload:")))
	}

	fmt.Println()
}
