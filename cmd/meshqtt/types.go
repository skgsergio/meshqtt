package main

import (
	"encoding/base64"
	"fmt"
	"strings"
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

var defaultKey = []byte{
	0xd4, 0xf1, 0xbb, 0x3a,
	0x20, 0x29, 0x07, 0x59,
	0xf0, 0xbc, 0xff, 0xab,
	0xcf, 0x4e, 0x69, 0x01,
}

