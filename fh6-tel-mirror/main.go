package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Statistics tracker
var (
	rxPackets  uint64
	txPackets  uint64
	totalDelay int64 // cumulative delay in nanoseconds
	livePps    uint64
)

// Inbound UDP Socket reference
var (
	udpConn     *net.UDPConn
	udpConnLock sync.Mutex
)

// Thread-safe active destination map
type ActiveTarget struct {
	Addr       *net.UDPAddr
	Conn       *net.UDPConn
	RateLimit  string
	Count      uint64
}

var (
	activeTargets = make(map[string]*ActiveTarget)
	targetsLock   sync.RWMutex
)

// Interactive TUI Prompt lock
var (
	isPrompting bool
	promptLock  sync.Mutex
)

// Rebuild UDP targets from configurations
func rebuildTargets() {
	targetsLock.Lock()
	defer targetsLock.Unlock()

	// Close existing active sockets
	for _, t := range activeTargets {
		if t.Conn != nil {
			t.Conn.Close()
		}
	}
	activeTargets = make(map[string]*ActiveTarget)

	configLock.RLock()
	defer configLock.RUnlock()

	for _, dest := range config.Destinations {
		if !dest.Enabled {
			continue
		}

		addrStr := fmt.Sprintf("%s:%d", dest.Host, dest.Port)
		raddr, err := net.ResolveUDPAddr("udp", addrStr)
		if err != nil {
			log.Printf("[Router] Error resolving address %s: %v", addrStr, err)
			continue
		}

		conn, err := net.DialUDP("udp", nil, raddr)
		if err != nil {
			log.Printf("[Router] Error dialing UDP socket to %s: %v", addrStr, err)
			continue
		}

		activeTargets[dest.ID] = &ActiveTarget{
			Addr:      raddr,
			Conn:      conn,
			RateLimit: dest.RateLimit,
			Count:     0,
		}
	}
}

func startUDPListener() {
	var connErr error
	configLock.RLock()
	bindAddr := fmt.Sprintf("0.0.0.0:%d", config.BindPort)
	configLock.RUnlock()

	laddr, err := net.ResolveUDPAddr("udp", bindAddr)
	if err != nil {
		log.Printf("[UDP] Error resolving bind address %s: %v", bindAddr, err)
		return
	}

	udpConnLock.Lock()
	if udpConn != nil {
		udpConn.Close()
	}
	udpConn, connErr = net.ListenUDP("udp", laddr)
	if connErr != nil {
		udpConnLock.Unlock()
		log.Printf("[UDP] Bind failure on %s: %v", bindAddr, connErr)
		return
	}
	udpConnLock.Unlock()

	buffer := make([]byte, 512)
	for {
		udpConnLock.Lock()
		activeConn := udpConn
		udpConnLock.Unlock()

		if activeConn == nil {
			break
		}

		n, _, err := activeConn.ReadFromUDP(buffer)
		if err != nil {
			break
		}

		if n > 0 {
			atomic.AddUint64(&rxPackets, 1)
			packetData := make([]byte, n)
			copy(packetData, buffer[:n])

			go routePacket(packetData)
		}
	}
}

// Low-latency packet duplicator and router
func routePacket(data []byte) {
	start := time.Now()
	
	targetsLock.RLock()
	defer targetsLock.RUnlock()

	var wg sync.WaitGroup
	for _, target := range activeTargets {
		target.Count++
		
		// Rate Limiting Logic: thinning 60Hz streams
		skip := false
		switch target.RateLimit {
		case "30Hz":
			if target.Count%2 != 0 {
				skip = true
			}
		case "20Hz":
			if target.Count%3 != 0 {
				skip = true
			}
		case "10Hz":
			if target.Count%6 != 0 {
				skip = true
			}
		}

		if skip {
			continue
		}

		wg.Add(1)
		go func(t *ActiveTarget) {
			defer wg.Done()
			_, err := t.Conn.Write(data)
			if err == nil {
				atomic.AddUint64(&txPackets, 1)
			}
		}(target)
	}
	wg.Wait()

	delay := time.Since(start).Nanoseconds()
	atomic.AddInt64(&totalDelay, delay)
}

// Track PPS and reset metrics every second
func startStatsTicker() {
	ticker := time.NewTicker(1 * time.Second)
	var prevRx uint64
	for range ticker.C {
		currentRx := atomic.LoadUint64(&rxPackets)
		pps := currentRx - prevRx
		prevRx = currentRx
		atomic.StoreUint64(&livePps, pps)
	}
}

// Draw premium native console Dashboard UI
func drawConsole() {
	promptLock.Lock()
	if isPrompting {
		promptLock.Unlock()
		return
	}
	promptLock.Unlock()

	// Clear terminal screen and reset cursor to home (HTOP style)
	fmt.Print("\033[H\033[2J")

	rx := atomic.LoadUint64(&rxPackets)
	tx := atomic.LoadUint64(&txPackets)
	pps := atomic.LoadUint64(&livePps)
	var avgUs int64
	if rx > 0 {
		avgUs = (atomic.LoadInt64(&totalDelay) / int64(rx)) / 1000
	}

	configLock.RLock()
	bindPort := config.BindPort
	dests := make([]Destination, len(config.Destinations))
	copy(dests, config.Destinations)
	configLock.RUnlock()

	fmt.Println("\033[36m==================================================================\033[0m")
	fmt.Println("       🏁 \033[1mFH6 TELEMETRY MIRROR MIDDLEWARE (NATIVE TUI)\033[0m 🏁")
	fmt.Println("\033[36m==================================================================\033[0m")
	fmt.Printf("  STATUS: \033[32mACTIVE\033[0m  |  INBOUND UDP PORT: \033[33m%-5d\033[0m\n", bindPort)
	
	statusText := "OFFLINE (Waiting for Forza UDP stream...)"
	statusColor := "\033[31;1m" // Blinking/bold red
	if pps > 0 {
		statusText = "LIVE LINK CONNECTED (Receiving active telemetry)"
		statusColor = "\033[32;1m" // Bold Green
	}
	fmt.Printf("  TELEMETRY STREAM: %s%s\033[0m\n", statusColor, statusText)
	fmt.Printf("  INBOUND PAYLOAD:  \033[1m%-3d pps\033[0m  |  AVG DUP LATENCY: \033[1m%d µs\033[0m\n", pps, avgUs)
	fmt.Printf("  PACKETS - RX: \033[35m%-6d\033[0m  |  TX (MIRRORED): \033[35m%d\033[0m\n", rx, tx)
	fmt.Println("\033[36m==================================================================\033[0m")
	fmt.Println(" ACTIVE MIRROR DESTINATIONS:")
	fmt.Println("------------------------------------------------------------------")
	fmt.Printf(" #   %-22s %-20s %-8s %-8s\n", "NAME", "ADDRESS", "LIMIT", "STATUS")
	fmt.Println("------------------------------------------------------------------")
	
	for i, d := range dests {
		status := "\033[31mDISABLED\033[0m"
		if d.Enabled {
			status = "\033[32mENABLED\033[0m"
		}
		addr := fmt.Sprintf("%s:%d", d.Host, d.Port)
		fmt.Printf(" [%d] %-22s %-20s %-8s %s\n", i+1, d.Name, addr, d.RateLimit, status)
	}
	if len(dests) == 0 {
		fmt.Println("  (No destination nodes configured. Press 'A' to add one!)")
	}
	fmt.Println("\033[36m==================================================================\033[0m")
	fmt.Println(" MENU COMMANDS:")
	fmt.Println(" \033[1;36m[A]\033[0m Add Node  |  \033[1;36m[T]\033[0m Toggle Node  |  \033[1;36m[D]\033[0m Delete Node")
	fmt.Println(" \033[1;36m[P]\033[0m Set Inbound UDP Port    |  \033[1;31m[Q]\033[0m Exit App")
	fmt.Println("\033[36m==================================================================\033[0m")
	fmt.Print("Enter command: ")
}

// Background TUI screen refresher
func startTUIRefresher() {
	ticker := time.NewTicker(1 * time.Second)
	for range ticker.C {
		promptLock.Lock()
		prompting := isPrompting
		promptLock.Unlock()

		if !prompting {
			drawConsole()
		}
	}
}

// Blocking keyboard input processing loop
func runInputLoop() {
	reader := bufio.NewReader(os.Stdin)
	for {
		drawConsole()

		input, err := reader.ReadString('\n')
		if err != nil {
			continue
		}

		cmd := strings.TrimSpace(strings.ToLower(input))
		if cmd == "" {
			continue
		}

		switch cmd {
		case "q":
			fmt.Println("\n\033[31mShutting down FH6 Mirror Middleware... Bye!\033[0m")
			os.Exit(0)
		case "a":
			promptLock.Lock()
			isPrompting = true
			promptLock.Unlock()

			fmt.Println("\n\033[36m--- ADD NEW DESTINATION NODE ---\033[0m")
			fmt.Print("Enter Friendly Name (e.g. SimHub): ")
			name, _ := reader.ReadString('\n')
			name = strings.TrimSpace(name)

			fmt.Print("Enter Destination IP (e.g. 192.168.1.10): ")
			host, _ := reader.ReadString('\n')
			host = strings.TrimSpace(host)

			fmt.Print("Enter Destination UDP Port (e.g. 20500): ")
			portStr, _ := reader.ReadString('\n')
			port, _ := strconv.Atoi(strings.TrimSpace(portStr))

			fmt.Print("Enter Rate Limit (60Hz, 30Hz, 20Hz, 10Hz): ")
			rate, _ := reader.ReadString('\n')
			rate = strings.TrimSpace(rate)
			if rate != "60Hz" && rate != "30Hz" && rate != "20Hz" && rate != "10Hz" {
				rate = "60Hz"
			}

			if name != "" && host != "" && port > 0 {
				id := fmt.Sprintf("node-%d", time.Now().Unix())
				configLock.Lock()
				config.Destinations = append(config.Destinations, Destination{
					ID:        id,
					Name:      name,
					Host:      host,
					Port:      port,
					RateLimit: rate,
					Enabled:   true,
				})
				saveConfigLocked()
				configLock.Unlock()
				rebuildTargets()
				fmt.Println("\n\033[32m✅ Destination node registered successfully!\033[0m")
			} else {
				fmt.Println("\n\033[31m❌ Invalid input fields. Action aborted.\033[0m")
			}
			time.Sleep(1500 * time.Millisecond)

			promptLock.Lock()
			isPrompting = false
			promptLock.Unlock()

		case "t":
			promptLock.Lock()
			isPrompting = true
			promptLock.Unlock()

			fmt.Println("\n\033[36m--- TOGGLE ACTIVE STATUS ---\033[0m")
			fmt.Print("Enter Destination Number [#] to Toggle: ")
			numStr, _ := reader.ReadString('\n')
			idx, err := strconv.Atoi(strings.TrimSpace(numStr))
			idx = idx - 1 // 0-based mapping

			configLock.Lock()
			if err == nil && idx >= 0 && idx < len(config.Destinations) {
				config.Destinations[idx].Enabled = !config.Destinations[idx].Enabled
				saveConfigLocked()
				fmt.Printf("\n\033[32m✅ Toggled %s successfully (Active: %t)!\033[0m\n", config.Destinations[idx].Name, config.Destinations[idx].Enabled)
			} else {
				fmt.Println("\n\033[31m❌ Invalid destination index.\033[0m")
			}
			configLock.Unlock()
			rebuildTargets()
			time.Sleep(1500 * time.Millisecond)

			promptLock.Lock()
			isPrompting = false
			promptLock.Unlock()

		case "d":
			promptLock.Lock()
			isPrompting = true
			promptLock.Unlock()

			fmt.Println("\n\033[31;1m--- DELETE DESTINATION NODE ---\033[0m")
			fmt.Print("Enter Destination Number [#] to Delete: ")
			numStr, _ := reader.ReadString('\n')
			idx, err := strconv.Atoi(strings.TrimSpace(numStr))
			idx = idx - 1 // 0-based mapping

			configLock.Lock()
			if err == nil && idx >= 0 && idx < len(config.Destinations) {
				name := config.Destinations[idx].Name
				config.Destinations = append(config.Destinations[:idx], config.Destinations[idx+1:]...)
				saveConfigLocked()
				fmt.Printf("\n\033[32m🗑️ Deleted %s from routing table successfully!\033[0m\n", name)
			} else {
				fmt.Println("\n\033[31m❌ Invalid destination index.\033[0m")
			}
			configLock.Unlock()
			rebuildTargets()
			time.Sleep(1500 * time.Millisecond)

			promptLock.Lock()
			isPrompting = false
			promptLock.Unlock()

		case "p":
			promptLock.Lock()
			isPrompting = true
			promptLock.Unlock()

			fmt.Println("\n\033[36m--- SET INBOUND TELEMETRY PORT ---\033[0m")
			fmt.Print("Enter Inbound UDP Bind Port (e.g. 20440): ")
			portStr, _ := reader.ReadString('\n')
			port, err := strconv.Atoi(strings.TrimSpace(portStr))

			if err == nil && port > 0 {
				configLock.Lock()
				portChanged := config.BindPort != port
				config.BindPort = port
				saveConfigLocked()
				configLock.Unlock()

				if portChanged {
					go startUDPListener()
				}
				fmt.Println("\n\033[32m✅ Inbound UDP port set successfully!\033[0m")
			} else {
				fmt.Println("\n\033[31m❌ Invalid UDP port number.\033[0m")
			}
			time.Sleep(1500 * time.Millisecond)

			promptLock.Lock()
			isPrompting = false
			promptLock.Unlock()
		}
	}
}

func main() {
	// Pipe standard log prints to a local file to prevent cluttering the TUI console
	logFile, err := os.OpenFile("mirror.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		log.SetOutput(logFile)
	} else {
		log.SetOutput(io.Discard)
	}

	if err := loadConfig(); err != nil {
		log.Fatalf("Fatal error loading config: %v", err)
	}

	rebuildTargets()
	go startUDPListener()
	go startStatsTicker()

	// Launch Native TUI Refresher and Input Loop
	go startTUIRefresher()
	runInputLoop()
}
