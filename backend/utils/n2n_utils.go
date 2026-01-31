package utils

import (
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
)

// GenerateRandomMac generates a random locally administered MAC address
func GenerateRandomMac() (string, error) {
	buf := make([]byte, 6)
	_, err := rand.Read(buf)
	if err != nil {
		return "", err
	}
	// Set the local bit and clear the multicast bit
	buf[0] = (buf[0] | 2) & 0xfe
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", buf[0], buf[1], buf[2], buf[3], buf[4], buf[5]), nil
}

// NextIP calculates the next IP address
func NextIP(ip net.IP) net.IP {
	next := make(net.IP, len(ip))
	copy(next, ip)
	for j := len(next) - 1; j >= 0; j-- {
		next[j]++
		if next[j] > 0 {
			break
		}
	}
	return next
}

// CompareIP compares two IP addresses numerically
// Returns: -1 if a < b, 0 if a == b, 1 if a > b
func CompareIP(a, b net.IP) int {
	a = a.To16()
	b = b.To16()
	for i := 0; i < len(a); i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

// WriteCommunityList writes a list of communities to a file
func WriteCommunityList(filePath string, communities []string) error {
	content := strings.Join(communities, "\n")
	if content != "" {
		content += "\n"
	}
	return os.WriteFile(filePath, []byte(content), 0644)
}

// ReadSupernodeConfig reads n2n supernode config file into a map
func ReadSupernodeConfig(filePath string) (map[string]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	
	config := make(map[string]string)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			key := strings.TrimPrefix(parts[0], "-")
			config[key] = parts[1]
		} else if strings.HasPrefix(line, "-") {
			// Handle flags without values, e.g., -f
			key := strings.TrimPrefix(line, "-")
			config[key] = ""
		}
	}
	return config, nil
}

// WriteSupernodeConfig writes map back to n2n supernode config file
func WriteSupernodeConfig(filePath string, config map[string]string) error {
	var lines []string
	for k, v := range config {
		if v == "" {
			lines = append(lines, fmt.Sprintf("-%s", k))
		} else {
			lines = append(lines, fmt.Sprintf("-%s=%s", k, v))
		}
	}
	
	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(filePath, []byte(content), 0644)
}

// RunCommand executes a system command
func RunCommand(name string, arg ...string) (string, error) {
	cmd := exec.Command(name, arg...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}