# knowledge-asset-smoke.ps1
#
# Smoke harness for the unify-wiki-and-ingested-documents change. Each
# scenario in section 10 of the change's tasks.md is mapped to an
# automated Go test under internal/knowledge/smoke_test.go. The script
# runs them in order and reports a pass/fail line per scenario, matching
# the pattern used by scripts/smoke/multi-tenant-smoke.ps1.
#
# Usage:
#   pwsh ./scripts/smoke/knowledge-asset-smoke.ps1
#
# Exits 0 on full success, non-zero with a diagnostic line on any failure.

[CmdletBinding()]
param(
    [string]$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
)

$ErrorActionPreference = "Stop"

function Write-Step($msg) {
    Write-Host "[knowledge-asset-smoke] $msg" -ForegroundColor Cyan
}

function Write-Fail($msg) {
    Write-Host "[knowledge-asset-smoke] FAIL: $msg" -ForegroundColor Red
}

$scenarios = @(
    @{ Id = "10.1"; Name = "Mixed-kind list (wiki + pdf)";              Run = "TestSmoke_10_1_ListReturnsMixedKinds" },
    @{ Id = "10.2"; Name = "Search across kinds";                       Run = "TestSmoke_10_2_SearchAcrossKinds" },
    @{ Id = "10.3"; Name = "Task description backlink extraction";     Run = "TestSmoke_10_3_TaskDescriptionBacklinkResolves" },
    @{ Id = "10.4"; Name = "Wiki save dispatches to IndexPipeline";     Run = "TestSmoke_10_4_WikiSaveDispatchesToIndexPipeline" },
    @{ Id = "10.5"; Name = "Materialize creates decoupled wiki asset";  Run = "TestSmoke_10_5_MaterializeCreatesDecoupledWikiAsset" },
    @{ Id = "10.6"; Name = "Review writeback skips non-wiki links";     Run = "TestSmoke_10_6_ReviewWritebackSkipsNonWikiLinks" }
)

$backendDir = Join-Path $RepoRoot "src-go"

Push-Location $backendDir
try {
    $anyFailed = $false
    foreach ($s in $scenarios) {
        Write-Step ("scenario {0}: {1}" -f $s.Id, $s.Name)
        & go test -count=1 ./internal/knowledge/ -run $s.Run
        if ($LASTEXITCODE -ne 0) {
            Write-Fail ("{0} failed" -f $s.Id)
            $anyFailed = $true
        } else {
            Write-Host ("[knowledge-asset-smoke] PASS {0}" -f $s.Id) -ForegroundColor Green
        }
    }
    if ($anyFailed) {
        exit 1
    }
} finally {
    Pop-Location
}

Write-Step "all scenarios passed"
