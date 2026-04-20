package server

import "os"

func readSourceFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
