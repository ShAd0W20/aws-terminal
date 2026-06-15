package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"aws-terminal/internal/app"
)

var version = "dev"

func main() {
	if handled, err := handleCLI(os.Args[1:]); handled {
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	if err := app.Run(version); err != nil {
		log.Fatal(err)
	}
}

func handleCLI(args []string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}

	switch args[0] {
	case "version", "--version", "-v":
		fmt.Printf("aws-terminal %s\n", version)
		return true, nil
	case "check-update":
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		result, err := app.NewUpdateService(version).Check(ctx)
		if err != nil {
			return true, err
		}
		if result.DevelopmentBuild {
			fmt.Printf("Development build; latest release is %s\n", result.LatestVersion)
			return true, nil
		}
		if result.UpdateAvailable {
			fmt.Printf("Update available: %s (current %s)\n", result.LatestVersion, result.CurrentVersion)
			return true, nil
		}
		fmt.Printf("aws-terminal is up to date (%s)\n", result.CurrentVersion)
		return true, nil
	case "update":
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		result, err := app.NewUpdateService(version).InstallLatest(ctx)
		if err != nil {
			return true, err
		}
		if result.Updated {
			fmt.Printf("Updated aws-terminal from %s to %s. Restart the app to use the new version.\n", result.CurrentVersion, result.LatestVersion)
			return true, nil
		}
		if result.SelfUpdatable {
			fmt.Printf("aws-terminal is up to date (%s)\n", result.LatestVersion)
			return true, nil
		}
		fmt.Println(result.Instructions)
		return true, nil
	default:
		return false, nil
	}
}
