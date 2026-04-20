# --- Card-delivery smoke fixtures (spec 1D) ---
# feishu-workflow-with-card.json  — Trace A: success card after /workflow echo-with-card
#   expect_card: { platform: "feishu", title_contains: "echo", status: "success", actions_min: 1 }
# feishu-workflow-http-fail.json  — Trace C: failure card after /workflow http-fail-demo
#   expect_card: { platform: "feishu", title_contains: "执行失败", status: "failed", fields_contains: ["失败节点", "Run"] }
# To run: Invoke-StubSmoke.ps1 -Platform feishu -FixturePath ./fixtures/feishu-workflow-with-card.json
# Assertion runner for expect_card is a future follow-up; for now verify the POST reaches the bridge manually.

param(
    [Parameter(Mandatory = $true)]
    [ValidateSet("feishu", "slack", "dingtalk", "telegram", "discord", "wecom", "qq", "qqbot")]
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
