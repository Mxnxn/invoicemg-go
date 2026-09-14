# Refresh web/ from the Node repo's frontend.
#
# ONE-TIME IMPORT, NOT AN ONGOING SYNC.
#
# web/ was seeded from InvoiceMG-Mxnxn/Invoice-mg and is now THE SOURCE OF TRUTH: all work
# happens in this repository. This script exists only to pull across anything that was left
# behind in the original before the switch.
#
# It MIRRORS (/MIR), so running it would DELETE every change made in web/ since the import.
# That is why it defaults to a dry run and why -Apply has to be typed deliberately. If you are
# not certain you want the old repo to win, do not pass it.
#
#   pwsh scripts/sync-web.ps1            # show what would change
#   pwsh scripts/sync-web.ps1 -Apply     # actually copy

param(
    [switch]$Apply,
    [string]$Source = "D:\JS\InvoiceMG-Mxnxn\Invoice-mg"
)

$ErrorActionPreference = "Stop"
$destination = Join-Path $PSScriptRoot "..\web"

if (-not (Test-Path $Source)) {
    Write-Error "Source not found: $Source"
}

# /MIR mirrors, so a file deleted upstream is deleted here too - without it the copy
# accumulates files the original no longer has, which is its own kind of drift.
# node_modules, build output and .git are excluded: they are derived, enormous, or belong to a
# different repository.
$common = @(
    $Source, $destination, "/MIR",
    "/XD", "node_modules", "build", "dist", ".git",
    "/XF", ".env",
    "/NFL", "/NDL", "/NJH", "/NJS", "/NP"
)

if (-not $Apply) {
    Write-Host "Dry run. -Apply would OVERWRITE web/ with the listed files and delete anything not in the source:`n"
    & robocopy @common /L
    Write-Host "`nNothing was changed."
    exit 0
}

& robocopy @common
# robocopy exits 0-7 for success; 8 and above is a real failure.
if ($LASTEXITCODE -ge 8) { Write-Error "robocopy failed with $LASTEXITCODE" }

$count = (Get-ChildItem $destination -Recurse -File | Measure-Object).Count
Write-Host "web/ synced from $Source - $count files."
Write-Host "Remember: edit the original, never this copy."
