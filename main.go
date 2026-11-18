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

const (
	PORT    = "6379"
	VERSION = "0.7.0" // Version bumped for Data Structures
)

const (
	reset   = "\033[0m"
	dim     = "\033[2m"
	red     = "\033[31m"
	green   = "\033[32m"
	magenta = "\033[35m"
	cyan    = "\033[36m"
)

func main() {
	db := database.NewDatabase()

	aof, err := database.NewAOFLogger("data.aof")
	if err != nil {
		fmt.Printf("Failed to open AOF file: %v\n", err)
		os.Exit(1)
	}
	defer aof.Close()

	loadAOF(db, "data.aof")

	listener, err := net.Listen("tcp", "0.0.0.0:"+PORT)
	if err != nil {
		fmt.Printf("Failed to start server on port %s: %v\n", PORT, err)
		os.Exit(1)
	}
	defer listener.Close()

	printBanner()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n" + dim + "Shutting down goredis server..." + reset)
		listener.Close()
		os.Exit(0)
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				break
			}
			continue
		}
		go handleClient(conn, db, aof)
	}
}

func handleClient(conn net.Conn, db *database.Database, aof *database.AOFLogger) {
	defer conn.Close()
	clientAddr := conn.RemoteAddr().String()
	fmt.Printf("%s[CONNECT]%s Client connected from %s\n", green, reset, clientAddr)

	reader := bufio.NewReader(conn)

	for {
		parts, err := readCommand(reader)
		if err != nil {
			return
		}
		if len(parts) == 0 { continue }

		command := strings.ToUpper(parts[0])
		response := processCommand(db, command, parts)

		if isMutatingCommand(command) {
			isErr := false
			if errStr, ok := response.(error); ok && errStr != nil { isErr = true }
			if !isErr {
				cmdToSave := strings.Join(parts, " ")
				aof.WriteCommand(cmdToSave)
			}
		}

		err = writeResponse(conn, response)
		if err != nil { return }
	}
}

func processCommand(db *database.Database, cmd string, parts []string) interface{} {
	switch cmd {
	case "PING": return SimpleString("PONG")
	case "SET": return cmdSet(db, parts)
	case "GET": return cmdGet(db, parts)
	case "DEL": return cmdDel(db, parts)
	case "EXISTS": return cmdExists(db, parts)
	case "TTL": return cmdTTL(db, parts)
	case "EXPIRE": return cmdExpire(db, parts)
	case "PERSIST": return cmdPersist(db, parts)
	case "LPUSH": return cmdLPush(db, parts)
	case "LRANGE": return cmdLRange(db, parts)
	case "HSET": return cmdHSet(db, parts)
	case "HGET": return cmdHGet(db, parts)
	case "HGETALL": return cmdHGetAll(db, parts)
	case "KEYS": return cmdKeys(db)
	case "DBSIZE": return db.Len()
	case "FLUSHALL": 
		n := db.FlushAll()
		return SimpleString(fmt.Sprintf("OK - flushed %d keys", n))
	case "INFO": return cmdInfo(db)
	case "VERSION": return SimpleString("goredis v" + VERSION + " - by sagar")
	case "HELP", "?": return cmdHelp()
	case "EXIT", "QUIT": return SimpleString("BYE")
	default: return fmt.Errorf("unknown command '%s'", parts[0])
	}
}

func isMutatingCommand(cmd string) bool {
	switch cmd {
	case "SET", "DEL", "EXPIRE", "PERSIST", "FLUSHALL", "LPUSH", "HSET":
		return true
	}
	return false
}

// --- Command Handlers ---

func cmdSet(db *database.Database, parts []string) interface{} {
	if len(parts) != 3 && len(parts) != 5 { return fmt.Errorf("usage: SET <key> <value> [EX <seconds>]") }
	var ttl time.Duration = 0
	if len(parts) == 5 {
		if strings.ToUpper(parts[3]) != "EX" { return fmt.Errorf("syntax error: expected 'EX'") }
		seconds, err := strconv.Atoi(parts[4])
		if err != nil || seconds <= 0 { return fmt.Errorf("invalid expiration time") }
		ttl = time.Duration(seconds) * time.Second
	}
	db.Set(parts[1], parts[2], ttl)
	return SimpleString("OK")
}

func cmdGet(db *database.Database, parts []string) interface{} {
	if len(parts) != 2 { return fmt.Errorf("usage: GET <key>") }
	val, ok := db.Get(parts[1])
	if ok { return val }
	return nil
}

func cmdLPush(db *database.Database, parts []string) interface{} {
	if len(parts) != 3 { return fmt.Errorf("usage: LPUSH <key> <value>") }
	length := db.LPush(parts[1], parts[2])
	if length == -1 { return fmt.Errorf("wrong type, key holds a non-list value") }
	return length
}

func cmdLRange(db *database.Database, parts []string) interface{} {
	if len(parts) != 4 { return fmt.Errorf("usage: LRANGE <key> <start> <stop>") }
	start, err1 := strconv.Atoi(parts[2])
	stop, err2 := strconv.Atoi(parts[3])
	if err1 != nil || err2 != nil { return fmt.Errorf("invalid range index") }
	
	list, ok := db.LRange(parts[1], start, stop)
	if !ok { return nil }
	return list // Returns []string which writeResponse encodes as Array
}

func cmdHSet(db *database.Database, parts []string) interface{} {
	if len(parts) != 4 { return fmt.Errorf("usage: HSET <key> <field> <value>") }
	isNew := db.HSet(parts[1], parts[2], parts[3])
	if isNew { return 1 }
	return 0
}

func cmdHGet(db *database.Database, parts []string) interface{} {
	if len(parts) != 3 { return fmt.Errorf("usage: HGET <key> <field>") }
	val, ok := db.HGet(parts[1], parts[2])
	if ok { return val }
	return nil
}

func cmdHGetAll(db *database.Database, parts []string) interface{} {
	if len(parts) != 2 { return fmt.Errorf("usage: HGETALL <key>") }
	hash, ok := db.HGetAll(parts[1])
	if !ok { return nil }
	return hash // Returns map[string]string which writeResponse encodes as Array
}

// --- Helper functions ---

func loadAOF(db *database.Database, filename string) {
	file, err := os.Open(filename)
	if err != nil { return }
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) == 0 { continue }

		cmd := strings.ToUpper(parts[0])
		switch cmd {
		case "SET":
			var ttl time.Duration = 0
			if len(parts) == 5 && strings.ToUpper(parts[3]) == "EX" {
				sec, err := strconv.Atoi(parts[4])
				if err == nil { ttl = time.Duration(sec) * time.Second }
			}
			db.Set(parts[1], parts[2], ttl)
		case "DEL":
			db.Del(parts[1])
		case "EXPIRE":
			sec, err := strconv.Atoi(parts[2])
			if err == nil { db.Expire(parts[1], time.Duration(sec)*time.Second) }
		case "PERSIST":
			db.Persist(parts[1])
		case "LPUSH":
			db.LPush(parts[1], parts[2])
		case "HSET":
			db.HSet(parts[1], parts[2], parts[3])
		case "FLUSHALL":
			db.FlushAll()
		}
	}
}

// (Other helper functions like cmdDel, cmdExists, cmdTTL, etc., are the same as before)
func cmdDel(db *database.Database, parts []string) interface{} {
	if len(parts) != 2 { return fmt.Errorf("usage: DEL <key>") }
	if db.Del(parts[1]) { return 1 }
	return 0
}
func cmdExists(db *database.Database, parts []string) interface{} {
	if len(parts) != 2 { return fmt.Errorf("usage: EXISTS <key>") }
	if db.Exists(parts[1]) { return 1 }
	return 0
}
func cmdTTL(db *database.Database, parts []string) interface{} {
	if len(parts) != 2 { return fmt.Errorf("usage: TTL <key>") }
	return db.TTL(parts[1])
}
func cmdExpire(db *database.Database, parts []string) interface{} {
	if len(parts) != 3 { return fmt.Errorf("usage: EXPIRE <key> <seconds>") }
	sec, err := strconv.Atoi(parts[2])
	if err != nil || sec <= 0 { return fmt.Errorf("invalid expiration time") }
	if db.Expire(parts[1], time.Duration(sec)*time.Second) { return 1 }
	return 0
}
func cmdPersist(db *database.Database, parts []string) interface{} {
	if len(parts) != 2 { return fmt.Errorf("usage: PERSIST <key>") }
	if db.Persist(parts[1]) { return 1 }
	return 0
}

func cmdKeys(db *database.Database) interface{} {
	snap := db.Snapshot()
	if len(snap) == 0 {
		return SimpleString("(empty database)")
	}

	keys := make([]string, 0, len(snap))
	maxKey, maxType, maxVal := 5, 4, 5
	
	for k, v := range snap {
		keys = append(keys, k)
		if len(k) > maxKey {
			maxKey = len(k)
		}
		if len(v.Type) > maxType {
			maxType = len(v.Type)
		}
		
		// Format value for preview using switch to avoid Go's else formatting issues
		var valStr string
		switch v.Type {
		case "string":
			valStr = v.Value.(string)
		case "list":
			valStr = fmt.Sprintf("[%d items]", len(v.Value.([]string)))
		case "hash":
			valStr = fmt.Sprintf("{%d fields}", len(v.Value.(map[string]string)))
		}
		
		if len(valStr) > maxVal {
			maxVal = len(valStr)
		}
	}
	sort.Strings(keys)

	numW := len(fmt.Sprintf("%d", len(keys)))
	var sb strings.Builder
	border := fmt.Sprintf("+%s+%s+%s+%s+\n",
		strings.Repeat("-", numW+2), strings.Repeat("-", maxKey+2),
		strings.Repeat("-", maxType+2), strings.Repeat("-", maxVal+2))

	sb.WriteString(border)
	sb.WriteString(fmt.Sprintf("| %-*s | %-*s | %-*s | %-*s |\n", numW, "#", maxKey, "KEY", maxType, "TYPE", maxVal, "VALUE"))
	sb.WriteString(border)

	for i, k := range keys {
		v := snap[k]
		var valStr string
		switch v.Type {
		case "string":
			valStr = v.Value.(string)
		case "list":
			valStr = fmt.Sprintf("[%d items]", len(v.Value.([]string)))
		case "hash":
			valStr = fmt.Sprintf("{%d fields}", len(v.Value.(map[string]string)))
		}
		
		sb.WriteString(fmt.Sprintf("| %-*d | %-*s | %-*s | %-*s |\n", numW, i+1, maxKey, k, maxType, v.Type, maxVal, valStr))
	}
	sb.WriteString(border)
	return sb.String()
}

func cmdInfo(db *database.Database) interface{} {
	hits, misses, commands, uptime := db.Stats()
	var sb strings.Builder
	sb.WriteString("-- GOREDIS INFO --\n")
	sb.WriteString(fmt.Sprintf("Version:        %s\n", VERSION))
	sb.WriteString(fmt.Sprintf("Uptime:         %s\n", formatDuration(uptime)))
	sb.WriteString(fmt.Sprintf("Total Keys:     %d\n", db.Len()))
	sb.WriteString(fmt.Sprintf("Total Commands: %d\n", commands))
	sb.WriteString(fmt.Sprintf("Get Hits:       %d\n", hits))
	sb.WriteString(fmt.Sprintf("Get Misses:     %d\n", misses))
	return sb.String()
}

func cmdHelp() interface{} {
	var sb strings.Builder
	sb.WriteString("-- COMMANDS --\n")
	cmds := []struct{ name, args, desc string }{
		{"SET", "<key> <value> [EX sec]", "store a string value"},
		{"GET", "<key>", "get a string value"},
		{"LPUSH", "<key> <value>", "prepend to a list"},
		{"LRANGE", "<key> <start> <stop>", "get list elements"},
		{"HSET", "<key> <field> <value>", "set a hash field"},
		{"HGET", "<key> <field>", "get a hash field"},
		{"HGETALL", "<key>", "get all hash fields"},
		{"DEL", "<key>", "delete a key"},
		{"EXISTS", "<key>", "check if key exists"},
		{"TTL", "<key>", "get remaining time to live"},
		{"KEYS", "", "list all keys"},
	}
	for _, c := range cmds {
		sb.WriteString(fmt.Sprintf("%-10s %-26s %s\n", c.name, c.args, c.desc))
	}
	return sb.String()
}

func formatDuration(d time.Duration) string {
	if d < time.Minute { return fmt.Sprintf("%ds", int(d.Seconds())) }
	if d < time.Hour { return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60) }
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

func printBanner() {
	fmt.Println()
	fmt.Println(cyan + "goredis server v" + VERSION + reset + " " + dim + "- listening on port " + PORT + reset)
	fmt.Println(dim + "waiting for connections... (telnet or redis-cli)" + reset)
	fmt.Println()
}