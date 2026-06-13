package main

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

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

	BrightRed     = "\033[91m"
	BrightGreen   = "\033[92m"
	BrightYellow  = "\033[93m"
	BrightBlue    = "\033[94m"
	BrightMagenta = "\033[95m"
	BrightCyan    = "\033[96m"

	BgRed    = "\033[41m"
	BgGreen  = "\033[42m"
	BgBlue   = "\033[44m"
	BgCyan   = "\033[46m"
)

func init() {
	handle := windows.Handle(os.Stdout.Fd())
	var mode uint32
	windows.GetConsoleMode(handle, &mode)
	windows.SetConsoleMode(handle, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
}

func timestamp() string {
	return time.Now().Format("15:04:05")
}

func setTitle(title string) {
	fmt.Printf("\033]0;%s\007", title)
}

func logInfo(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	fmt.Printf("  %s%s·%s %s\n", Dim, timestamp(), Reset, msg)
}

func logSuccess(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	fmt.Printf("  %s%s%s %s✓%s %s\n", Dim, timestamp(), Reset, BrightGreen+Bold, Reset, msg)
}

func logWarn(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	fmt.Printf("  %s%s%s %s!%s %s\n", Dim, timestamp(), Reset, BrightYellow+Bold, Reset, msg)
}

func logError(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	fmt.Printf("  %s%s%s %s✗%s %s\n", Dim, timestamp(), Reset, BrightRed+Bold, Reset, msg)
}

func logStep(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	fmt.Printf("  %s%s%s %s»%s %s%s%s\n", Dim, timestamp(), Reset, BrightMagenta+Bold, Reset, Bold, msg, Reset)
}

func printBanner() {
	banner := `
` + BrightMagenta + Bold + `   ██████╗ ██╗   ██╗███╗   ██╗███████╗` + Reset + `
` + BrightMagenta + Bold + `  ██╔════╝ ██║   ██║████╗  ██║██╔════╝` + Reset + `
` + BrightMagenta + Bold + `  ██║  ███╗██║   ██║██╔██╗ ██║███████╗` + Reset + `
` + BrightMagenta + Bold + `  ██║   ██║██║   ██║██║╚██╗██║╚════██║` + Reset + `
` + BrightMagenta + Bold + `  ╚██████╔╝╚██████╔╝██║ ╚████║███████║` + Reset + `
` + BrightMagenta + Bold + `   ╚═════╝  ╚═════╝ ╚═╝  ╚═══╝╚══════╝` + Reset + `
` + Dim + `  ─────────── github.com/glockinhand ───────────` + Reset + `
`
	fmt.Println(banner)
}

func printSeparator() {
	fmt.Printf("  %s────────────────────────────────────────%s\n", Dim, Reset)
}

var threadColors = []string{
	BrightCyan, BrightGreen, BrightYellow, BrightMagenta, BrightBlue,
	Cyan, Green, Yellow, Magenta, Blue,
}

func getThreadColor(threadID int) string {
	return threadColors[threadID%len(threadColors)]
}

func logThread(threadID int, format string, a ...any) {
	color := getThreadColor(threadID)
	msg := fmt.Sprintf(format, a...)
	fmt.Printf("  %s%s%s %s#%d%s %s\n", Dim, timestamp(), Reset, color, threadID+1, Reset, msg)
}

func logThreadSuccess(threadID int, format string, a ...any) {
	color := getThreadColor(threadID)
	msg := fmt.Sprintf(format, a...)
	fmt.Printf("  %s%s%s %s#%d%s %s✓ %s%s\n", Dim, timestamp(), Reset, color, threadID+1, Reset, BrightGreen+Bold, msg, Reset)
}

func logThreadError(threadID int, format string, a ...any) {
	color := getThreadColor(threadID)
	msg := fmt.Sprintf(format, a...)
	fmt.Printf("  %s%s%s %s#%d%s %s✗ %s%s\n", Dim, timestamp(), Reset, color, threadID+1, Reset, BrightRed, msg, Reset)
}
