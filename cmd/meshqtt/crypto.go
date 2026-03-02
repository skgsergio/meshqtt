package main

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"

	pb "github.com/skgsergio/meshqtt/internal/protobufs"
)

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

