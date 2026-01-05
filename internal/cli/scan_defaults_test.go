package cli

import (
	"repomedic/internal/config"
	"repomedic/internal/flags"
	"testing"

	"github.com/spf13/cobra"
)

func TestApplyImplicitDefaults_UserScan_DefaultsToIncludeForks(t *testing.T) {
	cfg := config.New()
	cfg.Targeting.User = "test-user"
	cfg.Targeting.Forks = "exclude"

	cmd := &cobra.Command{Use: "scan"}
	cmd.Flags().String(flags.FlagForks, "exclude", "")

	applyImplicitDefaults(cmd, cfg)

	if cfg.Targeting.Forks != "include" {
		t.Fatalf("expected forks default to be include for --user; got %q", cfg.Targeting.Forks)
	}
}

func TestApplyImplicitDefaults_UserScan_DoesNotOverrideExplicitForksFlag(t *testing.T) {
	cfg := config.New()
	cfg.Targeting.User = "test-user"
	cfg.Targeting.Forks = "exclude"

	cmd := &cobra.Command{Use: "scan"}
	cmd.Flags().String(flags.FlagForks, "exclude", "")
	if err := cmd.Flags().Set(flags.FlagForks, "exclude"); err != nil {
		t.Fatalf("failed to set forks flag: %v", err)
	}

	applyImplicitDefaults(cmd, cfg)

	if cfg.Targeting.Forks != "exclude" {
		t.Fatalf("expected forks to remain exclude when --forks explicitly set; got %q", cfg.Targeting.Forks)
	}
}
