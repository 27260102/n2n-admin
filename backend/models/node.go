package models

import (
	"time"

	"gorm.io/gorm"
)

type Node struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"size:100;not null" json:"name"`
	IPAddress   string         `gorm:"size:45;uniqueIndex" json:"ip_address"`
	MacAddress  string         `gorm:"size:17;uniqueIndex" json:"mac_address"`
	Community   string         `gorm:"size:50;index" json:"community"`
	Description string         `json:"description"`
	Encryption  string         `gorm:"default:AES" json:"encryption"` // AES, Twofish, ChaCha20
	Compression bool           `gorm:"default:false" json:"compression"`
	Routing     string         `json:"routing"`    // e.g., 192.168.1.0/24:10.10.10.5
	LocalPort   int            `json:"local_port"` // -p parameter
	IsEnabled   bool           `gorm:"default:true" json:"is_enabled"`
	LastSeen    *time.Time     `json:"last_seen"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

type Community struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	Name      string `gorm:"size:50;uniqueIndex" json:"name"`
	Range     string `gorm:"size:50" json:"range"` // e.g., 10.0.0.0/24
	Password  string `json:"password"`
	CreatedAt time.Time
}

type Setting struct {
	Key   string `gorm:"primaryKey"`
	Value string
}

type User struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	Username string `gorm:"size:100;uniqueIndex" json:"username"`
	Password string `json:"-"` // 不在 JSON 中返回
	IsAdmin  bool   `gorm:"default:true" json:"is_admin"`
}
