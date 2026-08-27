package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"github.com/joho/godotenv"
)

type OpenRouterRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenRouterResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func main() {
	_ = godotenv.Load()

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: OPENROUTER_API_KEY is missing.")
		os.Exit(1)
	}

	var rawJSON []byte
	var err error

	// FIX: Prioritize command-line file argument over stdin!
	if len(os.Args) > 1 {
		filePath := os.Args[1]
		rawJSON, err = os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", filePath, err)
			os.Exit(1)
		}
	} else {
		// Check if standard input has data piped into it
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			rawJSON, err = io.ReadAll(os.Stdin)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
				os.Exit(1)
			}
		} else {
			fmt.Fprintln(os.Stderr, "Usage: tf-explain <plan.json> OR cat plan.json | tf-explain")
			os.Exit(1)
		}
	}

	// Clean UTF Byte Order Marks
	rawJSON = bytes.TrimPrefix(rawJSON, []byte("\xef\xbb\xbf"))
	rawJSON = bytes.TrimPrefix(rawJSON, []byte("\xff\xfe"))

	// Step 1: Parse and Filter JSON
	filteredPlan, err := ParseAndFilter(rawJSON)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Filter error: %v\n", err)
		os.Exit(1)
	}

	filteredJSONBytes, _ := json.MarshalIndent(filteredPlan, "", "  ")

	// Step 2: Build OpenRouter Prompt
	prompt := fmt.Sprintf(`You are a Cloud Infrastructure & DevOps Auditor. Analyze this filtered Terraform plan and explain the impact.

Terraform Changes Payload:
%s

Output Format:
1. Executive Summary: 2-sentence impact report.
2. Resource Change Breakdown: Grouped by resource type.
3. Security & Destructive Action Warnings: Explicitly highlight deletions or unsafe configurations.`, string(filteredJSONBytes))

	reqBody := OpenRouterRequest{
		Model: "meta-llama/llama-3.3-70b-instruct",
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
	}

	jsonReq, _ := json.Marshal(reqBody)

	// Step 3: Send HTTP Request
	req, _ := http.NewRequest("POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(jsonReq))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "HTTP Request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var apiResp OpenRouterResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse LLM API response: %v\nRaw response: %s\n", err, string(respBody))
		os.Exit(1)
	}

	if apiResp.Error != nil {
		fmt.Fprintf(os.Stderr, "OpenRouter API Error: %s\n", apiResp.Error.Message)
		os.Exit(1)
	}

	if len(apiResp.Choices) > 0 {
		fmt.Println(apiResp.Choices[0].Message.Content)
	} else {
		fmt.Fprintln(os.Stderr, "No response generated from model.")
		os.Exit(1)
	}
}