package main

import (
	"context"
	"fmt"
	"os"

	"github.com/EyupEfeDuvarbasi/promptpatch/internal/cli"
	"github.com/EyupEfeDuvarbasi/promptpatch/internal/config"
	"github.com/EyupEfeDuvarbasi/promptpatch/internal/editor"
	"github.com/EyupEfeDuvarbasi/promptpatch/internal/llm"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "setup-codex" {
		if err := editor.SetupCodex(); err != nil {
			fail(err)
		}
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "edit" {
		if err := editor.Run(os.Args[2]); err != nil {
			fail(err)
		}
		return
	}

	var client *llm.Client
	if usesLLM(os.Args[1:]) {
		path, err := config.DefaultPath()
		if err != nil {
			fail(err)
		}
		resolved, err := config.Resolve(path, os.Stdin, os.Stdout)
		if err != nil {
			fail(err)
		}
		client = &resolved
	}
	if err := cli.Run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, client); err != nil {
		fail(err)
	}
}

func usesLLM(args []string) bool {
	for _, arg := range args {
		if arg == "--model" || len(arg) > len("--model=") && arg[:len("--model=")] == "--model=" {
			return true
		}
	}
	return false
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "promptcheck:", err)
	os.Exit(1)
}
