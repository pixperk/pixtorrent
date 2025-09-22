package meta

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"os"
)

type Torrent struct {
	Announce     string
	AnnounceList [][]string
	Info         InfoDict
	CreationDate int64
	CreatedBy    string
	Comment      string
	Encoding     string
}

type InfoDict struct {
	Name        string
	Length      int64      // single-file mode
	Files       []FileInfo // multi-file mode
	PieceLength int64
	Pieces      []byte
	Private     int64
}

type FileInfo struct {
	Length int64
	Path   []string
}

func ParseTorrentFile(filename string) (*Torrent, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read torrent file: %w", err)
	}
	return ParseTorrent(data)
}

func ParseTorrent(data []byte) (*Torrent, error) {
	decoder := NewDecoder(bytes.NewReader(data))
	node, err := decoder.Decode()
	if err != nil {
		return nil, fmt.Errorf("failed to decode bencode: %w", err)
	}

	dict, ok := node.(BDict)
	if !ok {
		return nil, fmt.Errorf("torrent file must be a dictionary, got %T", node)
	}

	torrent := &Torrent{}

	if announceNode, exists := dict["announce"]; exists {
		if announce, ok := announceNode.(BString); ok {
			torrent.Announce = string(announce)
		} else {
			return nil, fmt.Errorf("announce field must be a string, got %T", announceNode)
		}
	} else {
		return nil, fmt.Errorf("missing required announce field")
	}

	if infoNode, exists := dict["info"]; exists {
		if infoDict, ok := infoNode.(BDict); ok {
			info, err := parseInfoDict(infoDict)
			if err != nil {
				return nil, fmt.Errorf("failed to parse info dict: %w", err)
			}
			torrent.Info = *info
		} else {
			return nil, fmt.Errorf("info field must be a dictionary, got %T", infoNode)
		}
	} else {
		return nil, fmt.Errorf("missing required info field")
	}

	parseOptionalString(dict, "comment", &torrent.Comment)
	parseOptionalString(dict, "created by", &torrent.CreatedBy)
	parseOptionalString(dict, "encoding", &torrent.Encoding)
	parseOptionalInt(dict, "creation date", &torrent.CreationDate)

	if announceListNode, exists := dict["announce-list"]; exists {
		if announceList, ok := announceListNode.(BList); ok {
			torrent.AnnounceList = parseAnnounceList(announceList)
		}
	}

	return torrent, nil
}

func (t *Torrent) InfoHash() ([20]byte, error) {
	return calculateInfoHash(t.Info)
}

func calculateInfoHash(info InfoDict) ([20]byte, error) {

	encoded, err := encodeBencodeInfoDict(info)
	if err != nil {
		return [20]byte{}, fmt.Errorf("failed to encode info dict: %w", err)
	}

	// Calculate SHA1 hash
	hash := sha1.Sum(encoded)
	return hash, nil
}

func encodeBencodeInfoDict(info InfoDict) ([]byte, error) {
	var buf bytes.Buffer

	// Start dictionary
	buf.WriteString("d")

	if len(info.Files) > 0 {
		buf.WriteString(fmt.Sprintf("%d:files", len("files")))
		buf.WriteString("l")
		for _, file := range info.Files {
			buf.WriteString("d")
			// length
			buf.WriteString(fmt.Sprintf("%d:length", len("length")))
			buf.WriteString(fmt.Sprintf("i%de", file.Length))
			// path
			buf.WriteString(fmt.Sprintf("%d:path", len("path")))
			buf.WriteString("l")
			for _, component := range file.Path {
				buf.WriteString(fmt.Sprintf("%d:%s", len(component), component))
			}
			buf.WriteString("e") // end path list
			buf.WriteString("e") // end file dict
		}
		buf.WriteString("e") // end files list
	}

	//length field (for single-file torrents)
	if info.Length > 0 {
		buf.WriteString(fmt.Sprintf("%d:length", len("length")))
		buf.WriteString(fmt.Sprintf("i%de", info.Length))
	}

	//name field (required)
	buf.WriteString(fmt.Sprintf("%d:name", len("name")))
	buf.WriteString(fmt.Sprintf("%d:%s", len(info.Name), info.Name))

	//piece length field (required)
	buf.WriteString(fmt.Sprintf("%d:piece length", len("piece length")))
	buf.WriteString(fmt.Sprintf("i%de", info.PieceLength))

	//pieces field (required)
	buf.WriteString(fmt.Sprintf("%d:pieces", len("pieces")))
	buf.WriteString(fmt.Sprintf("%d:", len(info.Pieces)))
	buf.Write(info.Pieces)

	//private field (if present)
	if info.Private > 0 {
		buf.WriteString(fmt.Sprintf("%d:private", len("private")))
		buf.WriteString(fmt.Sprintf("i%de", info.Private))
	}

	//end dictionary
	buf.WriteString("e")

	return buf.Bytes(), nil
}

func parseInfoDict(dict BDict) (*InfoDict, error) {
	info := &InfoDict{}

	if nameNode, exists := dict["name"]; exists {
		if name, ok := nameNode.(BString); ok {
			info.Name = string(name)
		} else {
			return nil, fmt.Errorf("name field must be a string, got %T", nameNode)
		}
	} else {
		return nil, fmt.Errorf("missing required name field in info dict")
	}

	if pieceLengthNode, exists := dict["piece length"]; exists {
		if pieceLength, ok := pieceLengthNode.(BInt); ok {
			info.PieceLength = int64(pieceLength)
		} else {
			return nil, fmt.Errorf("piece length field must be an integer, got %T", pieceLengthNode)
		}
	} else {
		return nil, fmt.Errorf("missing required piece length field in info dict")
	}

	if piecesNode, exists := dict["pieces"]; exists {
		if pieces, ok := piecesNode.(BString); ok {
			info.Pieces = []byte(pieces)
		} else {
			return nil, fmt.Errorf("pieces field must be a string, got %T", piecesNode)
		}
	} else {
		return nil, fmt.Errorf("missing required pieces field in info dict")
	}

	if lengthNode, exists := dict["length"]; exists {
		// Single-file mode
		if length, ok := lengthNode.(BInt); ok {
			info.Length = int64(length)
		} else {
			return nil, fmt.Errorf("length field must be an integer, got %T", lengthNode)
		}
	} else if filesNode, exists := dict["files"]; exists {
		// Multi-file mode
		if filesList, ok := filesNode.(BList); ok {
			files, err := parseFileList(filesList)
			if err != nil {
				return nil, fmt.Errorf("failed to parse files list: %w", err)
			}
			info.Files = files
		} else {
			return nil, fmt.Errorf("files field must be a list, got %T", filesNode)
		}
	} else {
		return nil, fmt.Errorf("info dict must have either length (single-file) or files (multi-file) field")
	}

	parseOptionalInt(dict, "private", &info.Private)

	return info, nil
}

func parseFileList(filesList BList) ([]FileInfo, error) {
	files := make([]FileInfo, len(filesList))

	for i, fileNode := range filesList {
		fileDict, ok := fileNode.(BDict)
		if !ok {
			return nil, fmt.Errorf("file entry %d must be a dictionary, got %T", i, fileNode)
		}

		if lengthNode, exists := fileDict["length"]; exists {
			if length, ok := lengthNode.(BInt); ok {
				files[i].Length = int64(length)
			} else {
				return nil, fmt.Errorf("file %d length field must be an integer, got %T", i, lengthNode)
			}
		} else {
			return nil, fmt.Errorf("file %d missing required length field", i)
		}

		if pathNode, exists := fileDict["path"]; exists {
			if pathList, ok := pathNode.(BList); ok {
				path := make([]string, len(pathList))
				for j, pathComponent := range pathList {
					if pathStr, ok := pathComponent.(BString); ok {
						path[j] = string(pathStr)
					} else {
						return nil, fmt.Errorf("file %d path component %d must be a string, got %T", i, j, pathComponent)
					}
				}
				files[i].Path = path
			} else {
				return nil, fmt.Errorf("file %d path field must be a list, got %T", i, pathNode)
			}
		} else {
			return nil, fmt.Errorf("file %d missing required path field", i)
		}
	}

	return files, nil
}

func parseAnnounceList(announceList BList) [][]string {
	result := make([][]string, 0, len(announceList))

	for _, tierNode := range announceList {
		if tierList, ok := tierNode.(BList); ok {
			tier := make([]string, 0, len(tierList))
			for _, trackerNode := range tierList {
				if tracker, ok := trackerNode.(BString); ok {
					tier = append(tier, string(tracker))
				}
			}
			if len(tier) > 0 {
				result = append(result, tier)
			}
		}
	}

	return result
}

func parseOptionalString(dict BDict, key string, target *string) {
	if node, exists := dict[key]; exists {
		if str, ok := node.(BString); ok {
			*target = string(str)
		}
	}
}

func parseOptionalInt(dict BDict, key string, target *int64) {
	if node, exists := dict[key]; exists {
		if i, ok := node.(BInt); ok {
			*target = int64(i)
		}
	}
}
