package middleware

import (
	"net"
	"strings"
	"time"

	"vrec/pkg/sid"

	"github.com/gin-gonic/gin"
)

const (
	SidContextKey = "sid"
)

type SidMiddleware struct {
	generator *sid.Generator
	serverIP  string
}

func NewSidMiddleware(sidSecret string) *SidMiddleware {
	serverIP := getServerIP()
	return &SidMiddleware{
		generator: sid.NewGenerator(sidSecret, serverIP),
		serverIP:  serverIP,
	}
}

// getServerIP 获取服务器 IPv4 地址
// 优先从网卡获取，多网卡取第一个，不行再降级到 UDP 解析
func getServerIP() string {
	// 从网卡获取
	if ip := getFirstIPv4FromInterfaces(); ip != "" {
		return ip
	}
	// 降级到 UDP 解析
	return getOutboundIP()
}

// getFirstIPv4FromInterfaces 从网卡获取第一个 IPv4 地址
func getFirstIPv4FromInterfaces() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		// 跳过 loopback 和禁用状态
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

// getOutboundIP 通过 UDP 解析获取出口 IP
func getOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}

func (m *SidMiddleware) SidMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 优先从 header 获取用户传入的 sid
		userSid := strings.TrimSpace(c.GetHeader(sid.SidHeader))

		var sidValue string
		if userSid != "" {
			sidValue = userSid
		} else {
			// 用户未提供，使用服务器 IP 生成 sid
			sidValue = m.generator.Generate(time.Now())
		}

		c.Set(SidContextKey, sidValue)
		c.Header(sid.SidHeader, sidValue)
		c.Next()
	}
}
