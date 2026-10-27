param(
    [string]$ChromePath = 'C:\Program Files\Google\Chrome\Application\chrome.exe'
)

$ErrorActionPreference = 'Stop'
$project = Split-Path -Parent $PSScriptRoot
$svg = Join-Path $project 'internal\gui\frontend\logo.svg'
$branding = Join-Path $project 'branding'
$svgCopy = Join-Path $branding 'VWRD-logo.svg'
$png = Join-Path $branding 'VWRD-logo.png'
$ico = Join-Path $branding 'VWRD.ico'
$syso = Join-Path $project 'cmd\sar\rsrc.syso'
$manifest = Join-Path $project 'cmd\sar\sar.manifest'
$profile = Join-Path $env:TEMP 'vwrd-icon-render'

if (-not (Test-Path -LiteralPath $ChromePath)) { throw "Chrome not found: $ChromePath" }
New-Item -ItemType Directory -Path $branding -Force | Out-Null
Copy-Item -LiteralPath $svg -Destination $svgCopy -Force
if (Test-Path -LiteralPath $png) { Remove-Item -LiteralPath $png -Force }
$uri = [System.Uri]::new((Resolve-Path -LiteralPath $svg).Path).AbsoluteUri
& $ChromePath --headless=new --disable-gpu --no-first-run --no-default-browser-check --hide-scrollbars --default-background-color=00000000 --user-data-dir=$profile --screenshot=$png --window-size=256,256 $uri | Out-Null
for ($attempt = 0; $attempt -lt 50 -and -not (Test-Path -LiteralPath $png); $attempt++) {
    Start-Sleep -Milliseconds 200
}
if (-not (Test-Path -LiteralPath $png)) { throw 'Chrome did not render the SVG icon.' }

Add-Type -AssemblyName System.Drawing
$source = [System.Drawing.Bitmap]::FromFile($png)
try {
    if ($source.Width -ne 256 -or $source.Height -ne 256) { throw 'The rendered icon must be 256 x 256 pixels.' }
    $images = @()
    foreach ($size in @(16, 24, 32, 48, 64, 128, 256)) {
        $bitmap = [System.Drawing.Bitmap]::new($size, $size, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
        $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
        $stream = [System.IO.MemoryStream]::new()
        try {
            $graphics.Clear([System.Drawing.Color]::Transparent)
            $graphics.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
            $graphics.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
            $graphics.CompositingQuality = [System.Drawing.Drawing2D.CompositingQuality]::HighQuality
            $graphics.DrawImage($source, 0, 0, $size, $size)
            $bitmap.Save($stream, [System.Drawing.Imaging.ImageFormat]::Png)
            $images += [pscustomobject]@{ Size = $size; Bytes = $stream.ToArray() }
        } finally {
            $stream.Dispose()
            $graphics.Dispose()
            $bitmap.Dispose()
        }
    }
} finally {
    $source.Dispose()
}

$writer = [System.IO.BinaryWriter]::new([System.IO.File]::Create($ico))
try {
    $writer.Write([uint16]0)
    $writer.Write([uint16]1)
    $writer.Write([uint16]$images.Count)
    $offset = 6 + 16 * $images.Count
    foreach ($item in $images) {
        $writer.Write([byte]($item.Size -band 255))
        $writer.Write([byte]($item.Size -band 255))
        $writer.Write([byte]0)
        $writer.Write([byte]0)
        $writer.Write([uint16]1)
        $writer.Write([uint16]32)
        $writer.Write([uint32]$item.Bytes.Length)
        $writer.Write([uint32]$offset)
        $offset += $item.Bytes.Length
    }
    foreach ($item in $images) { $writer.Write([byte[]]$item.Bytes) }
} finally {
    $writer.Dispose()
}

& go run github.com/akavel/rsrc@v0.10.2 -arch amd64 -manifest $manifest -ico $ico -o $syso
if ($LASTEXITCODE -ne 0) { throw 'Failed to embed the Windows icon and manifest.' }
Write-Output "Created $svgCopy, $png and $ico; updated $syso"
