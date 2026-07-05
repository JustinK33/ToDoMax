package config

import (
	"os"
	"strings"
)

type Config struct {
	DatabaseURL     string
	SupabaseURL     string
	Port            string
	FrontendOrigins []string
}

func Load() Config {
	var origins []string
	for _, o := range strings.Split(os.Getenv("FRONTEND_ORIGINS"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			origins = append(origins, o)
		}
	}

	return Config{
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		SupabaseURL:     os.Getenv("SUPABASE_URL"),
		Port:            os.Getenv("PORT"),
		FrontendOrigins: origins,
	}
}
