# ADR-002: Networking Stack for Pico Hardware Control

## Status

Proposed

## Context

We need secure remote communication with the Raspberry Pi Pico running the LED matrix driver. The Pico operates without an OS, so we cannot rely on standard networking facilities. We need a solution that:

- Works on bare metal (no OS)
- Provides secure communication (SSH)
- Has minimal memory footprint
- Is compatible with TinyGo
- **Supports cross-platform development** - code must run on macOS and Windows for testing before deploying to real hardware
- **Enables device provisioning** - configure WiFi credentials, device identity, and initial settings

Two components have been identified:

1. **lneto** - Userspace TCP/IP stack for embedded systems
2. **ssh-bridge** - SSH tunnel server for secure bridged communication

## Decision

Use **lneto** as the networking foundation on the Pico, with **ssh-bridge** on a host machine for secure remote access.

## Architecture

### Production (Pico Hardware)

```
[Remote Client] <--SSH--> [ssh-bridge on Host] <--TCP/IP--> [Pico with lneto]
```

### Development (macOS/Windows)

```
[Application] <--lneto--> [TAP Interface] <---> [Host Network Stack]
```

TAP (network tap) virtual interfaces allow lneto to run on desktop operating systems by providing a virtual Layer 2 device. This enables testing the full networking code on laptops before deploying to real Pico hardware.

## Options Evaluated

### lneto (Selected for Pico and Desktop Testing)

**Pros:**
- Zero external dependencies
- Heapless packet processing
- Tiny footprint (~1kB for full Ethernet+IPv4+UDP+DHCP+DNS+NTP stack)
- Designed for TinyGo on Pico
- Nanosecond-level performance with zero allocations
- Supports: Ethernet, ARP, IPv4, IPv6, TCP, UDP, DHCP, DNS, NTP, HTTP/1.1
- **Cross-platform**: Can run on macOS/Windows via TAP interfaces for development testing

**Cons:**
- No built-in SSH support (requires external bridge)
- Relatively new project
- TAP interface setup required on desktop (may need admin privileges)

### ssh-bridge (Selected for Host)

**Pros:**
- Written in Go
- Isolated port forwarding (virtual spaces, not actual host ports)
- No TTY access by design (security)
- Key-based authentication only
- Efficient - reduces kernel-userspace transitions

**Cons:**
- Requires separate host machine
- Additional component to deploy

## Consequences

- The Pico will run lneto for raw TCP/IP networking
- Secure remote access achieved via ssh-bridge on an intermediary host
- No direct SSH on Pico (acceptable given memory constraints)
- Full networking capability without OS overhead
- **Same networking code runs on desktop and Pico** - TAP interfaces on macOS/Windows enable testing without hardware
- Developers can iterate quickly on laptops before flashing to Pico

## Cross-Platform Development

### TAP Interface Requirements

| Platform | TAP Solution |
|----------|--------------|
| macOS    | `utun` or third-party TAP driver (e.g., Tunnelblick's tap-kext) |
| Windows  | OpenVPN TAP-Windows adapter or WinTun |
| Linux    | Native TUN/TAP support via `/dev/net/tun` |

### Development Workflow

1. Write networking code using lneto
2. Test on laptop via TAP interface
3. Validate behavior matches expected protocol
4. Deploy to Pico hardware for final testing

## Go Build Tags

Build tags enable the same codebase to target different platforms with platform-specific implementations.

### Tag Structure

```
//go:build pico
// +build pico

//go:build darwin || windows || linux
// +build darwin windows linux
```

### Full Code Structure

```
plat-led/
├── cmd/
│   ├── led-driver/
│   │   └── main.go                  # LED driver entry point (runs on Pico or desktop sim)
│   └── led-client/
│       └── main.go                  # Client CLI for sending commands via SSH bridge
│
├── internal/
│   ├── netif/                       # Network interface abstraction
│   │   ├── netif.go                 # Interface definition: type NetIF interface { ... }
│   │   ├── netif_pico.go            # //go:build pico - Pico W cyw43 WiFi driver
│   │   ├── netif_tap_darwin.go      # //go:build darwin - macOS utun/TAP
│   │   ├── netif_tap_windows.go     # //go:build windows - WinTun/TAP-Windows
│   │   └── netif_tap_linux.go       # //go:build linux - /dev/net/tun
│   │
│   ├── led/                         # LED matrix driver
│   │   ├── matrix.go                # Shared types: Color, Frame, Matrix interface
│   │   ├── matrix_pico.go           # //go:build pico - PIO-based hardware driver
│   │   └── matrix_sim.go            # //go:build !pico - Terminal/GUI simulation
│   │
│   ├── protocol/                    # Wire protocol for client<->driver communication
│   │   ├── protocol.go              # Message types: SetPixel, SetFrame, GetStatus
│   │   ├── encode.go                # Binary encoding (minimal allocations)
│   │   └── decode.go                # Binary decoding
│   │
│   └── server/                      # TCP server running on lneto stack
│       ├── server.go                # Accepts connections, dispatches commands
│       └── handler.go               # Handles protocol messages, updates matrix
│
├── pkg/
│   └── client/                      # Client library for external use
│       └── client.go                # Connect(), SetPixel(), SetFrame(), etc.
│
├── pio/                             # PIO assembly programs (Pico only)
│   ├── hub75.pio                    # HUB75 LED panel timing
│   └── hub75.pio.go                 # Generated Go bindings (tinygo-org/pio)
│
├── configs/
│   ├── process-compose.yaml         # Local dev simulation orchestration
│   └── pico.json                    # TinyGo target configuration overrides
│
├── scripts/
│   ├── setup-tap-darwin.sh          # Create TAP interface on macOS
│   ├── setup-tap-windows.ps1        # Create TAP interface on Windows
│   └── flash.sh                     # Build and flash to Pico
│
├── docs/
│   ├── adr-001-runtime-approach.md
│   └── adr-002-networking-stack.md
│
├── go.mod
├── go.sum
└── README.md
```

### Package Responsibilities

| Package | Purpose |
|---------|---------|
| `cmd/led-driver` | Main binary for Pico and desktop simulation |
| `cmd/led-client` | CLI tool to control LED matrix remotely |
| `internal/netif` | Platform-specific network interface (lneto on Pico, TAP on desktop) |
| `internal/led` | LED matrix hardware abstraction (PIO on Pico, mock on desktop) |
| `internal/protocol` | Binary wire protocol for efficient communication |
| `internal/server` | TCP server accepting client commands |
| `pkg/client` | Reusable client library |
| `pio/` | PIO programs for precise LED timing |

### Interface Contracts

```go
// internal/netif/netif.go
type NetIF interface {
    Init() error
    MACAddress() [6]byte
    SendPacket([]byte) error
    RecvPacket([]byte) (int, error)
    Close() error
}

// internal/led/matrix.go
type Matrix interface {
    Init(rows, cols int) error
    SetPixel(x, y int, c Color) error
    SetFrame(f *Frame) error
    Refresh() error  // Called by PIO interrupt on Pico, goroutine on desktop
    Close() error
}
```

### Build Commands

```bash
# Desktop testing (macOS)
go build -o led-driver ./cmd/led-driver

# Desktop testing (Windows)
GOOS=windows go build -o led-driver.exe ./cmd/led-driver

# Pico hardware (TinyGo)
tinygo build -target=pico -o led-driver.uf2 ./cmd/led-driver
```

## Local Development Simulation with Process Compose

[Process Compose](https://github.com/F1bonacc1/process-compose) orchestrates multiple processes for local development, simulating the full production architecture on a single laptop.

### Simulated Components

| Process | Role | Description |
|---------|------|-------------|
| `ssh-bridge` | Server | SSH tunnel server (simulates remote host) |
| `led-driver-client` | Laptop | Client application connecting via SSH |
| `led-driver-pico` | Pico Device | LED driver with lneto over TAP (simulates Pico) |

### process-compose.yaml

```yaml
version: "0.5"

processes:
  # Simulates the remote SSH bridge server
  ssh-bridge:
    command: ./bin/ssh-bridge -listen :12022 -http :12023
    readiness_probe:
      http_get:
        host: localhost
        port: 12023
        path: /
      initial_delay_seconds: 1
      period_seconds: 2

  # Simulates the Pico device with TAP networking
  pico-simulator:
    command: sudo ./bin/led-driver --mode=pico --tap=tap0
    depends_on:
      ssh-bridge:
        condition: process_healthy
    environment:
      - LED_MATRIX_ROWS=32
      - LED_MATRIX_COLS=64

  # Client application (what runs on user's laptop in production)
  client:
    command: ./bin/led-client --bridge=localhost:12022
    depends_on:
      pico-simulator:
        condition: process_started
```

### Running the Simulation

```bash
# Start all components
process-compose up

# Start with TUI disabled (for CI)
process-compose up --tui=false

# View logs for specific process
process-compose logs pico-simulator
```

### Architecture in Simulation Mode

```
┌─────────────────────────────────────────────────────────────┐
│                     Developer Laptop                         │
│                                                              │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐  │
│  │    Client    │───▶│  ssh-bridge  │───▶│ pico-sim     │  │
│  │              │    │  :12022      │    │ (TAP: tap0)  │  │
│  └──────────────┘    └──────────────┘    └──────────────┘  │
│                                                 │            │
│                                          ┌──────▼─────┐     │
│                                          │ LED Matrix │     │
│                                          │ (Simulated)│     │
│                                          └────────────┘     │
└─────────────────────────────────────────────────────────────┘
```

This enables full end-to-end testing of the SSH tunnel, networking stack, and LED driver logic without any hardware.

## Device Provisioning

The networking stack also supports device provisioning - getting configuration onto fresh Pico devices.

### Provisioning Use Cases

| Use Case | Description |
|----------|-------------|
| WiFi Setup | Configure SSID and password for Pico W |
| Device Identity | Assign unique device ID, friendly name |
| Server Config | Set ssh-bridge host/port to connect to |
| LED Config | Matrix dimensions, brightness, color correction |
| Firmware Update | OTA updates via network |

### Provisioning Modes

#### 1. USB Serial (Initial Setup)

Fresh devices have no WiFi credentials. Use USB serial for first-time provisioning:

```
┌──────────┐  USB Serial   ┌──────────┐
│  Laptop  │──────────────▶│   Pico   │
│ (cli)    │  JSON config  │          │
└──────────┘               └──────────┘
```

```bash
# First-time provisioning via USB
./bin/led-provision --usb /dev/tty.usbmodem* --config device.json
```

#### 2. Network (Runtime Updates)

Once WiFi is configured, update settings over the network:

```
┌──────────┐     SSH      ┌───────────┐    TCP     ┌──────────┐
│  Laptop  │─────────────▶│ssh-bridge │───────────▶│   Pico   │
│ (cli)    │              │           │            │ (lneto)  │
└──────────┘              └───────────┘            └──────────┘
```

```bash
# Update config over network
./bin/led-client --bridge=myhost:12022 provision --config device.json
```

### Provisioning Protocol

Part of `internal/protocol`:

```go
// Provisioning message types
const (
    MsgSetWiFi      = 0x10  // SSID + password
    MsgSetDeviceID  = 0x11  // Unique identifier
    MsgSetServer    = 0x12  // Bridge host:port
    MsgSetLEDConfig = 0x13  // Matrix dimensions, brightness
    MsgGetConfig    = 0x14  // Read current config
    MsgReboot       = 0x15  // Restart device
    MsgOTAUpdate    = 0x16  // Firmware update
)
```

### Config Storage

On Pico, config stored in flash (survives reboot):

```go
// internal/config/config.go
type DeviceConfig struct {
    DeviceID   string
    WiFiSSID   string
    WiFiPass   string
    BridgeHost string
    BridgePort uint16
    MatrixRows uint8
    MatrixCols uint8
    Brightness uint8
}
```

### Updated Code Structure

```
├── internal/
│   ├── config/                      # Device configuration
│   │   ├── config.go                # Config struct and validation
│   │   ├── storage_pico.go          # //go:build pico - Flash storage
│   │   └── storage_desktop.go       # //go:build !pico - File-based
│   │
│   ├── provision/                   # Provisioning handlers
│   │   ├── provision.go             # Common provisioning logic
│   │   ├── usb.go                   # USB serial provisioning
│   │   └── network.go               # Network provisioning
```

## References

- https://github.com/soypat/lneto
- https://github.com/ddirect/ssh-bridge
