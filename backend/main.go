package main

import (
	"bufio"
	"crypto/rand"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"n2n_ui/backend/config"
	"n2n_ui/backend/models"
	"n2n_ui/backend/utils"
	"os/exec"
	"regexp"
	"sort"
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

const Version = "v1.0.0"

//go:embed dist/*
var content embed.FS

var db *gorm.DB
var n2nMgmt *utils.MgmtClient
var jwtSecret []byte
var appConfig *config.Config

func setupConfig() {
	appConfig = config.Get()
	jwtSecret = []byte(appConfig.JWTSecret)
}

type IPLocation struct {
	Country string `json:"country"`
	City    string `json:"city"`
	ISP     string `json:"isp"`
}

type IPCacheEntry struct {
	Location  IPLocation
	CreatedAt time.Time
}

var (
	relayMap   = make(map[string]*RelayEvent)
	relayMutex sync.Mutex
	ipCache    = make(map[string]*IPCacheEntry)
	cacheMutex sync.Mutex
)

type RelayEvent struct {
	SrcMac     string    `json:"src_mac"`
	DstMac     string    `json:"dst_mac"`
	LastActive time.Time `json:"last_active"`
	PktCount   int64     `json:"pkt_count"`
}

// 登录防爆破
type LoginAttempt struct {
	FailCount int
	LockUntil time.Time
	LastFail  time.Time
}

const (
	maxLoginAttempts  = 5                // 最大失败次数
	lockDuration      = 15 * time.Minute // 锁定时长
	maxLoginRecords   = 10000            // 最大记录数，防止内存耗尽
	loginRecordExpiry = 1 * time.Hour    // 记录过期时间
)

var (
	loginAttempts = make(map[string]*LoginAttempt) // key: IP 或 username
	loginMutex    sync.Mutex
)

// cleanExpiredLoginAttempts 清理过期的登录记录
func cleanExpiredLoginAttempts() {
	loginMutex.Lock()
	defer loginMutex.Unlock()

	now := time.Now()
	for key, attempt := range loginAttempts {
		// 清理过期记录（未锁定且超过过期时间）
		if now.After(attempt.LockUntil) && now.Sub(attempt.LastFail) > loginRecordExpiry {
			delete(loginAttempts, key)
		}
	}
}

// startLoginCleanupRoutine 启动定期清理
func startLoginCleanupRoutine() {
	ticker := time.NewTicker(10 * time.Minute)
	for range ticker.C {
		cleanExpiredLoginAttempts()
	}
}

// checkLoginLock 检查是否被锁定，返回剩余锁定时间
func checkLoginLock(key string) (bool, time.Duration) {
	loginMutex.Lock()
	defer loginMutex.Unlock()

	if attempt, ok := loginAttempts[key]; ok {
		if time.Now().Before(attempt.LockUntil) {
			return true, time.Until(attempt.LockUntil)
		}
		// 锁定已过期，重置
		if attempt.FailCount >= maxLoginAttempts {
			attempt.FailCount = 0
		}
	}
	return false, 0
}

// recordLoginFail 记录登录失败
func recordLoginFail(ip, username string) (locked bool, remaining time.Duration) {
	loginMutex.Lock()
	defer loginMutex.Unlock()

	// 检查记录数量，超限时强制清理最旧的记录
	if len(loginAttempts) >= maxLoginRecords {
		now := time.Now()
		// 第一轮：清理过期记录
		for key, attempt := range loginAttempts {
			if now.After(attempt.LockUntil) && now.Sub(attempt.LastFail) > 5*time.Minute {
				delete(loginAttempts, key)
			}
		}
		// 第二轮：如果仍超限，强制删除最旧的记录
		for len(loginAttempts) >= maxLoginRecords {
			var oldestKey string
			var oldestTime time.Time
			for key, attempt := range loginAttempts {
				if oldestKey == "" || attempt.LastFail.Before(oldestTime) {
					oldestKey = key
					oldestTime = attempt.LastFail
				}
			}
			if oldestKey != "" {
				delete(loginAttempts, oldestKey)
			} else {
				break
			}
		}
	}

	// 同时记录 IP 和用户名
	for _, key := range []string{ip, "user:" + username} {
		if _, ok := loginAttempts[key]; !ok {
			loginAttempts[key] = &LoginAttempt{}
		}
		loginAttempts[key].FailCount++
		loginAttempts[key].LastFail = time.Now()
		if loginAttempts[key].FailCount >= maxLoginAttempts {
			loginAttempts[key].LockUntil = time.Now().Add(lockDuration)
			locked = true
			remaining = lockDuration
		}
	}
	return
}

// clearLoginFail 登录成功后清除失败记录
func clearLoginFail(ip, username string) {
	loginMutex.Lock()
	defer loginMutex.Unlock()
	delete(loginAttempts, ip)
	delete(loginAttempts, "user:"+username)
}

// generateRandomPassword 生成随机密码
func generateRandomPassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		log.Fatalf("Failed to generate random password: %v", err)
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b)
}

func initDB() {
	var err error
	db, err = gorm.Open(sqlite.Open(appConfig.DBPath), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect database")
	}
	db.AutoMigrate(&models.Node{}, &models.Community{}, &models.Setting{}, &models.User{})
	var userCount int64
	db.Model(&models.User{}).Count(&userCount)
	if userCount == 0 {
		// 生成随机密码而非固定密码
		randomPassword := generateRandomPassword(12)
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(randomPassword), bcrypt.DefaultCost)
		if err != nil {
			log.Fatal("Failed to hash default password: ", err)
		}
		db.Create(&models.User{Username: "admin", Password: string(hashedPassword), IsAdmin: true})
		log.Println("========================================")
		log.Println("  首次启动，已创建管理员账户")
		log.Printf("  用户名: admin")
		log.Printf("  密  码: %s", randomPassword)
		log.Println("  请立即登录并修改密码！")
		log.Println("========================================")
	}
}

func getIPLocation(ip string) IPLocation {
	if ip == "" || strings.HasPrefix(ip, "127.") || strings.HasPrefix(ip, "192.168.") || strings.HasPrefix(ip, "10.") {
		return IPLocation{Country: "本地网络", City: "-", ISP: "-"}
	}
	cacheMutex.Lock()
	if entry, ok := ipCache[ip]; ok {
		// 检查缓存是否过期
		if time.Since(entry.CreatedAt) < appConfig.IPCacheTTL {
			cacheMutex.Unlock()
			return entry.Location
		}
		// 缓存过期，删除并重新查询
		delete(ipCache, ip)
	}
	cacheMutex.Unlock()

	url := fmt.Sprintf("http://ip-api.com/json/%s?lang=zh-CN", ip) // 免费版仅支持 HTTP
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
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
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return IPLocation{Country: "未知", City: "-", ISP: "-"}
	}
	if result.Status != "success" {
		return IPLocation{Country: "未知", City: "-", ISP: "-"}
	}

	loc := IPLocation{Country: result.Country, City: result.City, ISP: result.ISP}
	cacheMutex.Lock()
	// 检查缓存大小限制
	if len(ipCache) >= appConfig.IPCacheSize {
		// 删除最旧的条目
		var oldestIP string
		var oldestTime time.Time
		for ip, entry := range ipCache {
			if oldestIP == "" || entry.CreatedAt.Before(oldestTime) {
				oldestIP = ip
				oldestTime = entry.CreatedAt
			}
		}
		if oldestIP != "" {
			delete(ipCache, oldestIP)
		}
	}
	ipCache[ip] = &IPCacheEntry{Location: loc, CreatedAt: time.Now()}
	cacheMutex.Unlock()
	return loc
}

// startIPCacheCleaner 定期清理过期的 IP 缓存
func startIPCacheCleaner() {
	ticker := time.NewTicker(1 * time.Hour)
	for range ticker.C {
		cacheMutex.Lock()
		now := time.Now()
		for ip, entry := range ipCache {
			if now.Sub(entry.CreatedAt) > appConfig.IPCacheTTL {
				delete(ipCache, ip)
			}
		}
		cacheMutex.Unlock()
	}
}

func startLogAnalyzer() {
	re := regexp.MustCompile(`forwarding packet.*from ([0-9A-Fa-f:]{17}) to ([0-9A-Fa-f:]{17})`)
	for {
		cmd := exec.Command("journalctl", "-u", "supernode", "-f", "-n", "0")
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			log.Printf("Log analyzer: failed to create pipe: %v, retrying in 5s", err)
			time.Sleep(5 * time.Second)
			continue
		}
		if err := cmd.Start(); err != nil {
			log.Printf("Log analyzer: failed to start: %v, retrying in 5s", err)
			time.Sleep(5 * time.Second)
			continue
		}
		reader := bufio.NewReader(stdout)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				log.Printf("Log analyzer: read error: %v, restarting", err)
				cmd.Process.Kill()
				break
			}
			matches := re.FindStringSubmatch(line)
			if len(matches) == 3 {
				src := strings.ToUpper(strings.ReplaceAll(matches[1], ":", ""))
				dst := strings.ToUpper(strings.ReplaceAll(matches[2], ":", ""))
				key := src + "->" + dst
				relayMutex.Lock()
				if ev, ok := relayMap[key]; ok {
					ev.LastActive = time.Now(); ev.PktCount++
				} else {
					relayMap[key] = &RelayEvent{SrcMac: src, DstMac: dst, LastActive: time.Now(), PktCount: 1}
				}
				relayMutex.Unlock()
			}
		}
		time.Sleep(2 * time.Second)
	}
}

func jwtMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := c.GetHeader("Authorization")
		if strings.HasPrefix(tokenString, "Bearer ") {
			tokenString = strings.TrimPrefix(tokenString, "Bearer ")
		}
		// 安全：不再从 URL query 参数读取 token，防止日志泄露
		if tokenString == "" {
			c.JSON(401, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}
		token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
			// 显式校验签名算法，防止算法混淆攻击
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return jwtSecret, nil
		})
		if err != nil || token == nil || !token.Valid {
			c.JSON(401, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(401, gin.H{"error": "Invalid claims"})
			c.Abort()
			return
		}
		c.Set("username", claims["username"])
		c.Next()
	}
}

func main() {
	port := flag.String("p", "", "Web UI 监听端口")
	showVersion := flag.Bool("v", false, "显示版本信息")
	resetPassword := flag.String("reset-password", "", "重置指定用户的密码 (格式: 用户名:新密码)")
	flag.Parse()

	if *showVersion {
		fmt.Printf("n2n-admin version: %s\n", Version)
		return
	}

	setupConfig()
	initDB()

	// 处理密码重置
	if *resetPassword != "" {
		parts := strings.SplitN(*resetPassword, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			fmt.Println("错误: 格式不正确，请使用 -reset-password 用户名:新密码")
			fmt.Println("示例: ./n2n_admin -reset-password admin:newpassword123")
			return
		}
		username, newPass := parts[0], parts[1]
		if len(newPass) < 6 {
			fmt.Println("错误: 密码长度至少 6 位")
			return
		}
		var user models.User
		if err := db.Where("username = ?", username).First(&user).Error; err != nil {
			fmt.Printf("错误: 用户 '%s' 不存在\n", username)
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(newPass), bcrypt.DefaultCost)
		if err != nil {
			fmt.Println("错误: 密码加密失败")
			return
		}
		db.Model(&user).Update("password", string(hash))
		fmt.Printf("成功: 用户 '%s' 的密码已重置\n", username)
		return
	}

	go startLogAnalyzer()
	go startIPCacheCleaner()
	go startLoginCleanupRoutine()
	n2nMgmt = &utils.MgmtClient{Addr: appConfig.MgmtAddr}

	// 安全提示
	if !appConfig.JWTSecretFromEnv {
		log.Println("[安全提示] JWT 密钥为自动生成，重启后所有用户需重新登录。建议设置环境变量 N2N_ADMIN_SECRET")
	}
	if appConfig.CORSOrigins == "" {
		log.Println("[配置] CORS 未配置，仅允许同源请求。如需跨域访问请设置 N2N_CORS_ORIGINS")
	}
	if appConfig.DisableNetTools {
		log.Println("[配置] 网络诊断工具已禁用 (默认)，如需启用请设置 N2N_ENABLE_NET_TOOLS=true")
	} else {
		log.Println("[安全提示] 网络诊断工具已启用，可用于 ping/traceroute 内网地址")
	}

	// 命令行参数优先于环境变量
	listenPort := appConfig.Port
	if *port != "" {
		listenPort = *port
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	corsConfig := cors.DefaultConfig()
	if appConfig.CORSOrigins != "" {
		if appConfig.CORSOrigins == "*" {
			corsConfig.AllowAllOrigins = true
		} else {
			corsConfig.AllowOrigins = strings.Split(appConfig.CORSOrigins, ",")
		}
	} else {
		// 默认只允许同源请求：使用自定义函数验证 Origin 与 Host 匹配
		corsConfig.AllowOriginFunc = func(origin string) bool {
			return false // 拒绝所有跨域请求
		}
	}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
	r.Use(cors.New(corsConfig))

	api := r.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok", "version": Version}) })
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
			protected.GET("/supernode/logs/recent", getRecentLogs)
			protected.GET("/relays", getActiveRelays)
			protected.POST("/change-password", changePassword)
		}
	}

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/") {
			c.JSON(404, gin.H{"error": "Not Found"}); return
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

	log.Printf("n2n-admin %s starting on :%s\n", Version, listenPort)
	r.Run(":" + listenPort)
}

func getNodes(c *gin.Context) {
	var nodes []models.Node; db.Find(&nodes)
	edges, _ := n2nMgmt.GetEdgeInfo()
	
	relayMutex.Lock()
	activeRelays := make(map[string]bool)
	for key := range relayMap {
		srcMac := strings.Split(key, "->")[0]
		activeRelays[srcMac] = true
	}
	relayMutex.Unlock()

	res := make([]interface{}, 0)
	mappedMacs := make(map[string]bool)
	for _, n := range nodes {
		m := strings.ToUpper(strings.ReplaceAll(n.MacAddress, ":", ""))
		info, online := edges[m]
		var publicIP, locationStr, connType string
		if online {
			publicIP = strings.Split(info.External, ":")[0]
			loc := getIPLocation(publicIP)
			locationStr = fmt.Sprintf("%s %s (%s)", loc.Country, loc.City, loc.ISP)
			connType = "P2P"
			if activeRelays[m] {
				connType = "Relay"
			}
		}
		res = append(res, gin.H{
			"id": n.ID, "name": n.Name, "ip_address": n.IPAddress, "mac_address": n.MacAddress, 
			"community": n.Community, "is_online": online, "is_mapped": true,
			"external_ip": publicIP, "location": locationStr, "conn_type": connType,
		})
		mappedMacs[m] = true
	}
	for mac, info := range edges {
		if !mappedMacs[mac] {
			publicIP := strings.Split(info.External, ":")[0]
			loc := getIPLocation(publicIP)
			connType := "P2P"
			if activeRelays[mac] { connType = "Relay" }
			res = append(res, gin.H{
			"id": 0, "name": "新发现节点", "ip_address": info.Internal, "mac_address": mac,
			"community": "未知", "is_online": true, "is_mapped": false,
			"external_ip": publicIP, "location": fmt.Sprintf("%s %s", loc.Country, loc.City), "conn_type": connType,
		})
		}
	}
	// 按 IP 地址数值排序
	sort.Slice(res, func(i, j int) bool {
		ipA := res[i].(gin.H)["ip_address"].(string)
		ipB := res[j].(gin.H)["ip_address"].(string)
		a := net.ParseIP(ipA)
		b := net.ParseIP(ipB)
		if a == nil || b == nil {
			return ipA < ipB
		}
		return utils.CompareIP(a, b) < 0
	})
	c.JSON(200, res)
}
func login(c *gin.Context) {
	clientIP := c.ClientIP()

	var p struct {
		U string `json:"username"`
		P string `json:"password"`
	}
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	// 检查 IP 是否被锁定
	if locked, remaining := checkLoginLock(clientIP); locked {
		c.JSON(429, gin.H{
			"error":   fmt.Sprintf("登录尝试次数过多，请 %d 分钟后再试", int(remaining.Minutes())+1),
			"locked":  true,
			"seconds": int(remaining.Seconds()),
		})
		return
	}

	// 检查用户名是否被锁定
	if locked, remaining := checkLoginLock("user:" + p.U); locked {
		c.JSON(429, gin.H{
			"error":   fmt.Sprintf("该账户已被临时锁定，请 %d 分钟后再试", int(remaining.Minutes())+1),
			"locked":  true,
			"seconds": int(remaining.Seconds()),
		})
		return
	}

	var user models.User
	if err := db.Where("username = ?", p.U).First(&user).Error; err != nil {
		recordLoginFail(clientIP, p.U)
		c.JSON(401, gin.H{"error": "用户名或密码错误"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(p.P)); err != nil {
		locked, remaining := recordLoginFail(clientIP, p.U)
		if locked {
			c.JSON(429, gin.H{
				"error":   fmt.Sprintf("登录失败次数过多，账户已锁定 %d 分钟", int(remaining.Minutes())),
				"locked":  true,
				"seconds": int(remaining.Seconds()),
			})
		} else {
			c.JSON(401, gin.H{"error": "用户名或密码错误"})
		}
		return
	}

	// 登录成功，清除失败记录
	clearLoginFail(clientIP, p.U)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": user.Username,
		"exp":      time.Now().Add(time.Hour * 24).Unix(),
	})
	t, err := token.SignedString(jwtSecret)
	if err != nil {
		log.Printf("Failed to sign JWT token: %v", err)
		c.JSON(500, gin.H{"error": "Internal server error"})
		return
	}
	c.JSON(200, gin.H{"token": t, "user": user})
}

func changePassword(c *gin.Context) {
	var p struct { Old string `json:"old_password"`; New string `json:"new_password"` }
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"}); return
	}
	if len(p.New) < 6 {
		c.JSON(400, gin.H{"error": "New password must be at least 6 characters"}); return
	}
	u, _ := c.Get("username")
	var user models.User
	if err := db.Where("username = ?", u).First(&user).Error; err != nil {
		c.JSON(404, gin.H{"error": "User not found"}); return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(p.Old)); err != nil {
		c.JSON(401, gin.H{"error": "Old password incorrect"}); return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(p.New), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Failed to hash new password: %v", err)
		c.JSON(500, gin.H{"error": "Internal server error"}); return
	}
	db.Model(&user).Update("password", string(hash))
	c.JSON(200, gin.H{"message": "success"})
}

func getStats(c *gin.Context) {
	var n, cm int64; db.Model(&models.Node{}).Count(&n); db.Model(&models.Community{}).Count(&cm)
	macs, _ := n2nMgmt.GetOnlineMacs()
	c.JSON(200, gin.H{"node_count": n, "community_count": cm, "online_count": len(macs)})
}

// isValidMac 验证 MAC 地址格式
func isValidMac(mac string) bool {
	// 支持 AA:BB:CC:DD:EE:FF 或 AA-BB-CC-DD-EE-FF 或 AABBCCDDEEFF
	cleaned := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(mac, ":", ""), "-", ""))
	if len(cleaned) != 12 {
		return false
	}
	for _, c := range cleaned {
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func createNode(c *gin.Context) {
	var p struct {
		models.Node
		RouteNet string `json:"route_net"`
		RouteGw  string `json:"route_gw"`
	}
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	n := p.Node

	// 验证节点名称
	if strings.TrimSpace(n.Name) == "" {
		c.JSON(400, gin.H{"error": "Node name is required"})
		return
	}

	// 验证社区存在
	var comm models.Community
	if err := db.Where("name = ?", n.Community).First(&comm).Error; err != nil {
		c.JSON(400, gin.H{"error": "Community not found"})
		return
	}

	// 验证并处理 MAC 地址
	if n.MacAddress != "" {
		if !isValidMac(n.MacAddress) {
			c.JSON(400, gin.H{"error": "Invalid MAC address format"})
			return
		}
		n.MacAddress = strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(n.MacAddress, ":", ""), "-", ""))
	} else {
		mac, err := utils.GenerateRandomMac()
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to generate MAC address"})
			return
		}
		n.MacAddress = strings.ToUpper(strings.ReplaceAll(mac, ":", ""))
	}

	// 验证并处理 IP 地址
	if n.IPAddress != "" {
		if ip := net.ParseIP(n.IPAddress); ip == nil {
			c.JSON(400, gin.H{"error": "Invalid IP address format"})
			return
		}
		// 检查 IP 是否在社区范围内
		if comm.Range != "" {
			if _, ipnet, err := net.ParseCIDR(comm.Range); err == nil {
				if !ipnet.Contains(net.ParseIP(n.IPAddress)) {
					c.JSON(400, gin.H{"error": "IP address not in community range"})
					return
				}
			}
		}
	} else {
		// 自动分配 IP
		if comm.Range != "" {
			if baseIP, _, err := net.ParseCIDR(comm.Range); err == nil {
				var nodes []models.Node
				db.Where("community = ?", n.Community).Find(&nodes)
				if len(nodes) == 0 {
					n.IPAddress = utils.NextIP(utils.NextIP(baseIP)).String()
				} else {
					// 找出最大的 IP 地址（按数值比较）
					var maxIP net.IP
					for _, node := range nodes {
						ip := net.ParseIP(node.IPAddress)
						if ip != nil && (maxIP == nil || utils.CompareIP(ip, maxIP) > 0) {
							maxIP = ip
						}
					}
					if maxIP != nil {
						n.IPAddress = utils.NextIP(maxIP).String()
					} else {
						n.IPAddress = utils.NextIP(utils.NextIP(baseIP)).String()
					}
				}
			}
		}
	}

	// 处理路由配置
	if p.RouteNet != "" && p.RouteGw != "" {
		// 验证路由网段格式
		if _, _, err := net.ParseCIDR(p.RouteNet); err != nil {
			c.JSON(400, gin.H{"error": "Invalid route network format"})
			return
		}
		// 验证网关 IP
		if ip := net.ParseIP(p.RouteGw); ip == nil {
			c.JSON(400, gin.H{"error": "Invalid route gateway format"})
			return
		}
		n.Routing = p.RouteNet + ":" + p.RouteGw
	}

	db.Unscoped().Where("mac_address = ? OR ip_address = ?", n.MacAddress, n.IPAddress).Delete(&models.Node{})
	if err := db.Create(&n).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to create node"})
		return
	}
	c.JSON(200, n)
}

func deleteNode(c *gin.Context) {
	db.Delete(&models.Node{}, c.Param("id")); c.JSON(200, gin.H{"message": "deleted"})
}

func getNodeConfig(c *gin.Context) {
	var n models.Node; db.First(&n, c.Param("id"))
	var comm models.Community; db.Where("name = ?", n.Community).First(&comm)
	password := comm.Password; if password == "" { password = "password" }
	var s models.Setting; db.Where("key = ?", "supernode_host").First(&s)
	params := utils.ConfigParams{
		Name: n.Name, IP: n.IPAddress, Community: n.Community, Password: password, Supernode: s.Value, Mac: n.MacAddress,
		Encryption: n.Encryption, Compression: n.Compression, Routing: n.Routing, LocalPort: n.LocalPort,
	}
	c.JSON(200, gin.H{"conf": utils.GenerateConfFile(params)})
}

func getCommunities(c *gin.Context) {
	var comms []models.Community; db.Find(&comms); c.JSON(200, comms)
}

func syncCommunityList() {
	var comms []models.Community; db.Find(&comms)
	names := make([]string, 0)
	for _, c := range comms { names = append(names, c.Name) }
	utils.WriteCommunityList("/etc/n2n/community.list", names)
}

func createCommunity(c *gin.Context) {
	var cm models.Community
	if err := c.ShouldBindJSON(&cm); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}
	// 验证社区名称
	if strings.TrimSpace(cm.Name) == "" {
		c.JSON(400, gin.H{"error": "Community name is required"})
		return
	}
	// 验证 CIDR 格式
	if cm.Range != "" {
		if _, _, err := net.ParseCIDR(cm.Range); err != nil {
			c.JSON(400, gin.H{"error": "Invalid CIDR format"})
			return
		}
	}
	// 验证密码长度
	if len(cm.Password) < 4 {
		c.JSON(400, gin.H{"error": "Password must be at least 4 characters"})
		return
	}
	// 检查重复
	var existing models.Community
	if err := db.Where("name = ?", cm.Name).First(&existing).Error; err == nil {
		c.JSON(400, gin.H{"error": "Community already exists"})
		return
	}
	if err := db.Create(&cm).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to create community"})
		return
	}
	syncCommunityList()
	c.JSON(200, cm)
}

func deleteCommunity(c *gin.Context) {
	db.Delete(&models.Community{}, c.Param("id")); syncCommunityList(); c.JSON(200, gin.H{"message": "deleted"})
}

func getSettings(c *gin.Context) {
	var s []models.Setting; db.Find(&s)
	res := make(map[string]string)
	for _, x := range s { res[x.Key] = x.Value }
	c.JSON(200, res)
}

func saveSettings(c *gin.Context) {
	var p map[string]string; c.ShouldBindJSON(&p)
	for k, v := range p { db.Where("key = ?", k).Assign(models.Setting{Value: v}).FirstOrCreate(&models.Setting{Key: k}) }
	c.JSON(200, gin.H{"message": "saved"})
}

func getSupernodeConfig(c *gin.Context) {
	cfg, _ := utils.ReadSupernodeConfig("/etc/n2n/supernode.conf"); c.JSON(200, cfg)
}

func saveSupernodeConfig(c *gin.Context) {
	var n map[string]string; c.ShouldBindJSON(&n)
	curr, _ := utils.ReadSupernodeConfig("/etc/n2n/supernode.conf")
	if curr == nil { curr = make(map[string]string) }
	for k, v := range n { curr[k] = v }
	curr["f"] = ""; curr["v"] = ""; utils.WriteSupernodeConfig("/etc/n2n/supernode.conf", curr)
	c.JSON(200, gin.H{"message": "saved"})
}

func restartSupernode(c *gin.Context) {
	utils.RunCommand("systemctl", "restart", "supernode"); c.JSON(200, gin.H{"message": "restarted"})
}

// isValidTarget 验证目标是否为有效的 IP 地址或域名
func isValidTarget(target string) bool {
	if target == "" {
		return false
	}
	// 检查是否为有效 IP
	if ip := net.ParseIP(target); ip != nil {
		return true
	}
	// 检查是否为有效域名（只允许字母、数字、点和连字符）
	domainRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)
	return domainRegex.MatchString(target) && len(target) <= 253
}

func execTool(c *gin.Context) {
	// 检查是否禁用网络工具
	if appConfig.DisableNetTools {
		c.JSON(403, gin.H{"error": "网络诊断工具已被管理员禁用"})
		return
	}
	var p struct { Command string `json:"command"`; Target string `json:"target"` }
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"}); return
	}
	// 验证命令类型
	if p.Command != "ping" && p.Command != "traceroute" {
		c.JSON(400, gin.H{"error": "Invalid command"}); return
	}
	// 验证目标地址
	if !isValidTarget(p.Target) {
		c.JSON(400, gin.H{"error": "Invalid target address"}); return
	}
	var out string
	var err error
	if p.Command == "ping" {
		out, err = utils.RunCommand("ping", "-c", "4", "-W", "2", p.Target)
	} else {
		out, err = utils.RunCommand("traceroute", "-m", "10", "-n", p.Target)
	}
	if err != nil {
		c.JSON(200, gin.H{"output": out, "error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"output": out})
}

func getTopology(c *gin.Context) {
	var nodes []models.Node; db.Find(&nodes)
	macs, _ := n2nMgmt.GetOnlineMacs()
	vNodes := []interface{}{gin.H{"id": "supernode", "label": "Supernode", "group": "supernode"}}
	vEdges := []interface{}{}
	for _, n := range nodes {
		m := strings.ToUpper(strings.ReplaceAll(n.MacAddress, ":", "")); group := "offline"
		if _, online := macs[m]; online { group = "online"; vEdges = append(vEdges, gin.H{"from": "supernode", "to": m}) }
		vNodes = append(vNodes, gin.H{"id": m, "label": n.Name, "group": group})
	}
	c.JSON(200, gin.H{"nodes": vNodes, "edges": vEdges})
}

func streamLogs(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream"); c.Header("Cache-Control", "no-cache"); c.Header("Connection", "keep-alive")
	cmd := exec.Command("journalctl", "-u", "supernode", "-n", "100", "-f")
	stdout, _ := cmd.StdoutPipe(); cmd.Start(); defer cmd.Process.Kill()
	reader := bufio.NewReader(stdout)
	for {
		line, err := reader.ReadString('\n'); if err != nil { break }
		fmt.Fprintf(c.Writer, "data: %s\n\n", strings.TrimSpace(line)); c.Writer.Flush()
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

func getRecentLogs(c *gin.Context) {
	out, err := exec.Command("journalctl", "-u", "supernode", "-n", "100", "--no-pager").Output()
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to read logs"})
		return
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	c.JSON(200, gin.H{"logs": lines})
}