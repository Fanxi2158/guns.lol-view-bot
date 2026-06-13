package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	nonceRegex         = regexp.MustCompile(`_n: '([a-zA-Z0-9]{31,32})',`)
	o09Regex           = regexp.MustCompile(`o09: '([a-f0-9]{64})',`)
	underscore2xaRegex = regexp.MustCompile(`_2xa: '([a-zA-Z0-9_-]{80,})',`)
	timestampRegex     = regexp.MustCompile(`org_ts: \\\"(\d+)\\\",`)
	challengeNonceRegex = regexp.MustCompile(`[{,]_n:"([^"]+)"`)
	challengeO09Regex   = regexp.MustCompile(`[{,]o09:"([^"]+)"`)
	challenge2xaRegex   = regexp.MustCompile(`[{,]_2xa:"([^"]+)"`)
	challengeOrgTsRegex = regexp.MustCompile(`[{,]_org_ts:"([^"]+)"`)
	challengeDRegex     = regexp.MustCompile(`,d:"([^"]+)"`)
	challengeSRegex     = regexp.MustCompile(`[{,]__s:"([^"]+)"`)
)

type Session struct {
	Client    *http.Client
	Clearance string
	ProxyURL  string
	ThreadID  int
}

func NewSession(proxyURL string, threadID int) (*Session, error) {
	transport := &http.Transport{
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: false},
		DialContext: (&net.Dialer{
			Timeout:   15 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		MaxIdleConns:          10,
		MaxIdleConnsPerHost:   2,
		MaxConnsPerHost:       3,
		IdleConnTimeout:       60 * time.Second,
		DisableKeepAlives:     false,
	}

	if proxyURL != "" {
		parsed, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL %q: %w", proxyURL, err)
		}
		transport.Proxy = http.ProxyURL(parsed)
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   45 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	s := &Session{
		Client:   client,
		ProxyURL: proxyURL,
		ThreadID: threadID,
	}

	if data, err := os.ReadFile("clearance.txt"); err == nil {
		s.Clearance = strings.TrimSpace(string(data))
	}

	return s, nil
}

func (s *Session) saveClearance() {
	os.WriteFile("clearance.txt", []byte(s.Clearance), 0600)
}

type WorkerData struct {
	Nonce             string
	O09               string
	Underscore2xa     string
	OriginalTimestamp int64
}

func (s *Session) FetchWorkerData(ctx context.Context, username string) (*WorkerData, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://guns.lol/"+username, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	req.AddCookie(&http.Cookie{Name: "GUNS_LOCALE", Value: "en"})
	req.AddCookie(&http.Cookie{Name: "GUNS_PATH_LOCALE", Value: "en"})
	if s.Clearance != "" {
		req.AddCookie(&http.Cookie{Name: "guns_clearance", Value: s.Clearance})
	}

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	body := string(bodyBytes)

	if resp.StatusCode == http.StatusUnauthorized {
		if s.Clearance != "" {
			s.Clearance = ""
			os.Remove("clearance.txt")
		}
		logThread(s.ThreadID, "Got 401, solving clearance challenge...")
		if err = s.solveChallenge(ctx, body); err != nil {
			return nil, fmt.Errorf("challenge: %w", err)
		}
		return s.FetchWorkerData(ctx, username)
	}

	if resp.StatusCode == http.StatusTemporaryRedirect {
		for _, cookie := range resp.Cookies() {
			if cookie.Name == "guns_clearance" {
				s.Clearance = cookie.Value
				s.saveClearance()
				return s.FetchWorkerData(ctx, username)
			}
		}
		if s.Clearance != "" {
			location := resp.Header.Get("Location")
			return s.FetchWorkerData(ctx, location[1:])
		}
		return nil, errors.New("307 redirect without clearance cookie")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code: %d %s", resp.StatusCode, resp.Status)
	}

	nonce := nonceRegex.FindStringSubmatch(body)
	o09 := o09Regex.FindStringSubmatch(body)
	underscore2xa := underscore2xaRegex.FindStringSubmatch(body)
	timestamp := timestampRegex.FindStringSubmatch(body)

	if nonce == nil || o09 == nil || underscore2xa == nil || timestamp == nil {
		return nil, errors.New("failed to parse worker data from response")
	}

	originalTs, err := strconv.ParseInt(timestamp[1], 10, 64)
	if err != nil {
		return nil, err
	}

	return &WorkerData{
		Nonce:             nonce[1],
		O09:               o09[1],
		Underscore2xa:     underscore2xa[1],
		OriginalTimestamp: originalTs,
	}, nil
}

func (s *Session) solveChallenge(ctx context.Context, body string) error {
	nonce := challengeNonceRegex.FindStringSubmatch(body)
	o09 := challengeO09Regex.FindStringSubmatch(body)
	twoXa := challenge2xaRegex.FindStringSubmatch(body)
	orgTs := challengeOrgTsRegex.FindStringSubmatch(body)
	d := challengeDRegex.FindStringSubmatch(body)
	sv := challengeSRegex.FindStringSubmatch(body)

	if nonce == nil || o09 == nil || twoXa == nil || orgTs == nil || d == nil || sv == nil {
		return errors.New("failed to parse challenge data from page")
	}

	difficulty, err := strconv.Atoi(d[1])
	if err != nil {
		return fmt.Errorf("invalid difficulty %q: %w", d[1], err)
	}

	logThread(s.ThreadID, "Challenge params: o09=%s nonce=%s difficulty=%d", truncate(o09[1], 16), nonce[1], difficulty)
	res, err := SolveWithWasm(ctx, o09[1], difficulty, orgTs[1], nonce[1], twoXa[1])
	if err != nil {
		return fmt.Errorf("wasm solve: %w", err)
	}
	logThread(s.ThreadID, "Challenge solved: _oo=%s", truncate(res.Oo, 20))

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.WriteField("_o", res.Oo)
	w.WriteField("_s", sv[1])
	w.WriteField("_u", nonce[1])
	w.WriteField("_i", twoXa[1])
	w.WriteField("_x", o09[1])
	w.WriteField("_t", orgTs[1])
	w.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", "https://guns.lol/_challenge/verify", &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("User-Agent", userAgent)

	vresp, err := s.Client.Do(req)
	if err != nil {
		return err
	}
	defer vresp.Body.Close()
	io.ReadAll(vresp.Body)

	if vresp.StatusCode != http.StatusOK {
		return fmt.Errorf("verify returned %d", vresp.StatusCode)
	}

	for _, cookie := range vresp.Cookies() {
		if cookie.Name == "guns_clearance" {
			s.Clearance = cookie.Value
			s.saveClearance()
			return nil
		}
	}
	return errors.New("no clearance cookie in verify response")
}
