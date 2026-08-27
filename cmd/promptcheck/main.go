package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/EyupEfeDuvarbasi/promptpatch/internal/cli"
	"github.com/EyupEfeDuvarbasi/promptpatch/internal/config"
	"github.com/EyupEfeDuvarbasi/promptpatch/internal/editor"
	"github.com/EyupEfeDuvarbasi/promptpatch/internal/server"
)

var version = "dev"

func main() {
	if len(os.Args) == 2 && os.Args[1] == "version" {
		commit := "unknown"
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, setting := range info.Settings {
				if setting.Key == "vcs.revision" && len(setting.Value) >= 7 {
					commit = setting.Value[:7]
				}
			}
		}
		fmt.Printf("Prompter %s (%s)\n", version, commit)
		return
	}
	if len(os.Args) >= 2 && (os.Args[1] == "data" || os.Args[1] == "doctor" || os.Args[1] == "support-bundle" || os.Args[1] == "uninstall") {
		if err := localCommand(os.Args[1:], os.Stdin, os.Stdout); err != nil {
			fail(err)
		}
		return
	}
	if len(os.Args) == 2 && os.Args[1] == "serve" {
		config, err := server.FromEnv(os.Getenv)
		if err != nil {
			fail(err)
		}
		fmt.Printf("Prompter web arayüzü http://%s adresinde çalışıyor\n", config.Addr)
		if err := server.New(config).ListenAndServe(); err != nil {
			fail(err)
		}
		return
	}
	if len(os.Args) == 2 && os.Args[1] == "start" {
		if err := editor.SetupCodex(); err != nil {
			fail(err)
		}
		config, err := server.FromEnv(os.Getenv)
		if err != nil {
			fail(err)
		}
		go openPrompterWhenReady(config.Addr, os.Getenv("PROMPTER_WEB_URL"))
		if err := server.New(config).ListenAndServe(); err != nil {
			fail(err)
		}
		return
	}
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
