# MeshQTT

A powerful CLI tool to monitor and filter [Meshtastic](https://meshtastic.org/) traffic via MQTT. `meshqtt` provides rich, color-coded formatting for various Meshtastic message types, including positions, text messages, telemetry, and traceroutes.

> ⚠️ This project was entirely **vivecoded** (a combination of human vibes and AI-driven implementation). Expect a mix of architectural clarity, experimental logic, and a healthy dose of "wait, how does this work?". Use at your own risk.

## Features

- **Rich Formatting:** Color-coded terminal output with links to Google Maps for positions and waypoints.
- **Message Decryption:** Supports on-the-fly decryption if channel keys are provided.
- **Advanced Filtering:**
  - **Port Filter:** Filter by application port names or numbers (e.g., `TEXT_MESSAGE_APP`, `POSITION_APP`, `1`, `3`). Supports negation with `!` (e.g., `!TELEMETRY_APP`).
  - **Node Filter:** Filter by specific hex node IDs (without `0x` or `!`). Matches if either the sender or receiver node ID matches. Supports negation with `!` (e.g., `!9e7734d4` to exclude).
  - **Hop Filter:** Filter based on hop count expressions (e.g., `>0`, `<=3`, `==1`).
  - **Encryption/Empty Status:** Option to filter out packets with no payload.
- **Comprehensive Protobuf Support:** Decodes and displays:
  - Text Messages
  - Position Updates
  - User Info (Node Info)
  - Telemetry (Sensor data)
  - Traceroutes (including full path and SNR)
  - Routing Messages
  - Admin Messages
  - Waypoints

## Installation

Ensure you have [Go](https://golang.org/) 1.25 or later installed.

```bash
go install github.com/skgsergio/meshqtt/cmd/meshqtt@latest
```

Or clone the repository and build manually:

```bash
git clone https://github.com/skgsergio/meshqtt.git
cd meshqtt
go build ./cmd/meshqtt
```

## Usage

Basic usage subscribing to the default broker and topic:

```bash
meshqtt
```

### Command-line Options

| Flag                | Description                                    | Default                    |
|---------------------|------------------------------------------------|----------------------------|
| `-mqtt-broker`      | MQTT broker URL                                | `mqtt.meshtastic.org:1883` |
| `-mqtt-user`        | MQTT user name                                 | `meshdev`                  |
| `-mqtt-password`    | MQTT password                                  | `large4cats`               |
| `-mqtt-topic`       | MQTT topic to subscribe to                     | `msh/EU_868/2/e/#`         |
| `-mqtt-client-id`   | MQTT client ID                                 | `meshqtt-<timestamp>`      |
| `-channel-key`      | Channel keys (Format: `ChannelName:Base64Key`) | -                          |
| `-filter-port`      | Comma-separated list of port names or numbers  | -                          |
| `-filter-node`      | Comma-separated list of hex node IDs           | -                          |
| `-filter-hop`       | Hop filter expression (e.g. `>0`, `<=3`)       | -                          |
| `-filter-empty`     | Filter out packets with no payload             | `false`                    |
| `-filter-encrypted` | Filter out packets that are still encrypted    | `false`                    |

### Examples

**Filter for text messages and positions from a specific node:**

```bash
meshqtt -filter-port TEXT_MESSAGE_APP,POSITION_APP -filter-node !9e7734d4
```

**Filter for messages with more than 1 hop:**

```bash
meshqtt -filter-hop ">1"
```

**Connect to a custom broker and subscribe to a specific channel:**

```bash
meshqtt -mqtt-broker 192.168.1.50:1883 -mqtt-topic "msh/US/2/c/LongFast/#"
```

**Decrypt messages for a specific channel:**

```bash
meshqtt -channel-key "MyChannel:AQIDBAUGBwgJCgsMDQ4PEA=="
```

## License

This project is licensed under the **Better Ask The LLM License (BATL)**. See the [LICENSE](LICENSE) file for details.
