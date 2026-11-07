package gitrepo

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
)

const (
	sha1Size   = 20
	sha256Size = 32
)

var (
	errObjectIDLength = errors.New("invalid Git object ID length")
	errObjectIDPrefix = errors.New("Git object ID prefix is out of range")
)

// ObjectID is a comparable value. Parsing either hash width is independent of
// whether the repository reader supports the corresponding storage format.
type ObjectID struct {
	data   [sha256Size]byte
	length uint8
}

func parseObjectID(value string) (ObjectID, error) {
	return parseHexObjectID([]byte(value))
}

func parseHexObjectID(value []byte) (ObjectID, error) {
	if len(value) != 2*sha1Size && len(value) != 2*sha256Size {
		return ObjectID{}, fmt.Errorf("%w: %d hex characters", errObjectIDLength, len(value))
	}
	var id ObjectID
	n, err := hex.Decode(id.data[:], value)
	if err != nil {
		return ObjectID{}, err
	}
	id.length = uint8(n)
	return id, nil
}

func objectIDFromBytes(value []byte) (ObjectID, error) {
	if len(value) != sha1Size && len(value) != sha256Size {
		return ObjectID{}, fmt.Errorf("%w: %d bytes", errObjectIDLength, len(value))
	}
	var id ObjectID
	id.length = uint8(copy(id.data[:], value))
	return id, nil
}

func (id ObjectID) String() string {
	return hex.EncodeToString(id.data[:id.length])
}

func (id ObjectID) prefix(length int) (string, error) {
	if length < 0 || length > 2*int(id.length) {
		return "", errObjectIDPrefix
	}
	return hex.EncodeToString(id.data[:(length+1)/2])[:length], nil
}

func (id ObjectID) copyTo(dest []byte) error {
	if len(dest) < int(id.length) {
		return errObjectIDLength
	}
	copy(dest, id.data[:id.length])
	return nil
}

func (id ObjectID) firstTwoBytes() uint16 { return binary.BigEndian.Uint16(id.data[:]) }
