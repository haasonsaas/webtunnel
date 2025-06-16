# WebTunnel - Enhanced Remote Terminal Access

WebTunnel is a modern, secure, and scalable terminal tunneling application that enables remote control of terminal applications through the web. Inspired by the excellent VibeTunnel project, WebTunnel builds upon its foundation with additional enterprise features and enhanced security.

## Key Differences from VibeTunnel (Our Inspiration)

### Security Enhancements
While VibeTunnel provides excellent local tunneling, WebTunnel adds:
- **JWT-based authentication** with session management
- **HTTPS by default** with automatic cert generation
- **Rate limiting** and input validation
- **User isolation** with separate session contexts
- **Environment variable protection** against leakage

### Performance Optimizations  
Building on VibeTunnel's solid foundation:
- **WebSocket-based communication** for lower latency than HTTP streaming
- **Connection pooling** for HTTP clients
- **Compressed terminal streams** using binary protocols
- **Session caching** with Redis backend
- **Resource limits** per session (CPU/memory)

### Enhanced User Experience
Expanding VibeTunnel's great UX:
- **Multi-user support** with role-based access
- **Session sharing** via secure URLs
- **Terminal customization** (themes, fonts, keybindings)
- **File upload/download** capabilities
- **Session restoration** after disconnects
- **Real-time collaboration** on terminal sessions

### Modern Architecture
Different tech stack from VibeTunnel's Swift/Rust approach:
- **Go backend** for better performance and concurrency
- **React frontend** with TypeScript
- **PostgreSQL** for persistent data storage
- **Redis** for session caching and pub/sub
- **Docker deployment** with Kubernetes support
- **Comprehensive monitoring** with Prometheus/Grafana

## Features

- **Secure Remote Terminal Access**: JWT authentication with RBAC
- **Real-time Terminal Streaming**: WebSocket-based with binary compression
- **Multi-user Session Management**: Isolated environments per user
- **File System Integration**: Upload/download files directly
- **Session Sharing**: Generate secure URLs for collaboration
- **Custom Terminal Settings**: Themes, fonts, and key bindings
- **Resource Monitoring**: CPU/memory usage per session
- **Auto-scaling**: Horizontal scaling with load balancing
- **Audit Logging**: Complete session activity tracking

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   React Frontend │────│   Go API Server  │────│  PostgreSQL DB  │
│   (Web UI)       │    │   (REST + WS)    │    │  (Persistence)  │
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

## Quick Start

```bash
# Clone the repository
git clone https://github.com/yourusername/webtunnel.git
cd webtunnel

# Start with Docker Compose
docker-compose up -d

# Or build from source
make build
./webtunnel serve --port 8443
```

Access WebTunnel at `https://localhost:8443`

## Requirements

- Go 1.21+
- Node.js 18+
- PostgreSQL 14+
- Redis 6+
- Docker (optional)

## License

MIT License - see LICENSE file for details.