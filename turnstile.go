package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	turnstileSiteKey = "0x4AAAAAAAgU7T2niLQD-TLm"
	capmonsterURL    = "https://api.capmonster.cloud"
	capsolverURL     = "https://api.capsolver.com"
)

type capmonsterCreateRequest struct {
	ClientKey string         `json:"clientKey"`
	Task      capmonsterTask `json:"task"`
}

type capmonsterTask struct {
	Type          string `json:"type"`
	WebsiteURL    string `json:"websiteURL"`
	WebsiteKey    string `json:"websiteKey"`
	ProxyType     string `json:"proxyType,omitempty"`
	ProxyAddress  string `json:"proxyAddress,omitempty"`
	ProxyPort     int    `json:"proxyPort,omitempty"`
	ProxyLogin    string `json:"proxyLogin,omitempty"`
	ProxyPassword string `json:"proxyPassword,omitempty"`
}

type capmonsterCreateResponse struct {
	ErrorID int `json:"errorId"`
	TaskID  int `json:"taskId"`
}

type capmonsterResultRequest struct {
	ClientKey string `json:"clientKey"`
	TaskID    int    `json:"taskId"`
}

type capmonsterResultResponse struct {
	ErrorID  int    `json:"errorId"`
	Status   string `json:"status"`
	Solution struct {
		Token string `json:"token"`
	} `json:"solution"`
}

func SolveTurnstile(ctx context.Context, solverService, apiKey, pageURL string) (string, error) {
	if solverService == "capsolver" {
		return solveCapsolver(ctx, apiKey, pageURL)
	}
	return solveCapmonster(ctx, apiKey, pageURL)
}

func solveCapmonster(ctx context.Context, apiKey, pageURL string) (string, error) {
	task := capmonsterTask{
		Type:       "TurnstileTaskProxyless",
		WebsiteURL: pageURL,
		WebsiteKey: turnstileSiteKey,
	}

	createBody, _ := json.Marshal(capmonsterCreateRequest{ClientKey: apiKey, Task: task})
	req, err := http.NewRequestWithContext(ctx, "POST", capmonsterURL+"/createTask", bytes.NewReader(createBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var createResp capmonsterCreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return "", err
	}
	if createResp.ErrorID != 0 {
		return "", fmt.Errorf("capmonster createTask error %d", createResp.ErrorID)
	}

	resultBody, _ := json.Marshal(capmonsterResultRequest{ClientKey: apiKey, TaskID: createResp.TaskID})

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(3 * time.Second):
		}

		req, err = http.NewRequestWithContext(ctx, "POST", capmonsterURL+"/getTaskResult", bytes.NewReader(resultBody))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}

		var result capmonsterResultResponse
		if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return "", fmt.Errorf("capmonster getTaskResult decode: %w", err)
		}
		resp.Body.Close()

		if result.ErrorID != 0 {
			return "", fmt.Errorf("capmonster getTaskResult error %d", result.ErrorID)
		}
		if result.Status == "ready" {
			return result.Solution.Token, nil
		}
	}
}

func solveCapsolver(ctx context.Context, apiKey, pageURL string) (string, error) {
	task := map[string]interface{}{
		"type":       "AntiTurnstileTaskProxyLess",
		"websiteURL": pageURL,
		"websiteKey": turnstileSiteKey,
	}

	createBody, _ := json.Marshal(map[string]interface{}{
		"clientKey": apiKey,
		"task":      task,
	})
	req, err := http.NewRequestWithContext(ctx, "POST", capsolverURL+"/createTask", bytes.NewReader(createBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var createResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return "", err
	}
	if errID, ok := createResp["errorId"].(float64); ok && errID != 0 {
		code, _ := createResp["errorCode"].(string)
		desc, _ := createResp["errorDescription"].(string)
		return "", fmt.Errorf("capsolver createTask error %v: %s - %s", errID, code, desc)
	}

	taskID := createResp["taskId"]
	if taskID == nil {
		return "", fmt.Errorf("capsolver createTask no taskId returned")
	}

	resultBody, _ := json.Marshal(map[string]interface{}{
		"clientKey": apiKey,
		"taskId":    taskID,
	})

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(3 * time.Second):
		}

		req, err = http.NewRequestWithContext(ctx, "POST", capsolverURL+"/getTaskResult", bytes.NewReader(resultBody))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}

		var result map[string]interface{}
		if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return "", fmt.Errorf("capsolver getTaskResult decode: %w", err)
		}
		resp.Body.Close()

		if errID, ok := result["errorId"].(float64); ok && errID != 0 {
			code, _ := result["errorCode"].(string)
			desc, _ := result["errorDescription"].(string)
			return "", fmt.Errorf("capsolver getTaskResult error %v: %s - %s", errID, code, desc)
		}
		if status, _ := result["status"].(string); status == "ready" {
			if sol, ok := result["solution"].(map[string]interface{}); ok {
				if token, ok := sol["token"].(string); ok {
					return token, nil
				}
			}
			return "", fmt.Errorf("capsolver ready but no token")
		}
	}
}
