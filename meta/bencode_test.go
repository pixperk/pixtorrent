package meta

import (
	"fmt"
	"strings"
	"testing"
)

func TestBencodeDecoder(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Integer tests
		{"positive integer", "i42e", "42"},
		{"negative integer", "i-13e", "-13"},
		{"zero", "i0e", "0"},

		// String tests
		{"simple string", "4:spam", "spam"},
		{"empty string", "0:", ""},
		{"string with spaces", "11:hello world", "hello world"},

		// List tests
		{"empty list", "le", "[]"},
		{"single integer list", "li42ee", "[42]"},
		{"mixed list", "l4:spami42ee", "[spam 42]"},
		{"nested list", "ll4:spamei42ee", "[[spam] 42]"},

		// Dictionary tests
		{"empty dict", "de", "map[]"},
		{"simple dict", "d3:bar4:spam3:fooi42ee", "map[bar:spam foo:42]"},
		{"nested dict", "d4:listl4:spami42ee3:keyi99ee", "map[key:99 list:[spam 42]]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoder := NewDecoder(strings.NewReader(tt.input))
			result, err := decoder.Decode()

			if err != nil {
				t.Fatalf("Failed to decode %q: %v", tt.input, err)
			}

			fmt.Printf("Input: %-20s -> Output: %v\n", tt.input, result)

			resultStr := fmt.Sprintf("%v", result)
			if resultStr != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, resultStr)
			}
		})
	}
}

func TestBencodeTypes(t *testing.T) {
	fmt.Println("\n=== Type Testing ===")

	//integer type
	decoder := NewDecoder(strings.NewReader("i42e"))
	result, err := decoder.Decode()
	if err != nil {
		t.Fatal(err)
	}
	if bint, ok := result.(BInt); ok {
		fmt.Printf("Integer: %d (type: %T)\n", bint, bint)
	}

	//string type
	decoder = NewDecoder(strings.NewReader("4:test"))
	result, err = decoder.Decode()
	if err != nil {
		t.Fatal(err)
	}
	if bstr, ok := result.(BString); ok {
		fmt.Printf("String: %s (type: %T)\n", bstr, bstr)
	}

	//list type
	decoder = NewDecoder(strings.NewReader("li1ei2ee"))
	result, err = decoder.Decode()
	if err != nil {
		t.Fatal(err)
	}
	if blist, ok := result.(BList); ok {
		fmt.Printf("List: %v (type: %T, length: %d)\n", blist, blist, len(blist))
	}

	//dict type
	decoder = NewDecoder(strings.NewReader("d1:ai1ee"))
	result, err = decoder.Decode()
	if err != nil {
		t.Fatal(err)
	}
	if bdict, ok := result.(BDict); ok {
		fmt.Printf("Dict: %v (type: %T, keys: %d)\n", bdict, bdict, len(bdict))
	}
}

func TestBencodeErrors(t *testing.T) {
	fmt.Println("\n=== Error Testing ===")

	errorTests := []struct {
		name  string
		input string
	}{
		{"invalid integer", "i42"},
		{"invalid string", "4spam"},
		{"invalid list", "li42e"},
		{"invalid dict key", "di42ee"},
		{"empty input", ""},
		{"unknown type", "x42e"},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			decoder := NewDecoder(strings.NewReader(tt.input))
			result, err := decoder.Decode()

			if err != nil {
				fmt.Printf("Expected error for %q: %v\n", tt.input, err)
			} else {
				t.Errorf("Expected error for %q, but got result: %v", tt.input, result)
			}
		})
	}
}
