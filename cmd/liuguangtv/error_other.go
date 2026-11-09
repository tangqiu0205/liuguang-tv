//go:build !windows

package main

import "log"

func reportStartupError(err error) { log.Print(err) }
