param([string]$CompilerPath)

$ErrorActionPreference = 'Stop'
$project = Split-Path -Parent $PSScriptRoot
$script = Join-Path $project 'installer\VWRD.nsi'
$required = @(
    'VWRD-release\VWRD.exe',
    'VWRD-release\VWRD-MCP.exe',
    'LICENSE',
    'branding\VWRD-logo.svg',
    'branding\VWRD-logo.png',
    'branding\VWRD.ico',
    'docs\workflow.svg'
)
foreach ($relative in $required) {
    $path = Join-Path $project $relative
    if (-not (Test-Path -LiteralPath $path)) { throw "Missing installer input: $path" }
}
if (-not $CompilerPath) {
    $command = Get-Command makensis.exe -ErrorAction SilentlyContinue
    if ($command) { $CompilerPath = $command.Source }
}
if (-not $CompilerPath) {
    $candidates = @(
        (Join-Path $env:ProgramFiles 'NSIS\makensis.exe'),
        (Join-Path ${env:ProgramFiles(x86)} 'NSIS\makensis.exe'),
        (Join-Path $env:TEMP 'vwrd-nsis-portable\nsis-3.13\makensis.exe')
    )
    $CompilerPath = $candidates | Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1
}
if (-not $CompilerPath -or -not (Test-Path -LiteralPath $CompilerPath)) {
    throw 'NSIS makensis.exe was not found. Pass -CompilerPath to build-installer.ps1.'
}
$guiBytes = (Get-Item -LiteralPath (Join-Path $project 'VWRD-release\VWRD.exe')).Length
$mcpBytes = (Get-Item -LiteralPath (Join-Path $project 'VWRD-release\VWRD-MCP.exe')).Length
& $CompilerPath "/DGUI_BYTES=$guiBytes" "/DMCP_BYTES=$mcpBytes" /INPUTCHARSET UTF8 $script
if ($LASTEXITCODE -ne 0) { throw "NSIS compilation failed with exit code $LASTEXITCODE" }
Write-Output (Join-Path $project 'VWRD-Setup.exe')
