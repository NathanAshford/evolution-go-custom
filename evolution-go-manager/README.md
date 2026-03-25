<h1 align="center">Evolution Go Manager</h1>

<div align="center">

[![React](https://img.shields.io/badge/React-19-61DAFB?logo=react)](https://react.dev/)
[![TypeScript](https://img.shields.io/badge/TypeScript-5-3178C6?logo=typescript)](https://www.typescriptlang.org/)
[![Vite](https://img.shields.io/badge/Vite-6-646CFF?logo=vite)](https://vitejs.dev/)

</div>

## About

Web interface for managing WhatsApp instances through [Evolution Go](https://github.com/EvolutionAPI/evolution-go). Handles license activation, instance management, QR code pairing, messaging, and real-time event monitoring.

## Features

- **License Activation** — Built-in license registration and activation flow
- **Instance Management** — Create, connect, disconnect, and delete WhatsApp instances
- **QR Code Pairing** — Real-time QR code authentication
- **Messaging** — Send text, media, contacts, and location messages
- **Webhooks** — Per-instance webhook configuration
- **Event Monitor** — Real-time WebSocket event streaming
- **Dashboard** — Instance metrics and statistics

## Quick Start

```bash
# Install dependencies
pnpm install

# Run in development mode
pnpm dev

# Build for production
pnpm build
```

Available at `http://localhost:5174`

## Authentication & License

1. Enter your **Evolution Go API URL** (e.g., `http://localhost:8080`)
2. Enter your **GLOBAL_API_KEY** from the Evolution Go `.env` file
3. If no license is active, you'll be redirected to complete registration
4. After activation, the manager grants full access to the dashboard

Credentials are stored in the browser's localStorage.

## Project Structure

```
src/
├── pages/               # Login, Dashboard, Instances, LicenseCallback
├── components/
│   ├── base/            # Layout, Header, Sidebar, ErrorBoundary
│   └── instances/       # Instance cards, QR code, create modal
├── services/api/        # Axios client, license API, instances API
├── store/               # Zustand stores (auth, instances)
├── hooks/               # useAuth, useDarkMode
├── contexts/            # Theme context
└── types/               # TypeScript interfaces
```

## Technology Stack

| Component | Technology |
|-----------|-----------|
| Framework | React 19 |
| Language | TypeScript 5 |
| Build | Vite 6 |
| Styling | Tailwind CSS 4 |
| UI Components | @evoapi/design-system |
| State | Zustand |
| HTTP | Axios |
| Forms | React Hook Form + Zod |
| Routing | React Router 7 |
| Icons | Lucide React |
| Notifications | Sonner |

## Integration with Evolution Go

The manager communicates with Evolution Go via:

- **REST API** — All requests include `apikey` header
- **WebSocket** — Real-time events at `/ws?token=<apiKey>&instanceId=<id>`
- **License API** — `/license/status`, `/license/register`, `/license/activate`

## Docker

```bash
docker build -t evolution-go-manager:latest .
docker run -p 5174:80 evolution-go-manager:latest
```

## Documentation & Support

| Resource | Link |
|----------|------|
| Website | [evolutionfoundation.com.br](https://evolutionfoundation.com.br/) |
| Documentation | [docs.evolutionfoundation.com.br](https://docs.evolutionfoundation.com.br/) |
| Community | [evolutionfoundation.com.br/community](https://evolutionfoundation.com.br/community) |
| WhatsApp Support | [+55 31 7503-8350](https://wa.me/553175038350) |

## Hosting

| Product | Link |
|---------|------|
| Evolution Go VPS | [Hostgator - Evo Go](https://www.hostgator.com.br/52579-144-3-55.html) |
| Evolution API VPS | [Hostgator - Evo API](https://www.hostgator.com.br/servidor-vps/hospedagem-evo-api/lp-afiliado) |

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/new-feature`)
3. Commit your changes (`git commit -m 'feat: add new feature'`)
4. Push to the branch (`git push origin feature/new-feature`)
5. Open a Pull Request

## License

See [LICENSE](./LICENSE) file.

---

<div align="center">

**Evolution Go Manager** — WhatsApp Instance Management

Made with ❤️ by the [Evolution Team](https://evolutionfoundation.com.br/)

© 2025 Evolution Foundation

</div>
