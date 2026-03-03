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
	nodeInclude  map[uint32]struct{}
	nodeExclude  map[uint32]struct{}

	hideEmpty     bool
	hideEncrypted bool
}

// activeFilters is configured at startup from CLI flags.
var activeFilters filters

func parseFilters(portExpr, hopExpr, nodeExpr string, hideEmpty, hideEncrypted bool) (filters, error) {
	var f filters
	f.hideEmpty = hideEmpty
	f.hideEncrypted = hideEncrypted

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
		inc, exc, err := parseNodeIDs(nodeExpr)
		if err != nil {
			return f, err
		}
		f.nodeInclude = inc
		f.nodeExclude = exc
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
	// Explicit payload-type filters
	if f.hideEmpty && decoded == nil && packet.GetEncrypted() == nil {
		return false
	}
	if f.hideEncrypted && decoded == nil && packet.GetEncrypted() != nil {
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
	if len(f.portsInclude) > 0 || len(f.portsExclude) > 0 {
		if decoded == nil {
			// Can't tell the portnum if we failed to decode – treat as non-match.
			return false
		}

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
	}

	// Node filter: match if either From or To is in the configured set.
	if len(f.nodeInclude) > 0 || len(f.nodeExclude) > 0 {
		from := packet.GetFrom()
		to := packet.GetTo()

		// Include list: if set, only allow listed nodes.
		if len(f.nodeInclude) > 0 {
			if _, ok := f.nodeInclude[from]; !ok {
				if _, ok2 := f.nodeInclude[to]; !ok2 {
					return false
				}
			}
		}

		// Exclude list: if set, drop listed nodes.
		if len(f.nodeExclude) > 0 {
			if _, ok := f.nodeExclude[from]; ok {
				return false
			}
			if _, ok := f.nodeExclude[to]; ok {
				return false
			}
		}
	}

	return true
}

func parseNodeIDs(expr string) (map[uint32]struct{}, map[uint32]struct{}, error) {
	inc := make(map[uint32]struct{})
	exc := make(map[uint32]struct{})
	parts := strings.Split(expr, ",")

	for _, raw := range parts {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}

		neg := strings.HasPrefix(s, "!")
		if neg {
			s = strings.TrimSpace(s[1:])
		}

		if len(s) == 0 {
			return nil, nil, fmt.Errorf("invalid node ID %q", raw)
		}

		n, err := strconv.ParseUint(s, 16, 32)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid node ID %q", raw)
		}

		if neg {
			exc[uint32(n)] = struct{}{}
		} else {
			inc[uint32(n)] = struct{}{}
		}
	}

	return inc, exc, nil
}

