$ErrorActionPreference = 'Stop'
$ROOT = Split-Path -Parent $MyInvocation.MyCommand.Path | Split-Path -Parent
Write-Host "=== FreeCompute Backend (PowerShell) ===" -ForegroundColor Cyan
Write-Host "Root: $ROOT"

$TEMPDIR = $env:TEMP
$STORAGE = Join-Path $TEMPDIR "freecompute-storage"
New-Item -ItemType Directory -Force -Path $STORAGE | Out-Null
Write-Host "  Storage: $STORAGE (10GB per user)"

$env:FREECOMPUTE_GATEWAY_ADDR = ":8080"
$env:FREECOMPUTE_TUNNEL_TOKEN = "dev-token"
$env:FREECOMPUTE_DB_PATH = Join-Path $TEMPDIR "freecompute.db"
$env:FREECOMPUTE_RECORDING_DIR = Join-Path $TEMPDIR "freecompute-recordings"
$env:FREECOMPUTE_VM_STORAGEGB = "10"
$env:FREECOMPUTE_VM_ID = "local-vm-1"
$env:FREECOMPUTE_VM_REGION = "local"
$env:FREECOMPUTE_TUNNEL_ROUTES = '[{"id":"web","protocol":"http","target":"http://localhost:3000"},{"id":"api","protocol":"http","target":"http://localhost:8081"},{"id":"vm-http","protocol":"http","target":"http://localhost:8082"},{"id":"vm-ws","protocol":"websocket","target":"http://localhost:8082"},{"id":"vm-ssh","protocol":"ssh","target":"agent"},{"id":"vm-tcp","protocol":"tcp","target":"agent"},{"id":"vm-udp","protocol":"udp","target":"localhost:30000"},{"id":"rtc","protocol":"webrtc","listen":"0.0.0.0:8443"},{"id":"p2p-signal","protocol":"p2p","target":""},{"id":"game-udp","protocol":"udp","listen":"0.0.0.0:30001","target":"localhost:30001"}]'
$env:FREECOMPUTE_AGENT_GATEWAY_URL = "http://localhost:8080"
$env:FREECOMPUTE_AGENT_TOKEN = "dev-token"
$env:FREECOMPUTE_AGENT_ROUTES = '[{"id":"vm-ssh","target":"localhost:22"},{"id":"vm-tcp","target":"localhost:8082"}]'
$env:FREECOMPUTE_PROXY_UPSTREAM_IDLE_SECONDS = "30"
$env:FREECOMPUTE_PROXY_RESPONSE_HEADER_SECONDS = "5"
$env:FREECOMPUTE_PROXY_EXPECT_CONTINUE_SECONDS = "1"
$env:FREECOMPUTE_PROXY_MAX_IDLE_CONNS = "8192"
$env:FREECOMPUTE_PROXY_MAX_IDLE_CONNS_PER_HOST = "1024"
$env:FREECOMPUTE_PROXY_FLUSH_MS = "-1"
$env:FREECOMPUTE_RATE_LIMIT_RPM = "2000"
$env:FREECOMPUTE_MAX_CONNS_PER_USER = "100"
$env:FREECOMPUTE_DEFAULT_BROWSING_MODE = "casual"
$env:FREECOMPUTE_ADMIN_EMAIL = "admin"
$env:FREECOMPUTE_ADMIN_ROLE = "admin"
$env:FREECOMPUTE_TCP_CC_ALGO = "auto"
$env:FREECOMPUTE_TCP_BUFFER_SIZE = "8388608"
$env:FREECOMPUTE_UDP_BUFFER_SIZE = "16777216"
$env:FREECOMPUTE_MODERATION_LLM_URL = ""
$env:FREECOMPUTE_MODERATION_LLM_KEY = ""
$env:FREECOMPUTE_DNS_TTL_SECONDS = "600"
$env:FREECOMPUTE_DNS_MAX_ENTRIES = "4096"
$env:FREECOMPUTE_CDN_HOSTNAME = "cdn.freecompute.io"
$env:FREECOMPUTE_EDGE_HOSTNAME = "edge.freecompute.io"
$env:FREECOMPUTE_API_HOSTNAME = "api.freecompute.io"
$env:FREECOMPUTE_AGENT_DIAL_SECONDS = "5"
$env:FREECOMPUTE_AGENT_RECONNECT_SECONDS = "1"
$env:FREECOMPUTE_AGENT_INSECURE_SKIP_TLS = "0"

$GOMAXPROCS = go env GOMAXPROCS
if ($GOMAXPROCS) { $env:GOMAXPROCS = $GOMAXPROCS }

$GATEWAY_BIN = Join-Path $TEMPDIR "freecompute-gateway.exe"
$HOSTAGENT_BIN = Join-Path $TEMPDIR "freecompute-host-agent.exe"
$VMSETUP_BIN = Join-Path $TEMPDIR "freecompute-vm-setup.exe"

Write-Host "[1] Building gateway..."
Set-Location (Join-Path $ROOT "apps/gateway")
go build -a -buildvcs=false -o $GATEWAY_BIN ./cmd/gateway

Write-Host "[2] Building host-agent..."
Set-Location (Join-Path $ROOT "host-agent")
go build -a -buildvcs=false -o $HOSTAGENT_BIN ./cmd/host-agent

Write-Host "[3] Building vm-setup..."
Set-Location (Join-Path $ROOT "host-agent")
go build -a -buildvcs=false -o $VMSETUP_BIN ./cmd/vm-setup

Write-Host "  Checking port 8080..."
$existing = Get-NetTCPConnection -LocalPort 8080 -ErrorAction SilentlyContinue
if ($existing) {
    Write-Host "  WARNING: port 8080 is in use, killing existing process..."
    $ownerPids = $existing | Select-Object -ExpandProperty OwningProcess | Sort-Object -Unique
    foreach ($ownerPid in $ownerPids) {
        try {
            Stop-Process -Id $ownerPid -Force -ErrorAction SilentlyContinue
            Write-Host "    Killed PID $ownerPid"
        } catch {
            Write-Host "    Could not kill PID $($ownerPid): $($_.Exception.Message)"
        }
    }
    Start-Sleep -Seconds 2
    $still = Get-NetTCPConnection -LocalPort 8080 -ErrorAction SilentlyContinue
    if ($still) {
        Write-Host "  ERROR: port 8080 is still in use after cleanup"
        exit 1
    }
}

Write-Host "[4] Starting gateway..."
$gw = Start-Process -FilePath $GATEWAY_BIN -NoNewWindow -PassThru
Start-Sleep -Seconds 2

$healthUrl = "http://localhost:8080/healthz"
$maxAttempts = 3
$attempt = 0
$healthy = $false

while ($attempt -lt $maxAttempts -and -not $healthy) {
    $attempt++
    try {
        $r = Invoke-RestMethod -Uri $healthUrl -Method Head -TimeoutSec 5
        if (-not $gw.HasExited) {
            $healthy = $true
            Write-Host "  Gateway healthy (PID $($gw.Id))"
        } else {
            Write-Host "  WARNING: gateway process exited after health check (attempt $attempt/$maxAttempts)"
        }
    } catch {
        Write-Host "  WARNING: health check failed (attempt $attempt/$maxAttempts): $($_.Exception.Message)"
        if ($attempt -lt $maxAttempts) {
            Start-Sleep -Seconds 1
        }
    }
}

if (-not $healthy) {
    Write-Host "  ERROR: gateway failed to start after $maxAttempts attempts"
    if (-not $gw.HasExited) {
        Stop-Process -Id $gw.Id -Force -ErrorAction SilentlyContinue
    }
    exit 1
}

Write-Host "[5] Starting host-agent..."
$hostAgent = Start-Process -FilePath $HOSTAGENT_BIN -NoNewWindow -PassThru
Write-Host "  Host-agent started (PID $($hostAgent.Id))"

Write-Host "[6] Starting vm-setup..."
$vm = Start-Process -FilePath $VMSETUP_BIN -NoNewWindow -PassThru
Write-Host "  vm-setup started (PID $($vm.Id))"

Write-Host ""
Write-Host "=== All services running ===" -ForegroundColor Green
Write-Host "Gateway:      http://localhost:8080"
Write-Host "  Health:     http://localhost:8080/healthz"
Write-Host "  Capabilities: http://localhost:8080/capabilities"
Write-Host "Host-agent:   PID $($hostAgent.Id)"
Write-Host "vm-setup:     PID $($vm.Id)"
Write-Host "Storage:      $STORAGE (10GB per user)"
Write-Host ""
Write-Host "Press Ctrl+C to stop."

try {
    while ($true) { Start-Sleep -Seconds 1 }
} finally {
    Write-Host "`nStopping services..."
    Stop-Process -Id $gw.Id -Force -ErrorAction SilentlyContinue
    Stop-Process -Id $hostAgent.Id -Force -ErrorAction SilentlyContinue
    Stop-Process -Id $vm.Id -Force -ErrorAction SilentlyContinue
    Write-Host "All services stopped."
}
