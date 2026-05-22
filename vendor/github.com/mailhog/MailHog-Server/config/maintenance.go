package config

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/mailhog/storage"
)

func parseMaintenancePolicy(cfg *Config) (storage.MaintenancePolicy, error) {
	interval, err := parseMaintenanceDuration(cfg.MaintenanceInterval)
	if err != nil {
		return storage.MaintenancePolicy{}, fmt.Errorf("maintenance interval: %s", err)
	}
	if interval == 0 {
		interval = time.Hour
	}

	deleteOlderThan, err := parseMaintenanceDuration(cfg.MaintenanceDeleteOlderThan)
	if err != nil {
		return storage.MaintenancePolicy{}, fmt.Errorf("maintenance delete older than: %s", err)
	}

	maxBytes, err := parseMaintenanceSize(cfg.MaintenanceMaxMaildirSize)
	if err != nil {
		return storage.MaintenancePolicy{}, fmt.Errorf("maintenance max maildir size: %s", err)
	}

	minFreeBytes, err := parseMaintenanceSize(cfg.MaintenanceMinFreeSpace)
	if err != nil {
		return storage.MaintenancePolicy{}, fmt.Errorf("maintenance min free space: %s", err)
	}

	if cfg.MaintenanceMaxMessages < 0 {
		return storage.MaintenancePolicy{}, fmt.Errorf("maintenance max messages must be positive")
	}

	return storage.MaintenancePolicy{
		Enabled:         cfg.MaintenanceEnabled,
		Interval:        interval,
		DeleteOlderThan: deleteOlderThan,
		MaxBytes:        maxBytes,
		MaxMessages:     cfg.MaintenanceMaxMessages,
		MinFreeBytes:    minFreeBytes,
	}, nil
}

func parseMaintenanceDuration(value string) (time.Duration, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return 0, nil
	}

	switch value {
	case "hour":
		value = "1h"
	case "day":
		value = "1d"
	case "week":
		value = "1w"
	}

	unit := value[len(value)-1:]
	switch unit {
	case "d", "w":
		amount, err := strconv.ParseFloat(strings.TrimSpace(value[:len(value)-1]), 64)
		if err != nil || amount <= 0 {
			return 0, fmt.Errorf("must be a positive duration like 1h, 24h, 1d, or 1w")
		}
		if unit == "d" {
			return time.Duration(amount * float64(24*time.Hour)), nil
		}
		return time.Duration(amount * float64(7*24*time.Hour)), nil
	default:
		duration, err := time.ParseDuration(value)
		if err != nil {
			return 0, fmt.Errorf("must be a duration like 15m, 1h, 24h, 1d, or 1w")
		}
		if duration <= 0 {
			return 0, fmt.Errorf("must be positive")
		}
		return duration, nil
	}
}

func parseMaintenanceSize(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}

	number, unit, err := splitMaintenanceSize(value)
	if err != nil {
		return 0, err
	}

	multiplier, ok := maintenanceSizeMultiplier(unit)
	if !ok {
		return 0, fmt.Errorf("must use B, KB, MB, GB, KiB, MiB, or GiB units")
	}

	amount, err := strconv.ParseFloat(number, 64)
	if err != nil || amount <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return 0, fmt.Errorf("must be positive")
	}

	size := amount * float64(multiplier)
	if size > float64(math.MaxInt64) {
		return 0, fmt.Errorf("is too large")
	}
	return int64(size), nil
}

func splitMaintenanceSize(value string) (string, string, error) {
	value = strings.TrimSpace(value)
	index := len(value)
	for index > 0 {
		ch := value[index-1]
		if (ch >= '0' && ch <= '9') || ch == '.' {
			break
		}
		index--
	}
	if index == 0 {
		return "", "", fmt.Errorf("must include a number")
	}
	number := strings.TrimSpace(value[:index])
	unit := strings.ToLower(strings.TrimSpace(value[index:]))
	if number == "" {
		return "", "", fmt.Errorf("must include a number")
	}
	return number, unit, nil
}

func maintenanceSizeMultiplier(unit string) (int64, bool) {
	switch unit {
	case "", "b":
		return 1, true
	case "k", "kb":
		return 1000, true
	case "m", "mb":
		return 1000 * 1000, true
	case "g", "gb":
		return 1000 * 1000 * 1000, true
	case "t", "tb":
		return 1000 * 1000 * 1000 * 1000, true
	case "ki", "kib":
		return 1024, true
	case "mi", "mib":
		return 1024 * 1024, true
	case "gi", "gib":
		return 1024 * 1024 * 1024, true
	case "ti", "tib":
		return 1024 * 1024 * 1024 * 1024, true
	default:
		return 0, false
	}
}
