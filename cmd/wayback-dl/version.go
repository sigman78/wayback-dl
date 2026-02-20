package main

// Populated at build time via -ldflags:
//
//	go build -ldflags "-X main.version=v1.0.0 -X main.commit=abc1234 -X main.date=2025-01-01"
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)
