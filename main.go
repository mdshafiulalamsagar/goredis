package main

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"goredis/database"
)

// ═══════════ ANSI Color Codes ═══════════
const (
	reset   = "\033[0m"
	bold    = "\033[1m"
	dim     = "\033[2m"
	red     = "\033[31m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	magenta = "\033[35m"
	cyan    = "\033[36m"
	white   = "\033[37m"
)

const version = "0.2.0"

const banner = `
   ██████╗  ██████╗ ██████╗ ███████╗██████╗ ██╗███████╗
  ██╔════╝ ██╔═══██╗██╔══██╗██╔════╝██╔══██╗██║██╔════╝
  ██║  ███╗██║   ██║██████╔╝█████╗  ██║  ██║██║███████╗
  ██║   ██║██║   ██║██╔══██╗██╔══╝  ██║  ██║██║╚════██║
  ╚██████╔╝╚██████╔╝██║  ██║███████╗██████╔╝██║███████║
   ╚═════╝  ╚═════╝ ╚═╝  ╚═╝╚══════╝╚═════╝ ╚═╝╚══════╝`

// ═══════════ MAIN ═══════════
func main() {
	db := database.NewDatabase() // ✅ shadowing fix: db, database not
	reader := bufio.NewReader(os.Stdin)

	// Ctrl+C graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println()
		fmt.Println(dim + "  bye, goredis out." + reset)
		os.Exit(0)
	}()

	printBanner()

	for {
		// fancy prompt
		fmt.Print(bold + cyan + "goredis" + reset + bold + magenta + " ❯" + reset + " ")

		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println()
			fmt.Println(dim + "  bye, goredis out." + reset)
			return
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		cmd := strings.ToUpper(parts[0])

		switch cmd {
		case "SET":
			cmdSet(db, parts)
		case "GET":
			cmdGet(db, parts)
		case "DEL":
			cmdDel(db, parts)
		case "EXISTS":
			cmdExists(db, parts)
		case "KEYS":
			cmdKeys(db)
		case "DBSIZE":
			cmdDBSize(db)
		case "FLUSHALL":
			cmdFlushAll(db)
		case "INFO":
			cmdInfo(db)
		case "HELP", "?":
			cmdHelp()
		case "CLEAR", "CLS":
			fmt.Print("\033[2J\033[H")
			printBanner()
		case "VERSION":
			fmt.Println("  " + yellow + "goredis" + reset + " v" + version + " " +
				dim + "—" + reset + " " + magenta + "by sagar" + reset)
		case "EXIT", "QUIT":
			fmt.Println(dim + "  bye, goredis out." + reset)
			return
		default:
			printError(fmt.Sprintf("unknown command '%s' — type HELP", parts[0]))
		}
	}
}

// ═══════════ UI Helpers ═══════════

func printBanner() {
	fmt.Println()
	fmt.Println(cyan + banner + reset)
	fmt.Println()
	fmt.Println("            " + bold + "goredis" + reset +
		" " + dim + "—" + reset + " " + magenta + "by sagar" + reset)
	fmt.Println("            " + dim + "in-memory key-value store · built from scratch" + reset)
	fmt.Println()
	fmt.Println(dim + "            type " + reset + yellow + "HELP" + reset +
		dim + " for commands · " + reset + yellow + "EXIT" + reset +
		dim + " to quit" + reset)
	fmt.Println()
}

func printOK(msg string) {
	fmt.Println("  " + green + bold + "✓ " + reset + green + msg + reset)
}

func printError(msg string) {
	fmt.Println("  " + red + bold + "✗ " + reset + red + msg + reset)
}

func printMuted(msg string) {
	fmt.Println("  " + dim + msg + reset)
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

// ═══════════ Commands ═══════════

func cmdSet(db *database.Database, parts []string) {
	if len(parts) != 3 {
		printError("usage: SET <key> <value>")
		return
	}
	db.Set(parts[1], parts[2])
	printOK("OK")
}

func cmdGet(db *database.Database, parts []string) {
	if len(parts) != 2 {
		printError("usage: GET <key>")
		return
	}
	val, ok := db.Get(parts[1])
	if ok {
		fmt.Println("  " + green + "\"" + val + "\"" + reset)
	} else {
		printMuted("(nil)")
	}
}

func cmdDel(db *database.Database, parts []string) {
	if len(parts) != 2 {
		printError("usage: DEL <key>")
		return
	}
	if db.Del(parts[1]) {
		printOK("(integer) 1")
	} else {
		printMuted("(integer) 0")
	}
}

func cmdExists(db *database.Database, parts []string) {
	if len(parts) != 2 {
		printError("usage: EXISTS <key>")
		return
	}
	if db.Exists(parts[1]) {
		printOK("(integer) 1")
	} else {
		printMuted("(integer) 0")
	}
}

func cmdDBSize(db *database.Database) {
	fmt.Println("  " + yellow + fmt.Sprintf("%d", db.Len()) + reset)
}

func cmdFlushAll(db *database.Database) {
	n := db.FlushAll()
	printOK(fmt.Sprintf("OK — flushed %d keys", n))
}

// ─── KEYS: fancy Unicode table ───
func cmdKeys(db *database.Database) {
	snap := db.Snapshot()
	if len(snap) == 0 {
		printMuted("(empty database)")
		return
	}

	keys := make([]string, 0, len(snap))
	maxKey, maxVal := 5, 5 // minimum = header length
	for k, v := range snap {
		keys = append(keys, k)
		if len(k) > maxKey {
			maxKey = len(k)
		}
		if len(v) > maxVal {
			maxVal = len(v)
		}
	}
	sort.Strings(keys)

	numW := len(fmt.Sprintf("%d", len(keys)))

	top := fmt.Sprintf("  %s┌%s┬%s┬%s┐%s",
		cyan, strings.Repeat("─", numW+2), strings.Repeat("─", maxKey+2),
		strings.Repeat("─", maxVal+2), reset)
	mid := fmt.Sprintf("  %s├%s┼%s┼%s┤%s",
		cyan, strings.Repeat("─", numW+2), strings.Repeat("─", maxKey+2),
		strings.Repeat("─", maxVal+2), reset)
	bot := fmt.Sprintf("  %s└%s┴%s┴%s┘%s",
		cyan, strings.Repeat("─", numW+2), strings.Repeat("─", maxKey+2),
		strings.Repeat("─", maxVal+2), reset)

	fmt.Println(top)
	fmt.Printf("  %s│%s %s%-*s%s %s│%s %s%-*s%s %s│%s %s%-*s%s %s│%s\n",
		cyan, reset, yellow, numW, "#", reset,
		cyan, reset, yellow, maxKey, "KEY", reset,
		cyan, reset, yellow, maxVal, "VALUE", reset,
		cyan, reset)
	fmt.Println(mid)
	for i, k := range keys {
		fmt.Printf("  %s│%s %s%-*d%s %s│%s %s%-*s%s %s│%s %s%-*s%s %s│%s\n",
			cyan, reset, dim, numW, i+1, reset,
			cyan, reset, white, maxKey, k, reset,
			cyan, reset, white, maxVal, snap[k], reset,
			cyan, reset)
	}
	fmt.Println(bot)
}

// ─── INFO: stats panel ───
func cmdInfo(db *database.Database) {
	hits, misses, commands, uptime := db.Stats()
	keys := db.Len()
	hitRate := 0.0
	if hits+misses > 0 {
		hitRate = float64(hits) / float64(hits+misses) * 100
	}

	row := func(label, val string) {
		fmt.Printf("  %s│%s %s%-16s%s %s\n", cyan, reset, dim, label, reset, val)
	}

	fmt.Println()
	fmt.Println("  " + bold + cyan + "── GOREDIS INFO ──" + reset)
	row("Version", version)
	row("Author", magenta+"sagar"+reset)
	row("Uptime", formatDuration(uptime))
	row("Total Keys", green+fmt.Sprintf("%d", keys)+reset)
	row("Total Commands", yellow+fmt.Sprintf("%d", commands)+reset)
	row("Get Hits", green+fmt.Sprintf("%d", hits)+reset)
	row("Get Misses", red+fmt.Sprintf("%d", misses)+reset)
	row("Hit Rate", magenta+fmt.Sprintf("%.1f%%", hitRate)+reset)
	fmt.Println()
}

// ─── HELP: formatted list ───
func cmdHelp() {
	fmt.Println()
	fmt.Println("  " + bold + cyan + "── COMMANDS ──" + reset)
	fmt.Println()

	type entry struct{ name, args, desc string }
	cmds := []entry{
		{"SET", "<key> <value>", "store a key-value pair"},
		{"GET", "<key>", "retrieve value for key"},
		{"DEL", "<key>", "delete a key"},
		{"EXISTS", "<key>", "check if key exists"},
		{"KEYS", "", "list all keys (table)"},
		{"DBSIZE", "", "count of stored keys"},
		{"FLUSHALL", "", "remove all keys"},
		{"INFO", "", "server statistics"},
		{"CLEAR", "", "clear terminal screen"},
		{"VERSION", "", "show goredis version"},
		{"HELP", "", "show this help"},
		{"EXIT", "", "quit goredis"},
	}

	for _, c := range cmds {
		fmt.Printf("  %s%-10s%s %s%-22s%s %s%s%s\n",
			yellow, c.name, reset,
			dim, c.args, reset,
			white, c.desc, reset)
	}
	fmt.Println()
}
