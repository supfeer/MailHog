package config

import (
	"testing"
	"time"
)

func TestParseMaintenanceDurationSupportsHourDayWeek(t *testing.T) {
	duration, err := parseMaintenanceDuration("24h")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := duration, 24*time.Hour; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}

	duration, err = parseMaintenanceDuration("1d")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := duration, 24*time.Hour; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}

	duration, err = parseMaintenanceDuration("1w")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := duration, 7*24*time.Hour; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestParseMaintenanceDurationRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{"0h", "-1h", "later"} {
		if _, err := parseMaintenanceDuration(value); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}

func TestParseMaintenanceSizeSupportsDecimalAndBinaryUnits(t *testing.T) {
	size, err := parseMaintenanceSize("5GB")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := size, int64(5*1000*1000*1000); got != want {
		t.Fatalf("got %d, want %d", got, want)
	}

	size, err = parseMaintenanceSize("4Gi")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := size, int64(4*1024*1024*1024); got != want {
		t.Fatalf("got %d, want %d", got, want)
	}
}

func TestParseMaintenancePolicyBuildsStoragePolicy(t *testing.T) {
	policy, err := parseMaintenancePolicy(&Config{
		MaintenanceEnabled:         true,
		MaintenanceInterval:        "15m",
		MaintenanceDeleteOlderThan: "24h",
		MaintenanceMaxMaildirSize:  "4Gi",
		MaintenanceMaxMessages:     5000,
		MaintenanceMinFreeSpace:    "1Gi",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !policy.Enabled || !policy.Active() {
		t.Fatal("expected active maintenance policy")
	}
	if got, want := policy.Interval, 15*time.Minute; got != want {
		t.Fatalf("got interval %s, want %s", got, want)
	}
	if got, want := policy.DeleteOlderThan, 24*time.Hour; got != want {
		t.Fatalf("got delete age %s, want %s", got, want)
	}
	if got, want := policy.MaxBytes, int64(4*1024*1024*1024); got != want {
		t.Fatalf("got max bytes %d, want %d", got, want)
	}
	if got, want := policy.MaxMessages, 5000; got != want {
		t.Fatalf("got max messages %d, want %d", got, want)
	}
	if got, want := policy.MinFreeBytes, int64(1024*1024*1024); got != want {
		t.Fatalf("got min free bytes %d, want %d", got, want)
	}
}
