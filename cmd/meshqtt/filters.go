package main

import (
	"fmt"
	"strconv"
	"strings"

	pb "github.com/skgsergio/meshqtt/internal/protobufs"
)

type hopFilter struct {
	active bool
	op     string
	value  uint32
}

type filters struct {
	// portsInclude is an allow-list: if non-empty, only packets with these ports match.
	portsInclude map[pb.PortNum]struct{}
	// portsExclude is a deny-list: if non-empty, packets with these ports are filtered out.
	portsExclude map[pb.PortNum]struct{}
	hop          hopFilter
	nodeIDs      map[uint32]struct{}
}

// activeFilters is configured at startup from CLI flags.
var activeFilters filters

func parseFilters(portExpr, hopExpr, nodeExpr string) (filters, error) {
	var f filters

	portExpr = strings.TrimSpace(portExpr)
	if portExpr != "" {
		f.portsInclude = make(map[pb.PortNum]struct{})
		f.portsExclude = make(map[pb.PortNum]struct{})
		parts := strings.Split(portExpr, ",")
		for _, raw := range parts {
			part := strings.TrimSpace(raw)
			if part == "" {
				continue
			}

			neg := strings.HasPrefix(part, "!")
			if neg {
				part = strings.TrimSpace(part[1:])
				if part == "" {
					return f, fmt.Errorf("invalid port filter %q", raw)
				}
			}

			// Try symbolic name first (e.g. TEXT_MESSAGE_APP)
			name := strings.ToUpper(part)
			if v, ok := pb.PortNum_value[name]; ok {
				if neg {
					f.portsExclude[pb.PortNum(v)] = struct{}{}
				} else {
					f.portsInclude[pb.PortNum(v)] = struct{}{}
				}
				continue
			}

			// Fallback to numeric value
			n, err := strconv.ParseInt(part, 10, 32)
			if err != nil {
				return f, fmt.Errorf("invalid port filter %q", part)
			}
			if neg {
				f.portsExclude[pb.PortNum(n)] = struct{}{}
			} else {
				f.portsInclude[pb.PortNum(n)] = struct{}{}
			}
		}
	}

	hopExpr = strings.TrimSpace(hopExpr)
	if hopExpr != "" {
		hf, err := parseHopFilter(hopExpr)
		if err != nil {
			return f, err
		}
		f.hop = hf
	}

	nodeExpr = strings.TrimSpace(nodeExpr)
	if nodeExpr != "" {
		ids, err := parseNodeIDs(nodeExpr)
		if err != nil {
			return f, err
		}
		f.nodeIDs = ids
	}

	return f, nil
}

func parseHopFilter(expr string) (hopFilter, error) {
	e := strings.TrimSpace(expr)
	if e == "" {
		return hopFilter{}, nil
	}

	var op string
	switch {
	case strings.HasPrefix(e, ">="),
		strings.HasPrefix(e, "<="),
		strings.HasPrefix(e, "=="),
		strings.HasPrefix(e, "!="):
		op = e[:2]
		e = strings.TrimSpace(e[2:])
	case strings.HasPrefix(e, ">"),
		strings.HasPrefix(e, "<"):
		op = e[:1]
		e = strings.TrimSpace(e[1:])
	default:
		return hopFilter{}, fmt.Errorf("invalid hop filter %q", expr)
	}

	n, err := strconv.Atoi(e)
	if err != nil || n < 0 {
		return hopFilter{}, fmt.Errorf("invalid hop filter value %q", e)
	}

	return hopFilter{
		active: true,
		op:     op,
		value:  uint32(n),
	}, nil
}

func (f filters) match(packet *pb.MeshPacket, decoded *pb.Data) bool {
	// When any filter is active, only show packets where we have a decoded
	// Data payload. This hides header-only/control packets and packets that
	// are still encrypted/undecoded from filtered views.
	if (f.hop.active || len(f.portsInclude) > 0 || len(f.portsExclude) > 0 || len(f.nodeIDs) > 0) && decoded == nil {
		return false
	}

	// Hop filter (always available from the packet header)
	if f.hop.active {
		hop := packet.GetHopLimit()
		v := f.hop.value

		switch f.hop.op {
		case ">":
			if !(hop > v) {
				return false
			}
		case "<":
			if !(hop < v) {
				return false
			}
		case ">=":
			if !(hop >= v) {
				return false
			}
		case "<=":
			if !(hop <= v) {
				return false
			}
		case "==":
			if hop != v {
				return false
			}
		case "!=":
			if hop == v {
				return false
			}
		default:
			// Unknown operator: be conservative and drop.
			return false
		}
	}

	// Port filter (requires decoded Data, either from MQTT or after decryption).
	if decoded != nil {
		port := decoded.GetPortnum()

		// Include list: if set, only allow listed ports.
		if len(f.portsInclude) > 0 {
			if _, ok := f.portsInclude[port]; !ok {
				return false
			}
		}

		// Exclude list: if set, drop listed ports.
		if len(f.portsExclude) > 0 {
			if _, ok := f.portsExclude[port]; ok {
				return false
			}
		}
	} else if len(f.portsInclude) > 0 || len(f.portsExclude) > 0 {
		// Can't tell the portnum if we failed to decode – treat as non-match.
		return false
	}

	// Node filter: match if either From or To is in the configured set.
	if len(f.nodeIDs) > 0 {
		from := packet.GetFrom()
		to := packet.GetTo()

		if _, ok := f.nodeIDs[from]; !ok {
			if _, ok2 := f.nodeIDs[to]; !ok2 {
				return false
			}
		}
	}

	return true
}

func parseNodeIDs(expr string) (map[uint32]struct{}, error) {
	out := make(map[uint32]struct{})
	parts := strings.Split(expr, ",")

	for _, raw := range parts {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}

		// Allow IDs in forms like "!9e7734d4", "9e7734d4", "0x9e7734d4".
		s = strings.TrimPrefix(s, "!")
		if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
			s = s[2:]
		}

		if len(s) == 0 {
			return nil, fmt.Errorf("invalid node ID %q", raw)
		}

		n, err := strconv.ParseUint(s, 16, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid node ID %q", raw)
		}

		out[uint32(n)] = struct{}{}
	}

	return out, nil
}

