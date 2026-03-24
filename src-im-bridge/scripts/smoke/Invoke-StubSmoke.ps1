param(
    [Parameter(Mandatory = $true)]
    [ValidateSet("feishu", "slack", "dingtalk", "telegram", "discord")]
    [string]$Platform,

    [int]$Port = 7780,

    [string]$FixturePath
)

$ErrorActionPreference = "Stop"

if (-not $FixturePath) {
    $FixturePath = Join-Path $PSScriptRoot "fixtures\$Platform.json"
}

if (-not (Test-Path $FixturePath)) {
    throw "Fixture not found: $FixturePath"
}

$baseUrl = "http://127.0.0.1:$Port"
$payload = Get-Content $FixturePath -Raw

Invoke-RestMethod -Method Delete -Uri "$baseUrl/test/replies" | Out-Null
Invoke-RestMethod -Method Post -Uri "$baseUrl/test/message" -ContentType "application/json" -Body $payload | Out-Null
Start-Sleep -Milliseconds 250

$replies = Invoke-RestMethod -Method Get -Uri "$baseUrl/test/replies"
if (-not $replies) {
    throw "No replies captured for platform $Platform on port $Port."
}

$firstReply = @($replies)[0]
if ([string]::IsNullOrWhiteSpace($firstReply.content)) {
    throw "First reply for platform $Platform is empty."
}

$replies | ConvertTo-Json -Depth 5
