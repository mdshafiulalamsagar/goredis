package main

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"goredis/database"
)

// ANSI Color Codes for terminal UI formatting.
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

const version = "0.3.0"

const banner = `
   ██████╗  ██████╗ ██████╗ ███████╗██████╗ ██╗███████╗
  ██╔════╝ ██╔═══██╗██╔══██╗██╔════╝██╔══██╗██║██╔════╝
  ██║  ███╗██║   ██║██████╔╝█████╗  ██║  ██║██║███████╗
  ██║   ██║██║   ██║██╔══██╗██╔══╝  ██║  ██║██║╚════██║
  ╚██████╔╝╚██████╔╝██║  ██║███████╗██████╔╝██║███████║
   ╚═════╝  ╚═════╝ ╚═╝  ╚═╝╚══════╝╚═════╝ ╚═╝╚══════╝`

func main() {
	db := database.NewDatabase()
	reader := bufio.NewReader(os.Stdin)

	// Set up OS signal handling to gracefully shutdown the application (e.g., on Ctrl+C).
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println()
		fmt.Println(dim + "  bye, goredis out." + reset)
		os.Exit(0)
	}()

	printBanner()

	// Main Read-Eval-Print Loop (REPL).
	for {
		fmt.Print(bold + cyan + "goredis" + reset + bold + magenta + " > " + reset + " ")

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

		// Split the input by spaces to extract the command and arguments.
		parts := strings.Fields(line)
		cmd := strings.ToUpper(parts[0])

		// Command routing block.
		switch cmd {
		case "SET":
			cmdSet(db, parts)
		case "GET":
			cmdGet(db, parts)
		case "DEL":
			cmdDel(db, parts)
		case "EXISTS":
			cmdExists(db, parts)
		case "TTL":
			cmdTTL(db, parts)
		case "EXPIRE":
			cmdExpire(db, parts)
		case "PERSIST":
			cmdPersist(db, parts)
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
			fmt.Print("\033[2J\033[H") // ANSI code to clear terminal screen.
			printBanner()
		case "VERSION":
			fmt.Println("  " + yellow + "goredis" + reset + " v" + version + " " +
				dim + "-" + reset + " " + magenta + "by sagar" + reset)
		case "EXIT", "QUIT":
			fmt.Println(dim + "  bye, goredis out." + reset)
			return
		default:
			printError(fmt.Sprintf("unknown command '%s' - type HELP", parts[0]))
		}
	}
}

// printBanner displays the application logo and basic instructions.
func printBanner() {
	fmt.Println()
	fmt.Println(cyan + banner + reset)
	fmt.Println()
	fmt.Println("            " + bold + "goredis" + reset +
		" " + dim + "-" + reset + " " + magenta + "by sagar" + reset)
	fmt.Println("            " + dim + "in-memory key-value store - built from scratch" + reset)
	fmt.Println()
	fmt.Println(dim + "            type " + reset + yellow + "HELP" + reset +
		dim + " for commands - " + reset + yellow + "EXIT" + reset +
		dim + " to quit" + reset)
	fmt.Println()
}

// printOK prints a success message in green.
func printOK(msg string) {
	fmt.Println("  " + green + bold + "OK " + reset + green + msg + reset)
}

// printError prints an error message in red.
func printError(msg string) {
	fmt.Println("  " + red + bold + "ERR " + reset + red + msg + reset)
}

// printMuted prints a dimmed message.
func printMuted(msg string) {
	fmt.Println("  " + dim + msg + reset)
}

// formatDuration converts a time.Duration into a human-readable string.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

// cmdSet handles the SET command. Usage: SET <key> <value> [EX <seconds>]
func cmdSet(db *database.Database, parts []string) {
	if len(parts) != 3 && len(parts) != 5 {
		printError("usage: SET <key> <value> [EX <seconds>]")
		return
	}

	var ttl time.Duration = 0

	// Parse optional expiration time.
	if len(parts) == 5 {
		if strings.ToUpper(parts[3]) != "EX" {
			printError("syntax error: expected 'EX'")
			return
		}
		seconds, err := strconv.Atoi(parts[4])
		if err != nil || seconds <= 0 {
			printError("invalid expiration time")
			return
		}
		ttl = time.Duration(seconds) * time.Second
	}

	db.Set(parts[1], parts[2], ttl)
	printOK("OK")
}

// cmdGet handles the GET command.
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

// cmdDel handles the DEL command.
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

// cmdExists handles the EXISTS command.
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

// cmdTTL handles the TTL command to check remaining time on a key.
func cmdTTL(db *database.Database, parts []string) {
	if len(parts) != 2 {
		printError("usage: TTL <key>")
		return
	}
	ttl := db.TTL(parts[1])
	if ttl == -2 {
		fmt.Println("  " + yellow + "(integer) -2" + reset + dim + "  (key does not exist or expired)" + reset)
	} else if ttl == -1 {
		fmt.Println("  " + yellow + "(integer) -1" + reset + dim + "  (key has no expiration)" + reset)
	} else {
		fmt.Println("  " + yellow + fmt.Sprintf("(integer) %d", ttl) + reset + dim + " seconds remaining" + reset)
	}
}

// cmdExpire handles the EXPIRE command to set a timeout on an existing key.
func cmdExpire(db *database.Database, parts []string) {
	if len(parts) != 3 {
		printError("usage: EXPIRE <key> <seconds>")
		return
	}
	seconds, err := strconv.Atoi(parts[2])
	if err != nil || seconds <= 0 {
		printError("invalid expiration time")
		return
	}
	if db.Expire(parts[1], time.Duration(seconds)*time.Second) {
		printOK("(integer) 1")
	} else {
		printMuted("(integer) 0")
	}
}

// cmdPersist handles the PERSIST command to remove an expiration from a key.
func cmdPersist(db *database.Database, parts []string) {
	if len(parts) != 2 {
		printError("usage: PERSIST <key>")
		return
	}
	if db.Persist(parts[1]) {
		printOK("(integer) 1")
	} else {
		printMuted("(integer) 0")
	}
}

// cmdDBSize handles the DBSIZE command.
func cmdDBSize(db *database.Database) {
	fmt.Println("  " + yellow + fmt.Sprintf("%d", db.Len()) + reset)
}

// cmdFlushAll handles the FLUSHALL command.
func cmdFlushAll(db *database.Database) {
	n := db.FlushAll()
	printOK(fmt.Sprintf("flushed %d keys", n))
}

// cmdKeys displays all keys in a formatted table.
func cmdKeys(db *database.Database) {
	snap := db.Snapshot()
	if len(snap) == 0 {
		printMuted("(empty database)")
		return
	}

	keys := make([]string, 0, len(snap))
	maxKey, maxVal, maxTTL := 5, 5, 3
	
	for k, v := range snap {
		keys = append(keys, k)
		if len(k) > maxKey {
			maxKey = len(k)
		}
		if len(v.Value) > maxVal {
			maxVal = len(v.Value)
		}
		
		// Determine string length for TTL column.
		ttlStr := "permanent"
		if !v.ExpiresAt.IsZero() {
			ttlStr = fmt.Sprintf("%ds", int(time.Until(v.ExpiresAt).Seconds()))
		}
		if len(ttlStr) > maxTTL {
			maxTTL = len(ttlStr)
		}
	}
	sort.Strings(keys)

	numW := len(fmt.Sprintf("%d", len(keys)))

	// Construct table borders using Unicode box-drawing characters.
	top := fmt.Sprintf("  %s+%s+%s+%s+%s+%s",
		cyan, strings.Repeat("-", numW+2), strings.Repeat("-", maxKey+2),
		strings.Repeat("-", maxVal+2), strings.Repeat("-", maxTTL+2), reset)
	mid := fmt.Sprintf("  %s|%s|%s|%s|%s|%s",
		cyan, strings.Repeat("-", numW+2), strings.Repeat("-", maxKey+2),
		strings.Repeat("-", maxVal+2), strings.Repeat("-", maxTTL+2), reset)
	bot := fmt.Sprintf("  %s+%s+%s+%s+%s+%s",
		cyan, strings.Repeat("-", numW+2), strings.Repeat("-", maxKey+2),
		strings.Repeat("-", maxVal+2), strings.Repeat("-", maxTTL+2), reset)

	fmt.Println(top)
	fmt.Printf("  %s|%s %s%-*s%s %s|%s %s%-*s%s %s|%s %s%-*s%s %s|%s %s%-*s%s %s|%s\n",
		cyan, reset, yellow, numW, "#", reset,
		cyan, reset, yellow, maxKey, "KEY", reset,
		cyan, reset, yellow, maxVal, "VALUE", reset,
		cyan, reset, yellow, maxTTL, "TTL", reset,
		cyan, reset)
	fmt.Println(mid)

	for i, k := range keys {
		ttlStr := "permanent"
		ttlColor := dim
		if !snap[k].ExpiresAt.IsZero() {
			sec := int(time.Until(snap[k].ExpiresAt).Seconds())
			ttlStr = fmt.Sprintf("%ds", sec)
			ttlColor = red
		}
		
		fmt.Printf("  %s|%s %s%-*d%s %s|%s %s%-*s%s %s|%s %s%-*s%s %s|%s %s%-*s%s %s|%s\n",
			cyan, reset, dim, numW, i+1, reset,
			cyan, reset, white, maxKey, k, reset,
			cyan, reset, white, maxVal, snap[k].Value, reset,
			cyan, reset, ttlColor, maxTTL, ttlStr, reset,
			cyan, reset)
	}
	fmt.Println(bot)
}

// cmdInfo displays server statistics.
func cmdInfo(db *database.Database) {
	hits, misses, commands, uptime := db.Stats()
	keys := db.Len()
	hitRate := 0.0
	if hits+misses > 0 {
		hitRate = float64(hits) / float64(hits+misses) * 100
	}

	row := func(label, val string) {
		fmt.Printf("  %s|%s %s%-16s%s %s\n", cyan, reset, dim, label, reset, val)
	}

	fmt.Println()
	fmt.Println("  " + bold + cyan + "-- GOREDIS INFO --" + reset)
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

// cmdHelp displays the help menu.
func cmdHelp() {
	fmt.Println()
	fmt.Println("  " + bold + cyan + "-- COMMANDS --" + reset)
	fmt.Println()

	type entry struct{ name, args, desc string }
	cmds := []entry{
		{"SET", "<key> <value> [EX sec]", "store a key-value pair (optional expiry)"},
		{"GET", "<key>", "retrieve value for key"},
		{"DEL", "<key>", "delete a key"},
		{"EXISTS", "<key>", "check if key exists"},
		{"TTL", "<key>", "get remaining time to live"},
		{"EXPIRE", "<key> <seconds>", "set a timeout on a key"},
		{"PERSIST", "<key>", "remove the timeout from a key"},
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
		fmt.Printf("  %s%-10s%s %s%-26s%s %s%s%s\n",
			yellow, c.name, reset,
			dim, c.args, reset,
			white, c.desc, reset)
	}
	fmt.Println()
}