package main

import (
	"bufio"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"n2n_ui/backend/models"
	"n2n_ui/backend/utils"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

//go:embed dist/*
var content embed.FS

var db *gorm.DB
var n2nMgmt *utils.MgmtClient
var jwtSecret = []byte("n2n_admin_default_secret_32_chars")

func setupSecret() {
	if s := os.Getenv("N2N_ADMIN_SECRET"); s != "" {
		jwtSecret = []byte(s)
	}
}

type IPLocation struct {
	Country string `json:"country"`
	City    string `json:"city"`
	ISP     string `json:"isp"`
}

var (
	relayMap   = make(map[string]*RelayEvent)
	relayMutex sync.Mutex
	ipCache    = make(map[string]IPLocation)
	cacheMutex sync.Mutex
)

type RelayEvent struct {
	SrcMac     string    `json:"src_mac"`
	DstMac     string    `json:"dst_mac"`
	LastActive time.Time `json:"last_active"`
	PktCount   int64     `json:"pkt_count"`
}

func initDB() {
	var err error
	db, err = gorm.Open(sqlite.Open("n2n_admin.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect database")
	}
	db.AutoMigrate(&models.Node{}, &models.Community{}, &models.Setting{}, &models.User{})
	var userCount int64
	db.Model(&models.User{}).Count(&userCount)
	if userCount == 0 {
		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		db.Create(&models.User{Username: "admin", Password: string(hashedPassword), IsAdmin: true})
	}
}

func getIPLocation(ip string) IPLocation {
	if ip == "" || strings.HasPrefix(ip, "127.") || strings.HasPrefix(ip, "192.168.") {
		return IPLocation{Country: "本地网络", City: "-", ISP: "-"}
	}
	
	cacheMutex.Lock()
	if loc, ok := ipCache[ip]; ok {
		cacheMutex.Unlock()
		return loc
	}
	cacheMutex.Unlock()

	url := fmt.Sprintf("http://ip-api.com/json/%s?lang=zh-CN", ip)
	resp, err := http.Get(url)
	if err != nil {
		return IPLocation{Country: "未知", City: "查询失败", ISP: "-"}
	}
	defer resp.Body.Close()

	var result struct {
		Country string `json:"country"`
		City    string `json:"city"`
		ISP     string `json:"isp"`
		Status  string `json:"status"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Status != "success" {
		return IPLocation{Country: "未知", City: "-", ISP: "-"}
	}

	loc := IPLocation{Country: result.Country, City: result.City, ISP: result.ISP}
	
	cacheMutex.Lock()
	ipCache[ip] = loc
	cacheMutex.Unlock()
	
	return loc
}

func startLogAnalyzer() {
	cmd := exec.Command("journalctl", "-u", "supernode", "-f", "-n", "0")
	stdout, _ := cmd.StdoutPipe()
	cmd.Start()
	re := regexp.MustCompile(`forwarding packet.*from ([0-9A-Fa-f:]{17}) to ([0-9A-Fa-f:]{17})`)
	reader := bufio.NewReader(stdout)
	for {
		line, _ := reader.ReadString('\n')
		matches := re.FindStringSubmatch(line)
		if len(matches) == 3 {
			src := strings.ToUpper(strings.ReplaceAll(matches[1], ":", ""))
			dst := strings.ToUpper(strings.ReplaceAll(matches[2], ":", ""))
			key := src + ">" + dst
			relayMutex.Lock()
			if ev, ok := relayMap[key]; ok {
				ev.LastActive = time.Now()
				ev.PktCount++
			} else {
				relayMap[key] = &RelayEvent{SrcMac: src, DstMac: dst, LastActive: time.Now(), PktCount: 1}
			}
			relayMutex.Unlock()
		}
	}
}

func jwtMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := c.GetHeader("Authorization")
		if strings.HasPrefix(tokenString, "Bearer ") {
			tokenString = strings.TrimPrefix(tokenString, "Bearer ")
		} else {
			tokenString = c.Query("token")
		}
		if tokenString == "" {
			c.JSON(401, gin.H{"error": "Unauthorized"})
			c.Abort(); return
		}
		token, _ := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) { return jwtSecret, nil })
		if token == nil || !token.Valid {
			c.JSON(401, gin.H{"error": "Invalid token"})
			c.Abort(); return
		}
		claims, _ := token.Claims.(jwt.MapClaims)
		c.Set("username", claims["username"])
		c.Next()
	}
}

func main() {
	setupSecret()
	initDB()
	go startLogAnalyzer()
	n2nMgmt = &utils.MgmtClient{Addr: "127.0.0.1:56440"}
	r := gin.New()
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	r.Use(cors.New(config))

	api := r.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
		api.POST("/login", login)
		protected := api.Group("/")
		protected.Use(jwtMiddleware())
		{
			protected.GET("/nodes", getNodes)
			protected.POST("/nodes", createNode)
			protected.DELETE("/nodes/:id", deleteNode)
			protected.GET("/nodes/:id/config", getNodeConfig)
			protected.GET("/stats", getStats)
			protected.GET("/communities", getCommunities)
			protected.POST("/communities", createCommunity)
			protected.DELETE("/communities/:id", deleteCommunity)
			protected.GET("/settings", getSettings)
			protected.POST("/settings", saveSettings)
			protected.GET("/supernode/config", getSupernodeConfig)
			protected.POST("/supernode/config", saveSupernodeConfig)
			protected.POST("/supernode/restart", restartSupernode)
			protected.POST("/tools/exec", execTool)
			protected.GET("/topology", getTopology)
			protected.GET("/supernode/logs", streamLogs)
			protected.GET("/relays", getActiveRelays)
			protected.POST("/change-password", changePassword)
		}
	}

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/") {
			c.JSON(404, gin.H{"error": "Not Found"})
			return
		}
		targetPath := strings.TrimPrefix(path, "/")
		if targetPath == "" { targetPath = "index.html" }
		fileBytes, err := content.ReadFile("dist/" + targetPath)
		if err != nil {
			index, _ := content.ReadFile("dist/index.html")
			c.Data(200, "text/html; charset=utf-8", index)
			return
		}
		contentType := "text/plain"
		if strings.HasSuffix(targetPath, ".html") { contentType = "text/html; charset=utf-8"
		} else if strings.HasSuffix(targetPath, ".js") { contentType = "application/javascript"
		} else if strings.HasSuffix(targetPath, ".css") { contentType = "text/css"
		} else if strings.HasSuffix(targetPath, ".svg") { contentType = "image/svg+xml"
		} else if strings.HasSuffix(targetPath, ".png") { contentType = "image/png" }
		c.Data(200, contentType, fileBytes)
	})

	log.Println("Server starting on :8080")
	r.Run(":8080")
}

func getNodes(c *gin.Context) {
	var nodes []models.Node
	db.Find(&nodes)
	edges, _ := n2nMgmt.GetEdgeInfo()
	res := make([]interface{}, 0)
	for _, n := range nodes {
		m := strings.ToUpper(strings.ReplaceAll(n.MacAddress, ":", ""))
		info, online := edges[m]
		var publicIP, locationStr string
		if online {
			publicIP = strings.Split(info.External, ":")[0]
			loc := getIPLocation(publicIP)
			locationStr = fmt.Sprintf("%s %s (%s)", loc.Country, loc.City, loc.ISP)
		}
		res = append(res, gin.H{
			"id": n.ID, "name": n.Name, "ip_address": n.IPAddress, "mac_address": n.MacAddress, 
			"community": n.Community, "is_online": online, "is_mapped": true,
			"external_ip": publicIP, "location": locationStr, "last_seen": info.LastSeen,
		})
	}
	c.JSON(200, res)
}

func login(c *gin.Context) {
	var p struct { U string `json:"username"`; P string `json:"password"` }
	c.ShouldBindJSON(&p)
	var user models.User
	if err := db.Where("username = ?", p.U).First(&user).Error; err != nil {
		c.JSON(401, gin.H{"error": "User not found"}); return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(p.P)); err != nil {
		c.JSON(401, gin.H{"error": "Wrong password"}); return
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"username": user.Username, "exp": time.Now().Add(time.Hour * 24).Unix()})
	t, _ := token.SignedString(jwtSecret)
	c.JSON(200, gin.H{"token": t, "user": user})
}

func changePassword(c *gin.Context) {
	var p struct { Old string `json:"old_password"`; New string `json:"new_password"` }
	c.ShouldBindJSON(&p)
	u, _ := c.Get("username")
	var user models.User
	db.Where("username = ?", u).First(&user)
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(p.Old)); err != nil {
		c.JSON(401, gin.H{"error": "Old password incorrect"}); return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(p.New), bcrypt.DefaultCost)
	db.Model(&user).Update("password", string(hash))
	c.JSON(200, gin.H{"message": "success"})
}

func getStats(c *gin.Context) {
	var n, cm int64
	db.Model(&models.Node{}).Count(&n)
	db.Model(&models.Community{}).Count(&cm)
	macs, _ := n2nMgmt.GetOnlineMacs()
	c.JSON(200, gin.H{"node_count": n, "community_count": cm, "online_count": len(macs)})
}

func createNode(c *gin.Context) {
	var n models.Node
	c.ShouldBindJSON(&n)
	n.MacAddress = strings.ToUpper(strings.ReplaceAll(n.MacAddress, ":", ""))
	db.Create(&n)
	c.JSON(200, n)
}

func deleteNode(c *gin.Context) {
	db.Delete(&models.Node{}, c.Param("id"))
	c.JSON(200, gin.H{"message": "deleted"})
}

func getNodeConfig(c *gin.Context) {
	var n models.Node
	db.First(&n, c.Param("id"))
	var s models.Setting
	db.Where("key = ?", "supernode_host").First(&s)
	p := utils.ConfigParams{Name: n.Name, IP: n.IPAddress, Community: n.Community, Password: "mima", Supernode: s.Value, Mac: n.MacAddress}
	c.JSON(200, gin.H{"conf": utils.GenerateConfFile(p)})
}

func getCommunities(c *gin.Context) {
	var comms []models.Community
	db.Find(&comms)
	c.JSON(200, comms)
}

func createCommunity(c *gin.Context) {
	var cm models.Community
	c.ShouldBindJSON(&cm)
	db.Create(&cm)
	c.JSON(200, cm)
}

func deleteCommunity(c *gin.Context) {
	db.Delete(&models.Community{}, c.Param("id"))
	c.JSON(200, gin.H{"message": "deleted"})
}

func getSettings(c *gin.Context) {
	var s []models.Setting
	db.Find(&s)
	res := make(map[string]string)
	for _, x := range s { res[x.Key] = x.Value }
	c.JSON(200, res)
}

func saveSettings(c *gin.Context) {
	var p map[string]string
	c.ShouldBindJSON(&p)
	for k, v := range p {
		db.Where("key = ?", k).Assign(models.Setting{Value: v}).FirstOrCreate(&models.Setting{Key: k})
	}
	c.JSON(200, gin.H{"message": "saved"})
}

func getSupernodeConfig(c *gin.Context) {
	cfg, _ := utils.ReadSupernodeConfig("/etc/n2n/supernode.conf")
	c.JSON(200, cfg)
}

func saveSupernodeConfig(c *gin.Context) {
	var n map[string]string
	c.ShouldBindJSON(&n)
	curr, _ := utils.ReadSupernodeConfig("/etc/n2n/supernode.conf")
	if curr == nil { curr = make(map[string]string) }
	for k, v := range n { curr[k] = v }
	curr["f"] = ""; curr["v"] = ""
	utils.WriteSupernodeConfig("/etc/n2n/supernode.conf", curr)
	c.JSON(200, gin.H{"message": "saved"})
}

func restartSupernode(c *gin.Context) {
	utils.RunCommand("systemctl", "restart", "supernode")
	c.JSON(200, gin.H{"message": "restarted"})
}

func execTool(c *gin.Context) {
	var p struct { Command string `json:"command"`; Target string `json:"target"` }
	c.ShouldBindJSON(&p)
	var out string
	if p.Command == "ping" {
		out, _ = utils.RunCommand("ping", "-c", "4", "-W", "2", p.Target)
	} else if p.Command == "traceroute" {
		out, _ = utils.RunCommand("traceroute", "-m", "10", "-n", p.Target)
	}
	c.JSON(200, gin.H{"output": out})
}

func getTopology(c *gin.Context) {
	var nodes []models.Node
	db.Find(&nodes)
	macs, _ := n2nMgmt.GetOnlineMacs()
	vNodes := []interface{}{gin.H{"id": "supernode", "label": "Supernode", "group": "supernode"}}
	vEdges := []interface{}{}
	for _, n := range nodes {
		m := strings.ToUpper(strings.ReplaceAll(n.MacAddress, ":", ""))
		group := "offline"
		if _, online := macs[m]; online {
			group = "online"
			vEdges = append(vEdges, gin.H{"from": "supernode", "to": m})
		}
		vNodes = append(vNodes, gin.H{"id": m, "label": n.Name, "group": group})
	}
	c.JSON(200, gin.H{"nodes": vNodes, "edges": vEdges})
}

func streamLogs(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	cmd := exec.Command("journalctl", "-u", "supernode", "-n", "100", "-f")
	stdout, _ := cmd.StdoutPipe()
	cmd.Start()
	defer cmd.Process.Kill()
	reader := bufio.NewReader(stdout)
	for {
		line, err := reader.ReadString('\n')
		if err != nil { break }
		fmt.Fprintf(c.Writer, "data: %s\n\n", strings.TrimSpace(line))
		c.Writer.Flush()
	}
}

func getActiveRelays(c *gin.Context) {

	relayMutex.Lock(); defer relayMutex.Unlock()

	active := make([]*RelayEvent, 0); now := time.Now()

	for key, ev := range relayMap {

		if now.Sub(ev.LastActive) < 60*time.Second { active = append(active, ev) } else { delete(relayMap, key) }

	}

	c.JSON(200, active)

}
