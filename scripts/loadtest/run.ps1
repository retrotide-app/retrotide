[CmdletBinding()]
param(
    [string]$BaseUrl = 'http://localhost:4333',
    [switch]$NoDashboard
)

if (-not (Get-Command k6 -ErrorAction SilentlyContinue))
{
    Write-Host 'k6 is not installed. Install it with: winget install k6 --source winget' -ForegroundColor Red
    exit 1
}

try
{
    Invoke-WebRequest -Uri "$BaseUrl/livez" -TimeoutSec 5 -UseBasicParsing | Out-Null
}
catch
{
    Write-Host "No server responding at $BaseUrl/livez: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host "Start it with 'go run .' (the copied .env binds 127.0.0.1:4333) or 'docker compose --profile app up -d --build' (host 4333)."
    exit 1
}

if ($NoDashboard)
{
    $env:K6_WEB_DASHBOARD = 'false'
}
else
{
    $env:K6_WEB_DASHBOARD = 'true'
    $env:K6_WEB_DASHBOARD_OPEN = 'true'
}

Write-Host "Load testing $BaseUrl..." -ForegroundColor Cyan
k6 run --env "BASE_URL=$BaseUrl" (Join-Path $PSScriptRoot 'read-endpoints.js')

exit $LASTEXITCODE
