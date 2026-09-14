# Refresh web/ from the Node repo's frontend.
#
# web/ is a COPY. The original in InvoiceMG-Mxnxn/Invoice-mg is still the source of truth and
# is still being edited - so the danger is not that this copy is stale, it is that someone
# edits BOTH and the two quietly diverge into a merge nobody wants to do by hand.
#
# The rule while the migration is sideways:
#
#   Edit the frontend in InvoiceMG-Mxnxn/Invoice-mg. Run this to bring the copy forward.
#   Never hand-edit web/.
#
# At cutover that reverses: web/ becomes the source of truth, this script is deleted, and the
# old repo is archived.
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
    Write-Host "Dry run. Files that differ (pass -Apply to copy):`n"
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
