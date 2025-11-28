package flatseek

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func flatpakQuery(output string) []Package {
	lines := strings.Split(output, "\n")
	result := []Package{}

	if len(lines) < 2 {
		return result
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		parts := strings.Split(line, "\t")
		if len(parts) < 5 {
			continue
		}

		result = append(result, Package{
			Name:        parts[0],
			Description: parts[1],
			AppID:       parts[2],
			Version:     parts[3],
			Branch:      parts[4],
			Remote:      parts[5],
		})
	}

	return result
}

func (ps *UI) pkgSearch(term string) ([]Package, []Package, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, "flatpak", "search", "--columns=all", term)

	out, err := cmd.Output()
	if err != nil {
		return nil, nil, fmt.Errorf("flatpak search failed: %w", err)
	}

	packages := flatpakQuery(string(out))
	installed := []Package{}

	return packages, installed, nil
}

// checks the local db if a package is installed
func (ps *UI) pkgCheckInstalled(pkg string) bool {
	return false
}

func (ps *UI) pkgGetSuggestion(text string) string {
	return "dihh"
}
