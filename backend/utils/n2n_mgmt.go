package utils

import (
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
)

type MgmtClient struct {
	Addr     string
	Password string
}

type EdgeInfo struct {
	Mac      string `json:"mac"`
	Internal string `json:"internal"`
	External string `json:"external"`
	LastSeen int    `json:"last_seen"`
}

func (m *MgmtClient) Query(command string) (string, error) {
	conn, err := net.DialTimeout("udp", m.Addr, 1*time.Second)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	_, err = conn.Write([]byte(command))
	if err != nil {
		return "", err
	}

	var fullResp strings.Builder
	buffer := make([]byte, 8192)
	conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	
	for {
		n, err := conn.Read(buffer)
		if n > 0 {
			fullResp.Write(buffer[:n])
			conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
		}
		if err != nil {
			break
		}
	}
	return fullResp.String(), nil
}

func (m *MgmtClient) GetOnlineMacs() (map[string]int, error) {
	edges, err := m.GetEdgeInfo()
	if err != nil {
		return nil, err
	}
	res := make(map[string]int)
	for mac, info := range edges {
		res[mac] = info.LastSeen
	}
	return res, nil
}

func (m *MgmtClient) GetEdgeInfo() (map[string]EdgeInfo, error) {
	resp, err := m.Query("edges")
	if err != nil {
		return nil, err
	}

	reMac := regexp.MustCompile(`([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})`)
	lines := strings.Split(resp, "\n")
	onlineEdges := make(map[string]EdgeInfo)

	isSupernodeSection := false

	for _, line := range lines {
		if strings.Contains(line, "SUPERNODES") {
			isSupernodeSection = true
			continue
		}
		if strings.Contains(line, "SUPERNODE FORWARD") || strings.Contains(line, "FEDERATION") {
			isSupernodeSection = false
			continue
		}
		if isSupernodeSection {
			continue
		}

		mac := reMac.FindString(line)
		if mac != "" {
			cleanMac := strings.ToUpper(strings.ReplaceAll(mac, ":", ""))
			fields := strings.Split(line, "|")
			if len(fields) >= 5 {
				internal := strings.TrimSpace(fields[1])
				external := strings.TrimSpace(fields[3])
				lastSeen := 0
				fmt.Sscanf(strings.TrimSpace(fields[len(fields)-1]), "%d", &lastSeen)
				
				onlineEdges[cleanMac] = EdgeInfo{
					Mac:      cleanMac,
					Internal: internal,
					External: external,
					LastSeen: lastSeen,
				}
			}
		}
	}
	return onlineEdges, nil
}