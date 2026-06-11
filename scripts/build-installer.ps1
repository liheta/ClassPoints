$ErrorActionPreference = "Stop"

$Root = Resolve-Path "$PSScriptRoot\.."
$InnoCandidates = @(
  "$env:LOCALAPPDATA\Programs\Inno Setup 6\ISCC.exe",
  "C:\Program Files (x86)\Inno Setup 6\ISCC.exe",
  "C:\Program Files\Inno Setup 6\ISCC.exe"
)
$ISCC = $InnoCandidates | Where-Object { Test-Path $_ } | Select-Object -First 1
if (-not $ISCC) {
  $Command = Get-Command ISCC.exe -ErrorAction SilentlyContinue
  if ($Command) {
    $ISCC = $Command.Source
  }
}

if (-not $ISCC) {
  Write-Host "未找到 Inno Setup 6。请先安装 Inno Setup 6，然后重新运行本脚本。"
  Write-Host ""
  Write-Host "如果你的电脑支持 winget，可以运行："
  Write-Host "winget install JRSoftware.InnoSetup"
  Write-Host ""
  Write-Host "下载地址：https://jrsoftware.org/isinfo.php"
  exit 1
}

Set-Location $Root
.\scripts\build.ps1

$InstallerOut = Join-Path $Root "release\installer"
if (Test-Path $InstallerOut) {
  Remove-Item -LiteralPath $InstallerOut -Recurse -Force
}
New-Item -ItemType Directory -Path $InstallerOut | Out-Null

& $ISCC (Join-Path $Root "installer\classpoints.iss")

Write-Host "安装文件已生成："
Get-ChildItem -Path $InstallerOut -Filter "*.exe" | Select-Object FullName,Length
