package ajiasu

import (
	"bufio"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Node 爱加速节点信息
type Node struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// Manager 爱加速客户端管理器
type Manager struct {
	binary string

	mu        sync.RWMutex
	cmd       *exec.Cmd
	connected bool
	current   string
	proxyAddr string
	nodes     []Node
	lastList  time.Time
}

func New(binary string) *Manager {
	return &Manager{
		binary:    binary,
		proxyAddr: "127.0.0.1:1080",
	}
}

// Login 登录爱加速
func (m *Manager) Login() error {
	cmd := exec.Command(m.binary, "login")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("登录失败: %s, %w", string(out), err)
	}
	log.Printf("爱加速登录: %s", strings.TrimSpace(string(out)))
	return nil
}

// List 获取可用节点列表
func (m *Manager) List() ([]Node, error) {
	cmd := exec.Command(m.binary, "list")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("获取节点列表失败: %s, %w", string(out), err)
	}

	var nodes []Node
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "=") || strings.HasPrefix(line, "-") {
			continue
		}
		// 解析格式: "vvn-2431-7094 ok         厦门 #31"
		parts := strings.Fields(line)
		if len(parts) >= 3 && parts[1] == "ok" {
			name := strings.Join(parts[2:], " ")
			nodes = append(nodes, Node{ID: parts[0], Name: name})
		}
	}

	m.mu.Lock()
	m.nodes = nodes
	m.lastList = time.Now()
	m.mu.Unlock()

	return nodes, nil
}

// Connect 连接到指定节点（后台进程）
func (m *Manager) Connect(nodeName string) error {
	m.Disconnect()

	cmd := exec.Command(m.binary, "connect", nodeName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动失败: %w", err)
	}

	m.mu.Lock()
	m.cmd = cmd
	m.connected = true
	m.current = nodeName
	m.mu.Unlock()

	go func() {
		err := cmd.Wait()
		m.mu.Lock()
		m.connected = false
		m.current = ""
		m.cmd = nil
		m.mu.Unlock()
		if err != nil {
			log.Printf("爱加速进程退出: %v", err)
		}
	}()

	time.Sleep(3 * time.Second)
	log.Printf("爱加速已连接: %s, 代理: %s", nodeName, m.proxyAddr)
	return nil
}

// Disconnect 断开连接
func (m *Manager) Disconnect() {
	m.mu.Lock()
	cmd := m.cmd
	m.mu.Unlock()

	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}

	exec.Command(m.binary, "disconnect").Run()

	m.mu.Lock()
	m.connected = false
	m.current = ""
	m.cmd = nil
	m.mu.Unlock()
}

// IsConnected 是否已连接
func (m *Manager) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connected
}

// ProxyAddr 返回本地代理地址
func (m *Manager) ProxyAddr() string {
	return m.proxyAddr
}

// CurrentNode 当前节点
func (m *Manager) CurrentNode() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current
}

// Status 返回当前状态
func (m *Manager) Status() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := map[string]any{
		"connected":  m.connected,
		"proxy_addr": m.proxyAddr,
		"node_count": len(m.nodes),
	}
	if m.connected {
		status["current_node"] = m.current
	}
	if !m.lastList.IsZero() {
		status["last_list_at"] = m.lastList.Format("2006-01-02 15:04:05")
	}
	return status
}

// AutoResult 智能选节点结果
type AutoResult struct {
	Node      Node   `json:"node"`
	DelayMs   int    `json:"delay_ms"`
	IP        string `json:"ip"`
	ProxyAddr string `json:"proxy_addr"`
}

// DefaultCities 默认偏好城市
var DefaultCities = []string{"广州", "深圳", "厦门", "福州", "南宁"}

// TestConnection 测试当前代理连接的延迟和出口 IP
func (m *Manager) TestConnection() (delayMs int, ip string, err error) {
	delayCmd := exec.Command("curl",
		"-x", "http://"+m.proxyAddr,
		"-o", "/dev/null",
		"-s",
		"-w", "%{time_starttransfer}",
		"https://els.ztgame.com/",
		"--max-time", "5",
	)
	delayOut, err := delayCmd.CombinedOutput()
	if err != nil {
		return 0, "", fmt.Errorf("测速失败: %s, %w", string(delayOut), err)
	}
	totalSec, parseErr := strconv.ParseFloat(strings.TrimSpace(string(delayOut)), 64)
	if parseErr != nil {
		return 0, "", fmt.Errorf("解析延迟失败: %w", parseErr)
	}
	delayMs = int(totalSec * 1000)

	ipCmd := exec.Command("curl",
		"-x", "http://"+m.proxyAddr,
		"http://ip.sb",
		"--max-time", "10",
		"-s",
	)
	ipOut, err := ipCmd.CombinedOutput()
	if err != nil {
		return 0, "", fmt.Errorf("获取IP失败: %s, %w", string(ipOut), err)
	}
	ip = strings.TrimSpace(string(ipOut))

	return delayMs, ip, nil
}

// AutoSelect 智能选节点：过滤城市 → 随机选 → 连接 → 测速验证 → 失败重试
func (m *Manager) AutoSelect(preferCities []string) (*AutoResult, error) {
	if len(preferCities) == 0 {
		preferCities = DefaultCities
	}

	nodes, err := m.List()
	if err != nil {
		return nil, fmt.Errorf("获取节点列表失败: %w", err)
	}

	var filtered []Node
	for _, n := range nodes {
		for _, city := range preferCities {
			if strings.Contains(n.Name, city) {
				filtered = append(filtered, n)
				break
			}
		}
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("没有匹配城市 %v 的节点", preferCities)
	}

	rand.Shuffle(len(filtered), func(i, j int) {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	})

	const maxRetries = 8
	for i := 0; i < maxRetries && i < len(filtered); i++ {
		node := filtered[i]
		log.Printf("智能选节点: 尝试 %s (%d/%d)", node.Name, i+1, maxRetries)

		if err := m.Connect(node.Name); err != nil {
			log.Printf("连接失败: %s, %v", node.Name, err)
			continue
		}

		delayMs, ip, err := m.TestConnection()
		if err != nil {
			log.Printf("测速失败: %s, %v", node.Name, err)
			m.Disconnect()
			continue
		}

		if delayMs > 500 {
			log.Printf("延迟过高: %s, %dms", node.Name, delayMs)
			m.Disconnect()
			continue
		}

		log.Printf("智能选节点成功: %s, 延迟 %dms, IP %s", node.Name, delayMs, ip)
		return &AutoResult{
			Node:      node,
			DelayMs:   delayMs,
			IP:        ip,
			ProxyAddr: m.proxyAddr,
		}, nil
	}

	return nil, fmt.Errorf("尝试 %d 个节点均未达标(延迟>500ms)", maxRetries)
}
