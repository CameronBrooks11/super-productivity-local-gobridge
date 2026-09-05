package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/CameronBrooks11/super-productivity-local-gobridge/internal/bridge"
	"github.com/CameronBrooks11/super-productivity-local-gobridge/internal/cli"
	"github.com/CameronBrooks11/super-productivity-local-gobridge/internal/doctor"
	"github.com/CameronBrooks11/super-productivity-local-gobridge/internal/hostcfg"
	"github.com/CameronBrooks11/super-productivity-local-gobridge/internal/mcpadapter"
	"github.com/CameronBrooks11/super-productivity-local-gobridge/internal/version"
)

func main() {
	// Multicall dispatch based on argv[0]
	base := filepath.Base(os.Args[0])
	base = strings.TrimSuffix(base, ".exe")

	switch base {
	case "sp-local-bridge-mcp":
		runMCP()
		return
	case "sp-local-bridge-doctor":
		runDoctor()
		return
	case "sp-local-bridge-print-config":
		runPrintConfig()
		return
	case "sp-local-bridge-configure":
		runConfigure()
		return
	}

	// Subcommand dispatch
	if len(os.Args) < 2 {
		cli.Usage()
		os.Exit(0)
	}

	switch os.Args[1] {
	case "--version":
		fmt.Println(version.String())
		os.Exit(0)
	case "--help", "-h":
		cli.Usage()
		os.Exit(0)
	case "mcp":
		runMCP()
	case "doctor":
		// Strip the subcommand word here, as print-config and configure do, so
		// doctor.Run never has to guess whether "doctor" is its own name or a
		// stray argument. The multicall alias path passes os.Args[1:] instead.
		os.Exit(doctor.Run(os.Args[2:]))
	case "print-config":
		os.Exit(hostcfg.RunPrintConfig(os.Args[2:]))
	case "configure":
		os.Exit(hostcfg.RunConfigure(os.Args[2:]))
	default:
		code := cli.Run(os.Args[1:])
		os.Exit(code)
	}
}

func runMCP() {
	baseURL := bridge.DefaultBaseURL
	if env := os.Getenv("SP_BASE_URL"); env != "" {
		baseURL = env
	}
	if err := mcpadapter.Serve(baseURL); err != nil {
		fmt.Fprintf(os.Stderr, "sp-local-bridge-mcp: %v\n", err)
		os.Exit(1)
	}
}

func runDoctor() {
	code := doctor.Run(os.Args[1:])
	os.Exit(code)
}

func runPrintConfig() {
	code := hostcfg.RunPrintConfig(os.Args[1:])
	os.Exit(code)
}

func runConfigure() {
	code := hostcfg.RunConfigure(os.Args[1:])
	os.Exit(code)
}
