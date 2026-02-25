# ajiasu-proxy-api

爱加速国内 IP 代理 API 服务。通过 API 一键智能选节点，配合 socat 端口转发，让远程客户端（如 Windows Proxifier）通过 VPS 使用国内 IP 出口。

## 代理链路

```
客户端 (Proxifier) → VPS:47076 (socat) → 127.0.0.1:1080 (爱加速 SOCKS5) → 国内 IP
```

## 快速开始

### 1. 编译

```bash
# 本地交叉编译
GOOS=linux GOARCH=amd64 go build -o ajiasu-proxy-api .

# 上传到 VPS
scp ajiasu-proxy-api user@vps:/opt/ajiasu-proxy-api/
```

### 2. 配置

```bash
cp configs/app.example.yaml configs/app.yaml
# 编辑 configs/app.yaml，填入实际的 token 和 ajiasu 二进制路径
```

```yaml
server:
  port: 47079
  token: "your-secret-token"

ajiasu:
  binary: "/opt/ajiasu-proxy-api/ajiasu"
```

### 3. 部署

```bash
# 安装 systemd 服务
sudo cp deploy/ajiasu-proxy-api.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now ajiasu-proxy-api

# socat 端口转发（1080 → 47076 公网）
sudo cp deploy/ajiasu-forward.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now ajiasu-forward
```

### 4. 防火墙

```bash
sudo ufw allow 47079/tcp  # API 端口
sudo ufw allow 47076/tcp  # 代理端口
```

## API 接口

所有接口需要 Header: `Authorization: Bearer <token>`

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/ajiasu/auto` | 一键智能选节点 |
| GET | `/api/ajiasu/status` | 查看当前状态 |
| GET | `/api/ajiasu/nodes` | 获取所有节点列表 |
| POST | `/api/ajiasu/connect` | 连接指定节点 |
| POST | `/api/ajiasu/disconnect` | 断开连接 |

### POST /api/ajiasu/auto

智能选节点：按城市过滤 → 随机选 → 连接 → 测速验证（延迟 < 500ms）→ 失败自动重试（最多 8 次）。

每次调用会切换到新节点，出口 IP 随之改变。

```bash
# 使用默认城市（广州、深圳、厦门、福州、南宁）
curl -X POST http://VPS:47079/api/ajiasu/auto \
  -H "Authorization: Bearer <token>"

# 指定城市
curl -X POST http://VPS:47079/api/ajiasu/auto \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"cities":["广州","深圳"]}'
```

响应：

```json
{
  "node": {"name": "广州 #229", "id": "vvn-5829-8843"},
  "delay_ms": 263,
  "ip": "183.240.37.206",
  "proxy_addr": "127.0.0.1:1080",
  "message": "智能选节点成功"
}
```

### GET /api/ajiasu/status

查看当前连接状态（不会切换节点）。

```json
{
  "connected": true,
  "proxy_addr": "127.0.0.1:1080",
  "node_count": 2683,
  "current_node": "广州 #229"
}
```

### POST /api/ajiasu/connect

连接指定节点。

```bash
curl -X POST http://VPS:47079/api/ajiasu/connect \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"node":"厦门 #31"}'
```

## 客户端配置（Windows Proxifier）

1. Profile → Proxy Servers → Add
   - Address: `VPS 公网 IP`
   - Port: `47076`
   - Protocol: **SOCKS5**
2. Profile → Proxification Rules 添加需要走国内 IP 的程序
3. 浏览器访问 `http://ip.sb` 确认显示国内 IP

## 项目结构

```
├── main.go                          # 入口
├── configs/
│   └── app.example.yaml             # 配置示例
├── deploy/
│   ├── ajiasu-proxy-api.service     # API 服务 systemd
│   └── ajiasu-forward.service       # socat 转发 systemd
└── internal/
    ├── ajiasu/
    │   └── manager.go               # 爱加速客户端管理（节点、连接、测速）
    └── api/
        ├── router.go                # 路由 + 认证中间件
        └── handler.go               # API handler
```
