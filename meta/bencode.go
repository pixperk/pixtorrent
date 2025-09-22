package meta

import (
	"fmt"
	"io"
)

type Node any

type BInt int
type BString string
type BList []Node
type BDict map[string]Node

type Decode struct {
	r io.Reader
}

func NewDecoder(r io.Reader) *Decode {
	return &Decode{
		r: r,
	}
}

/* integers → i<number>e

example: i42e → 42

strings → <len>:<string>

example: 4:spam → "spam"

lists → l<items>e

example: l4:spami42ee → ["spam", 42]

dictionaries (map with string keys) → d<key><value>e

example: d3:bar4:spam3:fooi42ee → { "bar": "spam", "foo": 42 } */

func (d *Decode) Decode() (Node, error) {
	//peek the first byte
	var b [1]byte
	if _, err := d.r.Read(b[:]); err != nil {
		return nil, err
	}
	switch b[0] {
	case 'i':
		return d.parseInt()
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return d.parseString(b[0])
	case 'l':
		return d.parseList()
	case 'd':
		return d.parseDict()
	default:
		return nil, fmt.Errorf("unsupported bencode type: %c", b[0])
	}

}

func (d *Decode) parseInt() (BInt, error) {
	var numStr string
	var char [1]byte
	for {
		if _, err := d.r.Read(char[:]); err != nil {
			return 0, err
		}
		if char[0] == 'e' {
			break
		}
		numStr += string(char[0])
	}
	var i int
	if _, err := fmt.Sscanf(numStr, "%d", &i); err != nil {
		return 0, err
	}
	return BInt(i), nil
}

func (d *Decode) parseString(firstDigit byte) (BString, error) {

	lengthStr := string(firstDigit)
	var char [1]byte

	for {
		if _, err := d.r.Read(char[:]); err != nil {
			return "", err
		}
		if char[0] == ':' {
			break
		}
		lengthStr += string(char[0])
	}

	var length int
	if _, err := fmt.Sscanf(lengthStr, "%d", &length); err != nil {
		return "", err
	}

	content := make([]byte, length)
	if _, err := io.ReadFull(d.r, content); err != nil {
		return "", err
	}

	return BString(content), nil
}

func (d *Decode) parseList() (BList, error) {
	var list BList

	for {

		var firstByte [1]byte
		if _, err := d.r.Read(firstByte[:]); err != nil {
			return nil, err
		}

		if firstByte[0] == 'e' {
			break
		}

		var val Node
		var err error

		switch firstByte[0] {
		case 'i':
			val, err = d.parseInt()
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			val, err = d.parseString(firstByte[0])
		case 'l':
			val, err = d.parseList()
		case 'd':
			val, err = d.parseDict()
		default:
			return nil, fmt.Errorf("unsupported bencode type in list: %c", firstByte[0])
		}

		if err != nil {
			return nil, err
		}

		list = append(list, val)
	}

	return list, nil
}

func (d *Decode) parseDict() (BDict, error) {
	dict := make(BDict)

	for {

		var firstByte [1]byte
		if _, err := d.r.Read(firstByte[:]); err != nil {
			return nil, err
		}

		if firstByte[0] == 'e' {
			break
		}

		var key string
		switch firstByte[0] {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			keyStr, err := d.parseString(firstByte[0])
			if err != nil {
				return nil, err
			}
			key = string(keyStr)
		default:
			return nil, fmt.Errorf("dictionary keys must be strings, got: %c", firstByte[0])
		}

		value, err := d.Decode()
		if err != nil {
			return nil, err
		}

		dict[key] = value
	}

	return dict, nil
}
