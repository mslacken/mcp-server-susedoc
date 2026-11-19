package main

import (
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// loadTLS builds a *tls.Config from the certificate and private key .pem files
// in dir. The files may have any names: one containing a certificate
// (-----BEGIN CERTIFICATE-----) and one containing a private key
// (-----BEGIN ... PRIVATE KEY-----) are required.
func loadTLS(dir string) (*tls.Config, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading tls directory: %w", err)
	}
	var certFile, keyFile string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		s := string(data)
		switch {
		case strings.Contains(s, "BEGIN CERTIFICATE"):
			if certFile == "" {
				certFile = e.Name()
			}
		case strings.Contains(s, "PRIVATE KEY"):
			if keyFile == "" {
				keyFile = e.Name()
			}
		}
	}
	if certFile == "" {
		return nil, fmt.Errorf("no certificate .pem file found in %s", dir)
	}
	if keyFile == "" {
		return nil, fmt.Errorf("no private key .pem file found in %s", dir)
	}
	cert, err := tls.LoadX509KeyPair(filepath.Join(dir, certFile), filepath.Join(dir, keyFile))
	if err != nil {
		return nil, fmt.Errorf("loading tls certificate and key from %s: %w", dir, err)
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}}, nil
}
