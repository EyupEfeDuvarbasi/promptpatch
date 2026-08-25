package main

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/EyupEfeDuvarbasi/promptpatch/internal/cli"
	"github.com/EyupEfeDuvarbasi/promptpatch/internal/config"
	"github.com/EyupEfeDuvarbasi/promptpatch/internal/editor"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "setup-codex" {
		input := bufio.NewReader(os.Stdin)
		if err := editor.SetupCodex(); err != nil {
			fail(err)
		}
		path, err := config.DefaultPath()
		if err != nil {
			fail(err)
		}
		if _, err := config.ConfigureChatContext(path, input, os.Stdout); err != nil {
			fail(err)
		}
		return
	}
	if len(os.Args) == 2 && os.Args[1] == "configure-context" {
		path, err := config.DefaultPath()
		if err != nil {
			fail(err)
		}
		if _, err := config.ConfigureChatContextAgain(path, os.Stdin, os.Stdout); err != nil {
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
	if err := cli.Run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, nil); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "promptcheck:", err)
	os.Exit(1)
}
