package config

import "os"

func openToken(path string) (*os.File, error) { return os.Open(path) }
