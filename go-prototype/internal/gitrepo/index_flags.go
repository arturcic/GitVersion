package gitrepo

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/go-git/go-git/v5/plumbing/format/index"
)

// readIndexAssumeValid reads the bit which go-git's decoded Entry omits. The
// decoded index supplies names and ordering, including v4 compressed paths.
func readIndexAssumeValid(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var decoded index.Index
	if err := index.NewDecoder(bytes.NewReader(data)).Decode(&decoded); err != nil {
		return nil, err
	}
	const fixedLength = 62
	remaining := data[12:]
	var previous string
	var result map[string]bool
	for _, entry := range decoded.Entries {
		if len(remaining) < fixedLength {
			return nil, io.ErrUnexpectedEOF
		}
		flags := binary.BigEndian.Uint16(remaining[60:62])
		headerLength := fixedLength
		if flags&0x4000 != 0 {
			headerLength += 2
		}
		entryLength := 0
		if decoded.Version == 4 {
			if len(remaining) <= headerLength {
				return nil, io.ErrUnexpectedEOF
			}
			pathLength, err := indexV4SuffixLength(remaining[headerLength:], previous, entry.Name)
			if err != nil {
				return nil, err
			}
			entryLength = headerLength + pathLength
		} else {
			entryLength = (headerLength + len(entry.Name) + 8) &^ 7
		}
		if len(remaining) < entryLength {
			return nil, io.ErrUnexpectedEOF
		}
		if flags&0x8000 != 0 && entry.Stage == 0 {
			if result == nil {
				result = map[string]bool{}
			}
			result[entry.Name] = true
		}
		remaining, previous = remaining[entryLength:], entry.Name
	}
	return result, nil
}

func indexV4SuffixLength(data []byte, previous, current string) (int, error) {
	strip := 0
	for i, octet := range data {
		strip = (strip << 7) | int(octet&0x7f)
		if strip > len(previous) {
			return 0, fmt.Errorf("invalid index v4 path prefix")
		}
		if octet&0x80 != 0 {
			strip++
			continue
		}
		prefix := len(previous) - strip
		if prefix > len(current) || previous[:prefix] != current[:prefix] {
			return 0, fmt.Errorf("invalid index v4 path prefix")
		}
		return i + 1 + len(current) - prefix + 1, nil
	}
	return 0, io.ErrUnexpectedEOF
}
