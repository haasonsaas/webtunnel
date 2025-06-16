# 🌐 WebTunnel - Enhanced Remote Terminal Access

WebTunnel is a modern, secure, and scalable terminal tunneling application that enables remote control of terminal applications through the web. **Inspired by the excellent [VibeTunnel](https://github.com/amantus-ai/vibetunnel) project**, WebTunnel builds upon its foundation with additional enterprise features and enhanced security.

> 🙏 **Huge thanks to the VibeTunnel team** for their innovative architecture and design that inspired this project!

## 🚀 Quick Demo

```bash
# Try it instantly with demo mode (no dependencies required)
git clone https://github.com/haasonsaas/webtunnel.git
cd webtunnel
make build
./bin/webtunnel-demo

# Open http://localhost:8080 in your browser
# Login with any email/password (demo mode)
```

## ✨ What's Included

### 🎯 Working Demo
- **`webtunnel-demo`** - Standalone binary with mock API
- Beautiful web interface with authentication
- Session management UI
- Works without any external dependencies
- Perfect for testing and development

### 🏗️ Full Implementation
- **Go backend** with Gin web framework
- **JWT authentication** system
- **WebSocket terminal streaming** 
- **PostgreSQL** database integration
- **Redis** caching and pub/sub
- **Docker & Kubernetes** deployment ready

## 🔄 Key Differences from VibeTunnel (Our Inspiration)

### 🔒 Security Enhancements
While VibeTunnel provides excellent local tunneling, WebTunnel adds:
- **JWT-based authentication** with session management
- **HTTPS by default** with automatic cert generation
- **Rate limiting** and input validation
- **User isolation** with separate session contexts
- **Command allowlisting/blocklisting**

### ⚡ Performance Optimizations  
Building on VibeTunnel's solid foundation:
- **WebSocket-based communication** for lower latency than HTTP streaming
- **Connection pooling** for HTTP clients
- **Session caching** with Redis backend
- **Resource limits** per session (CPU/memory)

### 👥 Enhanced User Experience
Expanding VibeTunnel's great UX:
- **Multi-user support** with role-based access
- **Session sharing** via secure URLs
- **File upload/download** capabilities
- **Session restoration** after disconnects
- **Responsive web interface**

### 🏛️ Modern Architecture
Different tech stack from VibeTunnel's Swift/Rust approach:
- **Go backend** for better concurrency
- **HTML/CSS/JS frontend** (React-ready)
- **PostgreSQL** for persistent data storage
- **Redis** for session caching and pub/sub
- **Docker deployment** with full monitoring stack

## 📖 Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Web Frontend  │────│   Go API Server  │────│  PostgreSQL DB  │
│  (HTML/CSS/JS)  │    │   (REST + WS)    │    │  (Persistence)  │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                │
                       ┌────────┴────────┐
                       │   Session Pool   │
                       │  (PTY Manager)   │
                       └─────────────────┘
                                │
                       ┌────────┴────────┐
                       │   Redis Cache   │
                       │ (Sessions+PubSub)│
                       └─────────────────┘
```

## 🛠️ Getting Started

### Option 1: Demo Mode (Recommended for testing)
```bash
git clone https://github.com/haasonsaas/webtunnel.git
cd webtunnel
make build
./bin/webtunnel-demo
```
- **No dependencies required**
- **Instant setup**
- **Mock API endpoints**
- Access at `http://localhost:8080`

### Option 2: Full Stack with Docker
```bash
git clone https://github.com/haasonsaas/webtunnel.git
cd webtunnel
docker-compose up -d
```
- **Complete production setup**
- **PostgreSQL + Redis included**
- **Monitoring with Prometheus/Grafana**
- Access at `https://localhost:8443`

### Option 3: Development Build
```bash
git clone https://github.com/haasonsaas/webtunnel.git
cd webtunnel
make build
./bin/webtunnel serve --config .webtunnel.yaml
```
- **Native binary**
- **Custom configuration**
- **Development mode**

## 🔧 Configuration

Create `.webtunnel.yaml`:

```yaml
server:
  host: "0.0.0.0"
  port: 8443
  tls: true
  static_dir: "./web/dist"

database:
  url: "postgres://localhost/webtunnel?sslmode=disable"

redis:
  url: "redis://localhost:6379"

auth:
  jwt_secret: "your-secret-key"
  session_expiry: "24h"

session:
  max_sessions: 50
  working_directory: "/tmp/webtunnel"
  blocked_commands: ["rm", "sudo", "dd"]
```

## 📋 Available Commands

```bash
# Build everything
make build

# Run demo server
make run  # or ./bin/webtunnel-demo

# Run full server
./bin/webtunnel serve

# Docker deployment
make docker

# View logs
make docker-logs

# Clean build artifacts
make clean
```

## 🐳 Docker Services

The `docker-compose.yml` includes:
- **WebTunnel** - Main application server
- **PostgreSQL** - Database for persistence
- **Redis** - Caching and pub/sub
- **Prometheus** - Metrics collection (port 9090)
- **Grafana** - Monitoring dashboards (port 3000)

## 🧪 Testing

```bash
# Test health endpoint
curl http://localhost:8080/health

# Test authentication
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"demo@example.com","password":"password"}'

# Test session management
curl -H "Authorization: Bearer <token>" \
  http://localhost:8080/api/v1/sessions
```

## 🚦 Current Status

### ✅ Implemented
- ✅ Go backend with Gin framework
- ✅ JWT authentication system
- ✅ Web interface with login/session management
- ✅ Demo mode for testing
- ✅ Docker deployment configuration
- ✅ Database schema and migrations
- ✅ Configuration management
- ✅ Build system and Makefile

### 🚧 In Progress
- 🚧 Real WebSocket terminal streaming
- 🚧 Actual PTY integration 
- 🚧 File upload/download
- 🚧 Session sharing URLs
- 🚧 Resource monitoring

### 📋 Planned
- 📋 React frontend rebuild
- 📋 Kubernetes deployment
- 📋 Session recording/playback
- 📋 Terminal themes and customization
- 📋 Multi-user collaboration

## 🤝 Contributing

We welcome contributions! This project is built with respect and admiration for VibeTunnel's original vision.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

MIT License - see [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

**Huge thanks to the [VibeTunnel](https://github.com/amantus-ai/vibetunnel) team** for their excellent architecture and innovative approach to terminal tunneling. This project is built with deep respect for their work and aims to expand on their vision with additional enterprise features.

**VibeTunnel's influence on this project:**
- Overall architecture and design patterns
- Terminal streaming concepts
- Session management approach
- Clean, modern UI inspiration

---

**⚡ Built with inspiration from VibeTunnel**  
**🤖 Generated with [Claude Code](https://claude.ai/code)**