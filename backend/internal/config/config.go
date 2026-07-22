// Package config loads all runtime settings from environment variables.
// FRONTEND_ORIGINS is a comma-separated list; TZ falls back to UTC if unset
// or unparseable. Everything else is taken verbatim, empty if not provided.
package config

import (
	"os"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL       string
	SupabaseURL       string
	Port              string
	FrontendOrigins   []string
	Location          *time.Location
	ResendAPIKey      string
	ReminderFromEmail string
	ReminderToEmail   string
}

func Load() Config {
	var origins []string
	for _, o := range strings.Split(os.Getenv("FRONTEND_ORIGINS"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			origins = append(origins, o)
		}
	}

	loc, err := time.LoadLocation(os.Getenv("TZ"))
	if err != nil {
		loc = time.UTC
	}

	return Config{
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		SupabaseURL:       os.Getenv("SUPABASE_URL"),
		Port:              os.Getenv("PORT"),
		FrontendOrigins:   origins,
		Location:          loc,
		ResendAPIKey:      os.Getenv("RESEND_API_KEY"),
		ReminderFromEmail: os.Getenv("REMINDER_FROM_EMAIL"),
		ReminderToEmail:   os.Getenv("REMINDER_TO_EMAIL"),
	}
}
