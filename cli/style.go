// Copyright (c) 2026 Jules MAHOUNOU
// Project  : TRISH
// Initiated: 17/04/2026
// Origin   : Benin
// Contact  : jtodjinou@datatechnologies.bj | +229 0159521211
// License  : MIT — see LICENSE file for details

package cli

import (
	"fmt"
	"os"
	"strings"
)

const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiDim    = "\x1b[2m"
	ansiRed    = "\x1b[31m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiBlue   = "\x1b[34m"
	ansiCyan   = "\x1b[36m"
)

func color(code string, value string) string {
	if noColor() || value == "" {
		return value
	}
	return code + value + ansiReset
}

func bold(value string) string   { return color(ansiBold, value) }
func dim(value string) string    { return color(ansiDim, value) }
func red(value string) string    { return color(ansiRed, value) }
func green(value string) string  { return color(ansiGreen, value) }
func yellow(value string) string { return color(ansiYellow, value) }
func blue(value string) string   { return color(ansiBlue, value) }
func cyan(value string) string   { return color(ansiCyan, value) }

func noColor() bool {
	return strings.TrimSpace(os.Getenv("NO_COLOR")) != ""
}

func printBanner() {
	fmt.Println(cyan(`/####### /######  /####  /######/## /##   /##`))
	fmt.Println(cyan(`|__  ##__//##__  ##/##  ##|##_  ##_/ | ##  | ##`))
	fmt.Println(cyan(`   | ##  | ##  \__| ##  | ## \ ##    | ##  | ##`))
	fmt.Println(cyan(`   | ##  | ##   /######/  ##  \ ##   | ########`))
	fmt.Println(cyan(`   | ##  | ##  |__  ##_/  ##  | ##   |_____  ##`))
	fmt.Println(cyan(`   | ##  | ##  \  | ##    ##  | ##         | ##`))
	fmt.Println(cyan(`   | ##  |  ######/| ##   /######/         | ##`))
	fmt.Println(cyan(`   |__/   \______/ |__/  |______/          |__/`))
	fmt.Println(dim("Remote administration shell"))
	fmt.Println()
}

func printTable(headers []string, rows [][]string) {
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = len(stripANSI(header))
	}
	for _, row := range rows {
		for i := range headers {
			value := ""
			if i < len(row) {
				value = row[i]
			}
			if width := len(stripANSI(value)); width > widths[i] {
				widths[i] = width
			}
		}
	}

	printTableRow(headers, widths, true)
	sep := make([]string, len(headers))
	for i, width := range widths {
		sep[i] = strings.Repeat("-", width)
	}
	printTableRow(sep, widths, false)
	for _, row := range rows {
		printTableRow(row, widths, false)
	}
}

func printTableRow(values []string, widths []int, header bool) {
	for i, width := range widths {
		value := ""
		if i < len(values) {
			value = values[i]
		}
		if header {
			value = bold(value)
		}
		if i > 0 {
			fmt.Print("  ")
		}
		fmt.Print(value)
		padding := width - visibleLen(value)
		if padding > 0 {
			fmt.Print(strings.Repeat(" ", padding))
		}
	}
	fmt.Println()
}

func stripANSI(value string) string {
	var b strings.Builder
	inEscape := false
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if inEscape {
			if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') {
				inEscape = false
			}
			continue
		}
		if ch == 0x1b {
			inEscape = true
			continue
		}
		b.WriteByte(ch)
	}
	return b.String()
}

func visibleLen(value string) int {
	return len(stripANSI(value))
}

func statusLabel(status string, connected bool) string {
	normalized := defaultStatus(status)
	switch strings.ToLower(normalized) {
	case "online":
		if connected {
			return green(normalized)
		}
		return yellow(normalized)
	case "offline":
		return red(normalized)
	case "frozen", "restarting", "stopping":
		return yellow(normalized)
	default:
		return dim(normalized)
	}
}

func yesNo(value bool) string {
	if value {
		return green("yes")
	}
	return red("no")
}

func printCommandHint() {
	fmt.Println(bold("Commands"))
	fmt.Println("  " + cyan("list") + " | " + cyan("use <agent>") + " | " + cyan("current") + " | " + cyan("info [agent]") + " | " + cyan("ping [agent]"))
	fmt.Println("  " + cyan("exec [agent] <cmd> [args]") + " | " + cyan("agent <freeze|unfreeze|stop|restart> [agent]"))
	fmt.Println("  " + cyan("exec --all <cmd>") + " | " + cyan("exec --group <name> <cmd>") + " | " + cyan("group <...>") + " | " + cyan("tag <...>"))
	fmt.Println("  " + cyan("plugin <install|update|test|list|enable|disable|versions|rollback|status|remove>"))
	fmt.Println("  " + cyan("start gui") + " | " + cyan("help") + " | " + cyan("exit"))
}
