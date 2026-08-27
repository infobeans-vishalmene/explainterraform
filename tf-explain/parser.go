package main

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// Terraform Plan standard JSON schema structures
type Plan struct {
	ResourceChanges []ResourceChange `json:"resource_changes"`
}

type ResourceChange struct {
	Address string `json:"address"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Change  Change `json:"change"`
}

type Change struct {
	Actions         []string               `json:"actions"`
	Before          map[string]interface{} `json:"before"`
	After           map[string]interface{} `json:"after"`
	BeforeSensitive interface{}            `json:"before_sensitive"`
	AfterSensitive  interface{}            `json:"after_sensitive"`
}

// Cleaned / Filtered Structures optimized for LLM prompts
type FilteredPlan struct {
	Summary         PlanSummary              `json:"summary"`
	ResourceChanges []FilteredResourceChange `json:"resource_changes"`
}

type PlanSummary struct {
	Create  int `json:"create"`
	Update  int `json:"update"`
	Replace int `json:"replace"`
	Delete  int `json:"delete"`
}

type FilteredResourceChange struct {
	Address string                 `json:"address"`
	Type    string                 `json:"type"`
	Action  string                 `json:"action"`
	Changes map[string]interface{} `json:"changes,omitempty"`
}

func ParseAndFilter(rawJSON []byte) (*FilteredPlan, error) {
	var rawPlan Plan
	if err := json.Unmarshal(rawJSON, &rawPlan); err != nil {
		return nil, fmt.Errorf("failed to parse terraform JSON: %w", err)
	}

	filtered := &FilteredPlan{
		ResourceChanges: make([]FilteredResourceChange, 0),
	}

	for _, rc := range rawPlan.ResourceChanges {
		action := determineAction(rc.Change.Actions)

		// Rule 1: Skip no-op and read operations
		if action == "no-op" || action == "read" {
			continue
		}

		// Update global counts
		switch action {
		case "create":
			filtered.Summary.Create++
		case "update":
			filtered.Summary.Update++
		case "replace":
			filtered.Summary.Replace++
		case "delete":
			filtered.Summary.Delete++
		}

		filteredRC := FilteredResourceChange{
			Address: rc.Address,
			Type:    rc.Type,
			Action:  action,
		}

		// Rule 2 & 3: Extract & sanitize map differences based on action
		switch action {
		case "create":
			filteredRC.Changes = sanitizeMap(rc.Change.After, rc.Change.AfterSensitive)
		case "delete":
			filteredRC.Changes = sanitizeMap(rc.Change.Before, rc.Change.BeforeSensitive)
		case "update", "replace":
			before := sanitizeMap(rc.Change.Before, rc.Change.BeforeSensitive)
			after := sanitizeMap(rc.Change.After, rc.Change.AfterSensitive)
			filteredRC.Changes = computeMapDiff(before, after)
		}

		filtered.ResourceChanges = append(filtered.ResourceChanges, filteredRC)
	}

	return filtered, nil
}

// Determines standard action strings from Terraform's actions array
func determineAction(actions []string) string {
	if len(actions) == 0 {
		return "no-op"
	}
	if len(actions) == 1 {
		return actions[0] // "create", "update", "delete", "no-op", "read"
	}

	// Handles real Terraform replacement actions (both orderings)
	hasDelete := false
	hasCreate := false
	for _, act := range actions {
		if act == "delete" {
			hasDelete = true
		}
		if act == "create" {
			hasCreate = true
		}
	}

	if hasDelete && hasCreate {
		return "replace"
	}

	return actions[0]
}

func computeMapDiff(before, after map[string]interface{}) map[string]interface{} {
	diff := make(map[string]interface{})

	for k, afterVal := range after {
		beforeVal, exists := before[k]
		if !exists {
			diff[k] = map[string]interface{}{"old": nil, "new": afterVal}
			continue
		}
		if !reflect.DeepEqual(beforeVal, afterVal) {
			diff[k] = map[string]interface{}{"old": beforeVal, "new": afterVal}
		}
	}

	for k, beforeVal := range before {
		if _, exists := after[k]; !exists {
			diff[k] = map[string]interface{}{"old": beforeVal, "new": nil}
		}
	}

	return diff
}

func sanitizeMap(data map[string]interface{}, sensitive interface{}) map[string]interface{} {
	if data == nil {
		return nil
	}

	sanitized := make(map[string]interface{})
	sensitiveMap := extractSensitiveKeys(sensitive)

	for k, v := range data {
		if sensitiveMap[k] {
			sanitized[k] = "<SENSITIVE_REDACTED>"
		} else {
			sanitized[k] = v
		}
	}

	return sanitized
}

// Recursively processes Terraform's nested sensitivity maps
func extractSensitiveKeys(sensitive interface{}) map[string]bool {
	result := make(map[string]bool)
	if sensitive == nil {
		return result
	}

	if sMap, ok := sensitive.(map[string]interface{}); ok {
		for k, v := range sMap {
			if isBoolTrue(v) {
				result[k] = true
			}
		}
	}
	return result
}

func isBoolTrue(val interface{}) bool {
	if b, ok := val.(bool); ok {
		return b
	}
	return false
}