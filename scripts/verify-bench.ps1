param(
    [string]$ThresholdPath = (Join-Path $PSScriptRoot "benchmark-thresholds.json"),
    [string]$BenchTime = $env:GOARK_ORM_BENCHTIME,
    [switch]$EnforceTime
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($BenchTime)) {
    $BenchTime = "1s"
}

if (-not (Test-Path -LiteralPath $ThresholdPath)) {
    throw "benchmark threshold file not found: $ThresholdPath"
}

$thresholds = Get-Content -LiteralPath $ThresholdPath -Raw | ConvertFrom-Json
if ($null -eq $thresholds -or $thresholds.Count -eq 0) {
    throw "benchmark threshold file is empty: $ThresholdPath"
}

$benchNames = @()
foreach ($threshold in $thresholds) {
    $benchNames += [regex]::Escape([string]$threshold.name)
}
$benchPattern = "^(" + ($benchNames -join "|") + ")$"
$benchTimeArg = "-benchtime=" + $BenchTime

$env:GOWORK = "off"
$output = & go test -run '^$' -bench $benchPattern $benchTimeArg -benchmem ./internal/runtime 2>&1
$exitCode = $LASTEXITCODE
$output | ForEach-Object { Write-Output $_ }
if ($exitCode -ne 0) {
    throw "go benchmark command failed with exit code $exitCode"
}

$results = @{}
$linePattern = '^(Benchmark\S+)-\d+\s+\d+\s+([0-9.]+)\s+ns/op\s+([0-9.]+)\s+B/op\s+([0-9.]+)\s+allocs/op'
foreach ($line in $output) {
    $text = [string]$line
    $match = [regex]::Match($text, $linePattern)
    if (-not $match.Success) {
        continue
    }
    $results[$match.Groups[1].Value] = [pscustomobject]@{
        NsOp     = [double]$match.Groups[2].Value
        BOp      = [double]$match.Groups[3].Value
        AllocsOp = [double]$match.Groups[4].Value
    }
}

$failures = New-Object System.Collections.Generic.List[string]
foreach ($threshold in $thresholds) {
    $name = [string]$threshold.name
    if (-not $results.ContainsKey($name)) {
        $failures.Add("missing benchmark result: $name")
        continue
    }
    $actual = $results[$name]
    if ($actual.BOp -gt [double]$threshold.max_b_op) {
        $failures.Add("$name B/op $($actual.BOp) > $($threshold.max_b_op)")
    }
    if ($actual.AllocsOp -gt [double]$threshold.max_allocs_op) {
        $failures.Add("$name allocs/op $($actual.AllocsOp) > $($threshold.max_allocs_op)")
    }
    if ($EnforceTime -and $actual.NsOp -gt [double]$threshold.max_ns_op) {
        $failures.Add("$name ns/op $($actual.NsOp) > $($threshold.max_ns_op)")
    }
}

if ($failures.Count -gt 0) {
    $failures | ForEach-Object { Write-Error $_ }
    throw "benchmark thresholds failed"
}

Write-Output "benchmark thresholds passed"
