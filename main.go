package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36"

func loadProxies(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var proxies []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		proxies = append(proxies, line)
	}
	return proxies, nil
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.APIKey == "" || cfg.APIKey == "YOUR_API_KEY_HERE" {
		return nil, fmt.Errorf("please put your API key in %s", path)
	}
	if cfg.SolverService != "capmonster" && cfg.SolverService != "capsolver" {
		return nil, fmt.Errorf("invalid solver_service in %s, must be capmonster or capsolver", path)
	}
	return &cfg, nil
}

type Config struct {
	SolverService string `json:"solver_service"`
	APIKey        string `json:"api_key"`
}

func prompt(label string) string {
	fmt.Printf("  %s%s►%s %s%s:%s ", Bold, BrightMagenta, Reset, Bold, label, Reset)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	return strings.TrimSpace(scanner.Text())
}

func promptDefault(label, defaultVal string) string {
	fmt.Printf("  %s%s►%s %s%s%s %s(%s)%s: ", Bold, BrightMagenta, Reset, Bold, label, Reset, Dim, defaultVal, Reset)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	val := strings.TrimSpace(scanner.Text())
	if val == "" {
		return defaultVal
	}
	return val
}

type Stats struct {
	success  atomic.Int64
	failed   atomic.Int64
	total    atomic.Int64
	username string
}

func (s *Stats) AddSuccess() {
	s.success.Add(1)
	s.total.Add(1)
	s.updateTitle()
}

func (s *Stats) AddFail() {
	s.failed.Add(1)
	s.total.Add(1)
	s.updateTitle()
}

func (s *Stats) updateTitle() {
	setTitle(fmt.Sprintf("GUNS | %s | ✓ %d | ✗ %d | Total: %d",
		s.username, s.success.Load(), s.failed.Load(), s.total.Load()))
}

func (s *Stats) PrintStats() {
	fmt.Printf("\n  %s───── %s✓ %d%s  %s✗ %d%s  %sTotal %d%s ─────%s\n\n",
		Dim,
		BrightGreen+Bold, s.success.Load(), Dim,
		BrightRed, s.failed.Load(), Dim,
		Reset, s.total.Load(), Dim,
		Reset)
}

func main() {
	printBanner()
	setTitle("GUNS | Starting...")

	cfg, err := loadConfig("config.json")
	if err != nil {
		logError("config.json: %s", err)
		logInfo("Configure your solver in %sconfig.json%s", Yellow, Reset)
		fmt.Println()
		fmt.Print("  Press Enter to exit...")
		bufio.NewReader(os.Stdin).ReadBytes('\n')
		os.Exit(1)
	}
	logSuccess("Solver config loaded (%s)", cfg.SolverService)

	proxies, err := loadProxies("proxies.txt")
	if err != nil {
		logWarn("No proxies.txt — direct connection")
		proxies = []string{}
	} else if len(proxies) == 0 {
		logWarn("proxies.txt empty — direct connection")
	} else {
		logSuccess("Loaded %s%d%s proxies", BrightYellow, len(proxies), Reset)
	}

	fmt.Println()

	username := prompt("Username")
	if username == "" {
		logError("Username is required!")
		os.Exit(1)
	}

	threadCountStr := promptDefault("Threads", "20")
	threadCount, err := strconv.Atoi(threadCountStr)
	if err != nil || threadCount < 1 {
		logError("Invalid thread count")
		os.Exit(1)
	}

	fmt.Println()
	printSeparator()
	fmt.Printf("  %sUser:%s %s%-12s%s %sThreads:%s %s%d%s  %sProxies:%s %s%d%s\n",
		Dim, Reset, BrightYellow+Bold, username, Reset,
		Dim, Reset, BrightYellow+Bold, threadCount, Reset,
		Dim, Reset, BrightYellow+Bold, len(proxies), Reset)
	printSeparator()
	fmt.Println()

	stats := &Stats{username: username}
	stats.updateTitle()

	logStep("Starting %s%d%s threads", BrightYellow, threadCount, Reset)
	fmt.Println()

	var wg sync.WaitGroup
	for i := 0; i < threadCount; i++ {
		wg.Add(1)
		go func(threadID int) {
			defer wg.Done()
			runThread(threadID, username, cfg, proxies, stats)
		}(i)
		time.Sleep(time.Duration(200+rand.Intn(400)) * time.Millisecond)
	}

	wg.Wait()
}

func runThread(threadID int, username string, cfg *Config, proxies []string, stats *Stats) {
	iteration := 0

	for {
		iteration++

		var proxyURL string
		if len(proxies) > 0 {
			proxyURL = proxies[(threadID+iteration)%len(proxies)]
		}

		sess, err := NewSession(proxyURL, threadID)
		if err != nil {
			logThreadError(threadID, "Session error: %s", err)
			stats.AddFail()
			time.Sleep(2 * time.Second)
			continue
		}

		logThread(threadID, "iteration %s#%d%s", Dim, iteration, Reset)

		err = runSolveIteration(sess, threadID, username, cfg, iteration)
		if err != nil {
			logThreadError(threadID, "#%d: %s", iteration, err)
			stats.AddFail()
			backoff := time.Duration(1+rand.Intn(2)) * time.Second
			time.Sleep(backoff)
			continue
		}

		logThreadSuccess(threadID, "#%d sent (%s✓%d%s/%s✗%d%s)",
			iteration,
			BrightGreen, stats.success.Load(), Reset,
			BrightRed, stats.failed.Load(), Reset)
		stats.AddSuccess()

		time.Sleep(time.Duration(300+rand.Intn(700)) * time.Millisecond)
	}
}

func runSolveIteration(sess *Session, threadID int, username string, cfg *Config, iteration int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	start := time.Now()

	var workerData *WorkerData
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		workerData, err = sess.FetchWorkerData(ctx, username)
		if err == nil && workerData != nil {
			break
		}
		if attempt < 3 {
			backoff := time.Duration(attempt) * time.Second
			logThread(threadID, "%sfetch %d/3 failed, retry in %s...%s", Dim, attempt, backoff, Reset)
			time.Sleep(backoff)
		}
	}
	if err != nil {
		return fmt.Errorf("fetch (3x): %w", err)
	}
	if workerData == nil {
		return fmt.Errorf("no worker data")
	}
	data := *workerData
	fetchTime := time.Since(start).Round(time.Millisecond)

	logThread(threadID, "%sSolving PoW + Turnstile...%s", Dim, Reset)
	type wasmResult struct {
		res *WasmResult
		err error
		dur time.Duration
	}
	type turnstileResult struct {
		token string
		err   error
		dur   time.Duration
	}

	wasmCh := make(chan wasmResult, 1)
	turnstileCh := make(chan turnstileResult, 1)

	var solveWg sync.WaitGroup

	solveWg.Add(1)
	go func() {
		defer solveWg.Done()
		s := time.Now()
		res, err := SolveWithWasm(ctx, data.O09, 5, strconv.FormatInt(data.OriginalTimestamp, 10), data.Nonce, data.Underscore2xa)
		wasmCh <- wasmResult{res, err, time.Since(s)}
	}()

	solveWg.Add(1)
	go func() {
		defer solveWg.Done()
		s := time.Now()
		token, err := SolveTurnstile(ctx, cfg.SolverService, cfg.APIKey, "https://guns.lol/"+username)
		turnstileCh <- turnstileResult{token, err, time.Since(s)}
	}()

	solveWg.Wait()

	wasmR := <-wasmCh
	if wasmR.err != nil {
		return fmt.Errorf("pow: %w", wasmR.err)
	}
	logThread(threadID, "PoW solved! _oo=%s %s(%s)%s", BrightYellow, truncate(wasmR.res.Oo, 16), Reset, Dim, wasmR.dur.Round(time.Millisecond), Reset)

	tR := <-turnstileCh
	if tR.err != nil {
		return fmt.Errorf("turnstile: %w", tR.err)
	}
	logThread(threadID, "Turnstile solved! %s(%s)%s", Dim, tR.dur.Round(time.Millisecond), Reset)

	payload := SolutionPayload{
		Username:          username,
		Nonce:             data.Nonce,
		O09:               data.O09,
		Timestamp:         data.OriginalTimestamp,
		Underscore2xa:     data.Underscore2xa,
		Oo:                wasmR.res.Oo,
		Seal:              wasmR.res.Seal,
		TurnstileResponse: tR.token,
	}

	err = sess.SubmitSolution(payload)
	if errors.Is(err, ErrClearanceExpired) {
		logThread(threadID, "%sclearance expired, re-solving...%s", Yellow, Reset)
		sess.Clearance = ""
		_, fetchErr := sess.FetchWorkerData(ctx, username)
		if fetchErr == nil {
			err = sess.SubmitSolution(payload)
		}
	}
	if err != nil {
		return fmt.Errorf("submit: %w", err)
	}

	totalTime := time.Since(start).Round(time.Millisecond)
	logThread(threadID, "%sfetch %s · pow %s · captcha %s · total %s%s",
		Dim, fetchTime, wasmR.dur.Round(time.Millisecond), tR.dur.Round(time.Millisecond), totalTime, Reset)

	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
