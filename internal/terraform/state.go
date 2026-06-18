package terraform

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Resource represents a Terraform-managed resource.
type Resource struct {
	Address string `json:"address"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Mode    string `json:"mode"`
}

// Inventory holds parsed Terraform state resources.
type Inventory struct {
	StatePath string     `json:"state_path"`
	Resources []Resource `json:"resources"`
	Total     int        `json:"total"`
}

type stateFile struct {
	Resources []stateResource `json:"resources"`
}

type stateResource struct {
	Mode      string `json:"mode"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	Instances []any  `json:"instances"`
}

// ParseState reads a Terraform JSON state file.
func ParseState(path string) (*Inventory, error) {
	if path == "" {
		return nil, fmt.Errorf("terraform state path not configured")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read terraform state: %w", err)
	}

	var state stateFile
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse terraform state: %w", err)
	}

	inv := &Inventory{StatePath: path}
	for _, r := range state.Resources {
		if len(r.Instances) == 0 {
			continue
		}
		addr := fmt.Sprintf("%s.%s", r.Type, r.Name)
		if r.Mode != "" && r.Mode != "managed" {
			addr = fmt.Sprintf("%s.%s", r.Mode, addr)
		}
		inv.Resources = append(inv.Resources, Resource{
			Address: addr,
			Type:    r.Type,
			Name:    r.Name,
			Mode:    r.Mode,
		})
	}
	inv.Total = len(inv.Resources)
	return inv, nil
}

func (inv *Inventory) ToJSON() (string, error) {
	data, err := json.MarshalIndent(inv, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (inv *Inventory) Summary() string {
	counts := map[string]int{}
	for _, r := range inv.Resources {
		counts[r.Type]++
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Terraform Resources (%d total):\n\n", inv.Total)
	for t, n := range counts {
		fmt.Fprintf(&b, "- %s: %d\n", t, n)
	}
	if inv.Total > 0 {
		b.WriteString("\nSample resources:\n")
		limit := inv.Total
		if limit > 10 {
			limit = 10
		}
		for i := 0; i < limit; i++ {
			fmt.Fprintf(&b, "- %s\n", inv.Resources[i].Address)
		}
	}
	return b.String()
}
