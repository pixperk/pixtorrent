package meta

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func calculateBencodeStringLength(s string) string {
	return fmt.Sprintf("%d:%s", len(s), s)
}

func calculateBencodeIntLength(i int64) string {
	return fmt.Sprintf("i%de", i)
}

func generateSingleFileTorrentData(announce, name string, length, pieceLength int64, pieces []byte) []byte {
	announceStr := calculateBencodeStringLength(announce)
	nameStr := calculateBencodeStringLength(name)
	lengthStr := calculateBencodeIntLength(length)
	pieceLengthStr := calculateBencodeIntLength(pieceLength)
	piecesStr := calculateBencodeStringLength(string(pieces))

	infoDict := fmt.Sprintf("d%s%s%s%s%s%s%s%se",
		calculateBencodeStringLength("length"), lengthStr,
		calculateBencodeStringLength("name"), nameStr,
		calculateBencodeStringLength("piece length"), pieceLengthStr,
		calculateBencodeStringLength("pieces"), piecesStr)

	torrent := fmt.Sprintf("d%s%s%s%se",
		calculateBencodeStringLength("announce"), announceStr,
		calculateBencodeStringLength("info"), infoDict)

	return []byte(torrent)
}

func generateMultiFileTorrentData(announce, name string, pieceLength int64, pieces []byte, files []FileInfo) []byte {
	announceStr := calculateBencodeStringLength(announce)
	nameStr := calculateBencodeStringLength(name)
	pieceLengthStr := calculateBencodeIntLength(pieceLength)
	piecesStr := calculateBencodeStringLength(string(pieces))

	var filesStr strings.Builder
	filesStr.WriteString("l")
	for _, file := range files {
		var pathStr strings.Builder
		pathStr.WriteString("l")
		for _, component := range file.Path {
			pathStr.WriteString(calculateBencodeStringLength(component))
		}
		pathStr.WriteString("e")

		fileDict := fmt.Sprintf("d%s%s%s%se",
			calculateBencodeStringLength("length"), calculateBencodeIntLength(file.Length),
			calculateBencodeStringLength("path"), pathStr.String())
		filesStr.WriteString(fileDict)
	}
	filesStr.WriteString("e")

	infoDict := fmt.Sprintf("d%s%s%s%s%s%s%s%se",
		calculateBencodeStringLength("files"), filesStr.String(),
		calculateBencodeStringLength("name"), nameStr,
		calculateBencodeStringLength("piece length"), pieceLengthStr,
		calculateBencodeStringLength("pieces"), piecesStr)

	torrent := fmt.Sprintf("d%s%s%s%se",
		calculateBencodeStringLength("announce"), announceStr,
		calculateBencodeStringLength("info"), infoDict)

	return []byte(torrent)
}

func TestParseTorrent_SingleFile(t *testing.T) {
	data := generateSingleFileTorrentData(
		"http://tracker.example.com/announce",
		"test.txt",
		1024,
		32768,
		[]byte("01234567890123456789"), // 20 bytes SHA1
	)

	torrent, err := ParseTorrent(data)
	if err != nil {
		t.Fatalf("ParseTorrent failed: %v", err)
	}

	if torrent.Announce != "http://tracker.example.com/announce" {
		t.Errorf("Expected announce 'http://tracker.example.com/announce', got %q", torrent.Announce)
	}
	if torrent.Info.Name != "test.txt" {
		t.Errorf("Expected name 'test.txt', got %q", torrent.Info.Name)
	}
	if torrent.Info.Length != 1024 {
		t.Errorf("Expected length 1024, got %d", torrent.Info.Length)
	}
	if torrent.Info.PieceLength != 32768 {
		t.Errorf("Expected piece length 32768, got %d", torrent.Info.PieceLength)
	}
	if len(torrent.Info.Files) != 0 {
		t.Errorf("Expected single-file torrent to have empty Files slice, got %d files", len(torrent.Info.Files))
	}
}

func TestParseTorrent_MultiFile(t *testing.T) {
	files := []FileInfo{
		{Length: 1024, Path: []string{"file1.txt"}},
		{Length: 2048, Path: []string{"dir", "file2.txt"}},
	}

	data := generateMultiFileTorrentData(
		"http://tracker.example.com/announce",
		"testdir",
		32768,
		[]byte("01234567890123456789"),
		files,
	)

	torrent, err := ParseTorrent(data)
	if err != nil {
		t.Fatalf("ParseTorrent failed: %v", err)
	}

	if torrent.Info.Length != 0 {
		t.Errorf("Expected multi-file torrent to have Length=0, got %d", torrent.Info.Length)
	}
	if len(torrent.Info.Files) != 2 {
		t.Fatalf("Expected 2 files, got %d", len(torrent.Info.Files))
	}
	if torrent.Info.Files[0].Length != 1024 {
		t.Errorf("File 0: expected length 1024, got %d", torrent.Info.Files[0].Length)
	}
	if len(torrent.Info.Files[1].Path) != 2 || torrent.Info.Files[1].Path[0] != "dir" {
		t.Errorf("File 1: expected path [dir file2.txt], got %v", torrent.Info.Files[1].Path)
	}
}

func TestParseTorrentFile(t *testing.T) {
	tempDir := t.TempDir()

	// Valid file test
	data := generateSingleFileTorrentData("http://test.com/announce", "test.txt", 1024, 32768, []byte("01234567890123456789"))
	filename := filepath.Join(tempDir, "valid.torrent")
	err := os.WriteFile(filename, data, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	torrent, err := ParseTorrentFile(filename)
	if err != nil {
		t.Fatalf("ParseTorrentFile failed: %v", err)
	}
	if torrent.Announce != "http://test.com/announce" {
		t.Errorf("Expected announce 'http://test.com/announce', got %q", torrent.Announce)
	}

	// File not found test
	_, err = ParseTorrentFile(filepath.Join(tempDir, "nonexistent.torrent"))
	if err == nil {
		t.Fatal("Expected error for non-existent file, got nil")
	}
	if !strings.Contains(err.Error(), "failed to read torrent file") {
		t.Errorf("Expected error about reading file, got: %v", err)
	}
}

func TestParseTorrent_ErrorCases(t *testing.T) {
	errorTests := []struct {
		name     string
		data     string
		expected string
	}{
		{"not dictionary", "i42e", "torrent file must be a dictionary"},
		{"empty data", "", "failed to decode bencode"},
		{"missing announce", "de", "missing required announce field"},
		{"announce not string", fmt.Sprintf("d%s%se", calculateBencodeStringLength("announce"), "i42e"), "announce field must be a string"},
		{"missing info", fmt.Sprintf("d%s%se", calculateBencodeStringLength("announce"), calculateBencodeStringLength("http://example.com")), "missing required info field"},
		{"info not dict", fmt.Sprintf("d%s%s%s%se", calculateBencodeStringLength("announce"), calculateBencodeStringLength("http://example.com"), calculateBencodeStringLength("info"), "i42e"), "info field must be a dictionary"},
	}

	for _, test := range errorTests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseTorrent([]byte(test.data))
			if err == nil {
				t.Fatal("Expected error, got nil")
			}
			if !strings.Contains(err.Error(), test.expected) {
				t.Errorf("Expected error containing %q, got: %v", test.expected, err)
			}
		})
	}
}

func TestParseTorrent_InfoDictErrors(t *testing.T) {
	infoErrorTests := []struct {
		name     string
		infoDict string
		expected string
	}{
		{"missing name", "de", "missing required name field"},
		{"missing piece length", fmt.Sprintf("d%s%se", calculateBencodeStringLength("name"), calculateBencodeStringLength("test")), "missing required piece length field"},
		{"missing pieces", fmt.Sprintf("d%s%s%s%se", calculateBencodeStringLength("name"), calculateBencodeStringLength("test"), calculateBencodeStringLength("piece length"), "i32768e"), "missing required pieces field"},
		{"missing length and files", fmt.Sprintf("d%s%s%s%s%s%se", calculateBencodeStringLength("name"), calculateBencodeStringLength("test"), calculateBencodeStringLength("piece length"), "i32768e", calculateBencodeStringLength("pieces"), calculateBencodeStringLength("12345678901234567890")), "info dict must have either length"},
	}

	for _, test := range infoErrorTests {
		t.Run(test.name, func(t *testing.T) {
			torrentData := fmt.Sprintf("d%s%s%s%se",
				calculateBencodeStringLength("announce"), calculateBencodeStringLength("http://example.com"),
				calculateBencodeStringLength("info"), test.infoDict)

			_, err := ParseTorrent([]byte(torrentData))
			if err == nil {
				t.Fatal("Expected error, got nil")
			}
			if !strings.Contains(err.Error(), test.expected) {
				t.Errorf("Expected error containing %q, got: %v", test.expected, err)
			}
		})
	}
}
