package cmd

import (
	"fmt"
	"strings"
)

// ANSI color codes
const (
	Reset     = "\033[0m"
	Bold      = "\033[1m"
	Dim       = "\033[2m"

	Red       = "\033[31m"
	Green     = "\033[32m"
	Yellow    = "\033[33m"
	Blue      = "\033[34m"
	Magenta   = "\033[35m"
	Cyan      = "\033[36m"
	White     = "\033[37m"

	BgBlue    = "\033[44m"
	BgMagenta = "\033[45m"
	BgCyan    = "\033[46m"
)

var logo = `
        _      _                            _
  _ __ (_)_  _| |_ ___  _ __ _ __ ___ _ __ | |_
 | '_ \| \ \/ / __/ _ \| '__| '__/ _ \ '_ \| __|
 | |_) | |>  <| || (_) | |  | | |  __/ | | | |_
 | .__/|_/_/\_\\__\___/|_|  |_|  \___|_| |_|\__|
 |_|
`

var logoSmall = `
        _      _                            _
  _ __ (_)_  _| |_ ___  _ __ _ __ ___ _ __ | |_
 | '_ \| \ \/ / __/ _ \| '__| '__/ _ \ '_ \| __|
 | |_) | |>  <| || (_) | |  | | |  __/ | | | |_
 | .__/|_/_/\_\\__\___/|_|  |_|  \___|_| |_|\__|
 |_|
`

func PrintLogo() {
	fmt.Print(Cyan + Bold)
	fmt.Println(logo)
	fmt.Print(Reset)
}

func PrintLogoSmall() {
	fmt.Print(Cyan + Bold)
	fmt.Println(logoSmall)
	fmt.Print(Reset)
}

func PrintHeader(title string) {
	width := 50
	padding := (width - len(title) - 2) / 2

	fmt.Println()
	fmt.Print(Cyan)
	fmt.Println("  ╭" + strings.Repeat("─", width) + "╮")
	fmt.Printf("  │%s %s%s%s %s│\n",
		strings.Repeat(" ", padding),
		Bold + White, title, Reset + Cyan,
		strings.Repeat(" ", width-padding-len(title)-2))
	fmt.Println("  ╰" + strings.Repeat("─", width) + "╯")
	fmt.Print(Reset)
}

func PrintSection(title string) {
	fmt.Println()
	fmt.Printf("  %s%s▸ %s%s\n", Bold, Magenta, title, Reset)
	fmt.Printf("  %s%s%s\n", Dim, strings.Repeat("─", 48), Reset)
}

func PrintKeyValue(key, value string) {
	fmt.Printf("  %s%-12s%s %s%s%s\n", Dim, key, Reset, White, value, Reset)
}

func PrintKeyValueHighlight(key, value string) {
	fmt.Printf("  %s%-12s%s %s%s%s%s\n", Dim, key, Reset, Bold, Cyan, value, Reset)
}

func PrintSuccess(msg string) {
	fmt.Printf("\n  %s%s✓ %s%s\n", Bold, Green, msg, Reset)
}

func PrintError(msg string) {
	fmt.Printf("\n  %s%s✗ %s%s\n", Bold, Red, msg, Reset)
}

func PrintWarning(msg string) {
	fmt.Printf("\n  %s%s⚠ %s%s\n", Bold, Yellow, msg, Reset)
}

func PrintInfo(msg string) {
	fmt.Printf("  %s%s→ %s%s\n", Dim, Cyan, msg, Reset)
}

func PrintCommand(cmd string) {
	fmt.Println()
	fmt.Printf("  %s%s$ %s%s\n", Bold, Green, cmd, Reset)
}

func PrintBox(lines []string) {
	maxLen := 0
	for _, line := range lines {
		if len(line) > maxLen {
			maxLen = len(line)
		}
	}

	width := maxLen + 4

	fmt.Println()
	fmt.Print(Dim)
	fmt.Println("  ┌" + strings.Repeat("─", width) + "┐")
	for _, line := range lines {
		padding := width - len(line) - 2
		fmt.Printf("  │ %s%s%s%s │\n", Reset, line, Dim, strings.Repeat(" ", padding))
	}
	fmt.Println("  └" + strings.Repeat("─", width) + "┘")
	fmt.Print(Reset)
}

func PrintDivider() {
	fmt.Printf("\n  %s%s%s\n", Dim, strings.Repeat("─", 50), Reset)
}

func PrintStatus(label, status, color string) {
	fmt.Printf("  %s%-12s%s [%s%s%s%s]\n", Dim, label, Reset, Bold, color, status, Reset)
}

func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
