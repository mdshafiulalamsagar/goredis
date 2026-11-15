package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"goredis/database"
)

// Application configuration.
const (
	PORT    = "6379" // Default Redis port.
	VERSION = "0.4.0"
)

// ANSI Color Codes for server logs.
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

const banner = `
   ██████╗  ██████╗ ██████╗ ███████╗██████╗ ██╗███████╗
  ██╔════╝ ██╔═══██╗██╔══██╗██╔════╝██╔══██╗██║██╔════╝
  ██║  ███╗██║   ██║██████╔╝█████╗  ██║  ██║██║███████╗
  ██║   ██║██║   ██║██╔══██╗██╔══╝  ██║  ██║██║╚════██║
  ╚██████╔╝╚██████╔╝██║  ██║███████╗██████╔╝██║███████║
   ╚═════╝  ╚═════╝ ╚═╝  ╚═╝╚══════╝╚═════╝ ╚═╝╚══════╝`

func main() {
	// Initialize the shared in-memory database.
	db := database.NewDatabase()

	// Start listening for incoming TCP connections on the specified port.
	// The server will bind to all available network interfaces (0.0.0.0).
	listener, err := net.Listen("tcp", "0.0.0.0:"+PORT)
	if err != nil {
		fmt.Printf("Failed to start server on port %s: %v\n", PORT, err)
		os.Exit(1)
	}
	defer listener.Close()

	printBanner()

	// Handle OS signals (Ctrl+C) to gracefully shutdown the server.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n" + dim + "Shutting down goredis server..." + reset)
		listener.Close()
		os.Exit(0)
	}()

	// Infinite loop to accept new client connections.
	for {
		// Accept blocks until a new client connects.
		conn, err := listener.Accept()
		if err != nil {
			// If the listener is closed (e.g., during shutdown), break the loop.
			if strings.Contains(err.Error(), "use of closed network connection") {
				break
			}
			fmt.Printf("Failed to accept connection: %v\n", err)
			continue
		}

		// Handle each client connection in a separate Goroutine.
		// This allows the server to handle multiple clients simultaneously.
		go handleClient(conn, db)
	}
}

// handleClient reads commands from a connected client, processes them, and sends back responses.
func handleClient(conn net.Conn, db *database.Database) {
	defer conn.Close() // Ensure the connection is closed when the function exits.

	// Log the new connection to the server terminal.
	clientAddr := conn.RemoteAddr().String()
	fmt.Printf("%s[CONNECT]%s Client connected from %s\n", green, reset, clientAddr)

	// Create a buffered reader to read lines from the network connection.
	reader := bufio.NewReader(conn)

	for {
		// Read until the newline character.
		line, err := reader.ReadString('\n')
		if err != nil {
			// If the client disconnects (EOF), log it and exit the loop.
			fmt.Printf("%s[DISCONNECT]%s Client %s disconnected\n", red, reset, clientAddr)
			return
		}

		// Clean up the input string (remove newline and extra spaces).
		line = strings.TrimSpace(line)
		if line == "" {
			continue // Ignore empty lines.
		}

		// Split the input into command and arguments.
		parts := strings.Fields(line)
		command := strings.ToUpper(parts[0])

		// Process the command and get the response string.
		// Responses must end with "\r\n" to follow standard network text protocols.
		response := processCommand(db, command, parts)

		// Send the response back to the client over the TCP connection.
		_, err = conn.Write([]byte(response + "\r\n"))
		if err != nil {
			fmt.Printf("Failed to send response to %s: %v\n", clientAddr, err)
			return
		}
	}
}

// processCommand acts as the command router. It calls the appropriate handler
// function based on the parsed command and returns a formatted string response.
func processCommand(db *database.Database, cmd string, parts []string) string {
	switch cmd {
	case "PING":
		return "PONG"
	case "SET":
		return cmdSet(db, parts)
	case "GET":
		return cmdGet(db, parts)
	case "DEL":
		return cmdDel(db, parts)
	case "EXISTS":
		return cmdExists(db, parts)
	case "TTL":
		return cmdTTL(db, parts)
	case "EXPIRE":
		return cmdExpire(db, parts)
	case "PERSIST":
		return cmdPersist(db, parts)
	case "KEYS":
		return cmdKeys(db)
	case "DBSIZE":
		return cmdDBSize(db)
	case "FLUSHALL":
		return cmdFlushAll(db)
	case "INFO":
		return cmdInfo(db)
	case "VERSION":
		return "goredis v" + VERSION + " - by sagar"
	case "HELP", "?":
		return cmdHelp()
	case "EXIT", "QUIT":
		return "BYE"
	default:
		return "ERR unknown command '" + parts[0] + "'"
	}
}

// -------------------------------------------------------------------------
// Command Handlers
// These functions execute DB operations and return standard string responses.
// -------------------------------------------------------------------------

func cmdSet(db *database.Database, parts []string) string {
	if len(parts) != 3 && len(parts) != 5 {
		return "ERR usage: SET <key> <value> [EX <seconds>]"
	}

	var ttl time.Duration = 0

	// Parse optional expiration time.
	if len(parts) == 5 {
		if strings.ToUpper(parts[3]) != "EX" {
			return "ERR syntax error: expected 'EX'"
		}
		seconds, err := strconv.Atoi(parts[4])
		if err != nil || seconds <= 0 {
			return "ERR invalid expiration time"
		}
		ttl = time.Duration(seconds) * time.Second
	}

	db.Set(parts[1], parts[2], ttl)
	return "OK"
}

func cmdGet(db *database.Database, parts []string) string {
	if len(parts) != 2 {
		return "ERR usage: GET <key>"
	}
	val, ok := db.Get(parts[1])
	if ok {
		return val // Return the raw value.
	}
	return "(nil)"
}

func cmdDel(db *database.Database, parts []string) string {
	if len(parts) != 2 {
		return "ERR usage: DEL <key>"
	}
	if db.Del(parts[1]) {
		return "(integer) 1"
	}
	return "(integer) 0"
}

func cmdExists(db *database.Database, parts []string) string {
	if len(parts) != 2 {
		return "ERR usage: EXISTS <key>"
	}
	if db.Exists(parts[1]) {
		return "(integer) 1"
	}
	return "(integer) 0"
}

func cmdTTL(db *database.Database, parts []string) string {
	if len(parts) != 2 {
		return "ERR usage: TTL <key>"
	}
	ttl := db.TTL(parts[1])
	return fmt.Sprintf("(integer) %d", ttl)
}

func cmdExpire(db *database.Database, parts []string) string {
	if len(parts) != 3 {
		return "ERR usage: EXPIRE <key> <seconds>"
	}
	seconds, err := strconv.Atoi(parts[2])
	if err != nil || seconds <= 0 {
		return "ERR invalid expiration time"
	}
	if db.Expire(parts[1], time.Duration(seconds)*time.Second) {
		return "(integer) 1"
	}
	return "(integer) 0"
}

func cmdPersist(db *database.Database, parts []string) string {
	if len(parts) != 2 {
		return "ERR usage: PERSIST <key>"
	}
	if db.Persist(parts[1]) {
		return "(integer) 1"
	}
	return "(integer) 0"
}

func cmdDBSize(db *database.Database) string {
	return fmt.Sprintf("(integer) %d", db.Len())
}

func cmdFlushAll(db *database.Database) string {
	n := db.FlushAll()
	return fmt.Sprintf("OK - flushed %d keys", n)
}

// cmdKeys returns a formatted string representing a table of keys and values.
func cmdKeys(db *database.Database) string {
	snap := db.Snapshot()
	if len(snap) == 0 {
		return "(empty database)"
	}

	keys := make([]string, 0, len(snap))
	maxKey, maxVal, maxTTL := 5, 5, 3
	
	for k, v := range snap {
		keys = append(keys, k)
		if len(k) > maxKey { maxKey = len(k) }
		if len(v.Value) > maxVal { maxVal = len(v.Value) }
		
		ttlStr := "permanent"
		if !v.ExpiresAt.IsZero() {
			ttlStr = fmt.Sprintf("%ds", int(time.Until(v.ExpiresAt).Seconds()))
		}
		if len(ttlStr) > maxTTL { maxTTL = len(ttlStr) }
	}
	sort.Strings(keys)

	numW := len(fmt.Sprintf("%d", len(keys)))
	var sb strings.Builder

	// Construct table borders. Since this is raw text over network, we avoid color codes here.
	border := fmt.Sprintf("+%s+%s+%s+%s+",
		strings.Repeat("-", numW+2), strings.Repeat("-", maxKey+2),
		strings.Repeat("-", maxVal+2), strings.Repeat("-", maxTTL+2))

	sb.WriteString(border + "\r\n")
	sb.WriteString(fmt.Sprintf("| %-*s | %-*s | %-*s | %-*s |\r\n", 
		numW, "#", maxKey, "KEY", maxVal, "VALUE", maxTTL, "TTL"))
	sb.WriteString(border + "\r\n")

	for i, k := range keys {
		ttlStr := "permanent"
		if !snap[k].ExpiresAt.IsZero() {
			ttlStr = fmt.Sprintf("%ds", int(time.Until(snap[k].ExpiresAt).Seconds()))
		}
		sb.WriteString(fmt.Sprintf("| %-*d | %-*s | %-*s | %-*s |\r\n",
			numW, i+1, maxKey, k, maxVal, snap[k].Value, maxTTL, ttlStr))
	}
	sb.WriteString(border)
	
	return sb.String()
}

// cmdInfo returns the server statistics as formatted text.
func cmdInfo(db *database.Database) string {
	hits, misses, commands, uptime := db.Stats()
	keys := db.Len()
	hitRate := 0.0
	if hits+misses > 0 {
		hitRate = float64(hits) / float64(hits+misses) * 100
	}

	var sb strings.Builder
	sb.WriteString("-- GOREDIS INFO --\r\n")
	sb.WriteString(fmt.Sprintf("Version:        %s\r\n", VERSION))
	sb.WriteString(fmt.Sprintf("Author:         sagar\r\n"))
	sb.WriteString(fmt.Sprintf("Uptime:         %s\r\n", formatDuration(uptime)))
	sb.WriteString(fmt.Sprintf("Total Keys:     %d\r\n", keys))
	sb.WriteString(fmt.Sprintf("Total Commands: %d\r\n", commands))
	sb.WriteString(fmt.Sprintf("Get Hits:       %d\r\n", hits))
	sb.WriteString(fmt.Sprintf("Get Misses:     %d\r\n", misses))
	sb.WriteString(fmt.Sprintf("Hit Rate:       %.1f%%", hitRate))
	
	return sb.String()
}

func formatDuration(d time.Duration) string {
	if d < time.Minute { return fmt.Sprintf("%ds", int(d.Seconds())) }
	if d < time.Hour { return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60) }
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

func printBanner() {
	fmt.Println()
	fmt.Println(cyan + banner + reset)
	fmt.Println()
	fmt.Println("            " + bold + "goredis server" + reset +
		" " + dim + "-" + reset + " " + magenta + "by sagar" + reset)
	fmt.Println("            " + dim + "version " + VERSION + " - listening on port " + PORT + reset)
	fmt.Println()
	fmt.Println(dim + "            waiting for connections... (press Ctrl+C to stop)" + reset)
	fmt.Println()
}

// cmdHelp returns the list of available commands as a formatted string.
func cmdHelp() string {
	var sb strings.Builder
	sb.WriteString("-- COMMANDS --\r\n")
	
	cmds := []struct{ name, args, desc string }{
		{"SET", "<key> <value> [EX sec]", "store a key-value pair (optional expiry)"},
		{"GET", "<key>", "retrieve value for key"},
		{"DEL", "<key>", "delete a key"},
		{"EXISTS", "<key>", "check if key exists"},
		{"TTL", "<key>", "get remaining time to live"},
		{"EXPIRE", "<key> <seconds>", "set a timeout on a key"},
		{"PERSIST", "<key>", "remove the timeout from a key"},
		{"KEYS", "", "list all keys (table format)"},
		{"DBSIZE", "", "count of stored keys"},
		{"FLUSHALL", "", "remove all keys"},
		{"INFO", "", "server statistics"},
		{"PING", "", "test server connection"},
		{"HELP", "", "show this help"},
		{"EXIT", "", "disconnect from server"},
	}

	for _, c := range cmds {
		sb.WriteString(fmt.Sprintf("%-10s %-24s %s\r\n", c.name, c.args, c.desc))
	}
	return sb.String()
}