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
		fmt.Println("Error: OPENROUTER_API_KEY environment variable is not set.")
		os.Exit(1)
	}

	var rawJSON []byte
	var err error

	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		rawJSON, err = io.ReadAll(os.Stdin)
	} else if len(os.Args) > 1 {
		rawJSON, err = os.ReadFile(os.Args[1])
	} else {
		fmt.Println("Usage: cat plan.json | go run .  OR  go run . plan.json")
		os.Exit(1)
	}

	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	rawJSON = bytes.TrimPrefix(rawJSON, []byte("\xef\xbb\xbf")) // UTF-8 BOM
	rawJSON = bytes.TrimPrefix(rawJSON, []byte("\xff\xfe"))     // UTF-16LE BOM
	
	// Step 1: Filter the JSON
	filteredPlan, err := ParseAndFilter(rawJSON)
	if err != nil {
		fmt.Printf("Filter error: %v\n", err)
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
		Model: "meta-llama/llama-3.3-70b-instruct", // Free/low-cost fast OpenRouter model
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
	}

	jsonReq, _ := json.Marshal(reqBody)

	// Step 3: Send HTTP Request to OpenRouter
	req, _ := http.NewRequest("POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(jsonReq))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("HTTP Request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var apiResp OpenRouterResponse
	json.Unmarshal(respBody, &apiResp)

	if apiResp.Error != nil {
		fmt.Printf("OpenRouter API Error: %s\n", apiResp.Error.Message)
		os.Exit(1)
	}

	if len(apiResp.Choices) > 0 {
		fmt.Println("\n--- TERRAFORM PLAN EXPLANATION ---")
		fmt.Println(apiResp.Choices[0].Message.Content)
	} else {
		fmt.Println("No response generated.")
	}
}