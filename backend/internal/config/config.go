package config

import "os"

type Config struct {
	DatabaseURL      string
	SupabaseJWTSecret string
	Port             string
}

func Load() Config {
	return Config{
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		SupabaseJWTSecret: os.Getenv("SUPABASE_JWT_SECRET"),
		Port:              os.Getenv("PORT"),
	}
}
