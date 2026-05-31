package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/CameronBrooks11/super-productivity-local-gobridge/internal/bridge"
	"github.com/CameronBrooks11/super-productivity-local-gobridge/internal/version"
)

// Run executes the doctor diagnostics. Returns exit code.
func Run(args []string) int {
	baseURL := bridge.DefaultBaseURL
	if env := os.Getenv("SP_BASE_URL"); env != "" {
		baseURL = env
	}

	fmt.Printf("sp-local-bridge doctor (%s)\n", version.Version)
	fmt.Println("─────────────────────────────────────")

	fmt.Printf("Target: %s\n", baseURL)
	fmt.Println()

	client := bridge.NewClient(baseURL)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Check health
	fmt.Print("Health check... ")
	health := client.Health(ctx)
	if health.OK {
		fmt.Println("OK")
	} else {
		fmt.Printf("FAILED: %s\n", health.Error.Message)
		return 1
	}

	// Check status
	fmt.Print("Status check... ")
	status := client.Status(ctx)
	if status.OK {
		fmt.Println("OK")
		data, _ := json.MarshalIndent(status.Data, "  ", "  ")
		fmt.Printf("  %s\n", data)
	} else {
		fmt.Printf("FAILED: %s\n", status.Error.Message)
		return 1
	}

	fmt.Println()
	fmt.Println("All checks passed.")
	return 0
}
