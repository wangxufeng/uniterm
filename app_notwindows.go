//go:build !windows

package main

import (
	"os"
	"os/exec"
	"strings"
)

func (a *App) findMainWindow() uintptr { return 0 }

func (a *App) subclassMainWindow() {}

func (a *App) unsubclassMainWindow() {}

func (a *App) GetAvailableShells() []string {
	var shells []string
	var seen = make(map[string]bool)

	add := func(path string) {
		if path == "" {
			return
		}
		abs, err := exec.LookPath(path)
		if err != nil {
			return
		}
		key := strings.ToLower(strings.ReplaceAll(abs, `\`, `/`))
		if seen[key] {
			return
		}
		seen[key] = true
		shells = append(shells, abs)
	}

	add(os.Getenv("SHELL"))
	add("bash")
	add("zsh")
	add("fish")
	add("sh")
	return shells
}
