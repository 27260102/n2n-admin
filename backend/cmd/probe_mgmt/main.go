package main

import (
	"fmt"
	"net"
	"time"
)

func main() {
	addr := "127.0.0.1:5644"
	fmt.Printf("正在探测 Supernode 管理端口: %s...\n", addr)

	conn, err := net.DialTimeout("udp", addr, 2*time.Second)
	if err != nil {
		fmt.Printf("连接失败: %v\n", err)
		return
	}
	defer conn.Close()

	// 发送 edges 命令获取节点信息
	// 注意：某些版本可能需要以 \n 结尾
	_, err = conn.Write([]byte("edges"))
	if err != nil {
		fmt.Printf("发送命令失败: %v\n", err)
		return
	}

	buffer := make([]byte, 8192)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buffer)
	if err != nil {
		fmt.Printf("读取响应失败 (可能是端口未开启或无响应): %v\n", err)
		fmt.Println("请确保 Supernode 启动参数包含 -t 5644")
		return
	}

	fmt.Println("--- 原始响应内容 ---")
	fmt.Println(string(buffer[:n]))
	fmt.Println("--- 响应结束 ---")
}
