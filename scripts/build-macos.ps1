$ErrorActionPreference = "Stop"

$Root = Resolve-Path "$PSScriptRoot\.."
$Release = Join-Path $Root "release"
$MacRelease = Join-Path $Release "macos"
$Version = "0.1.0"
$AppName = "ClassPoints"
$DisplayName = "班级积分系统"

function Write-AsciiField($Header, $Offset, $Length, $Value) {
  $Bytes = [System.Text.Encoding]::ASCII.GetBytes($Value)
  $Count = [Math]::Min($Bytes.Length, $Length)
  [Array]::Copy($Bytes, 0, $Header, $Offset, $Count)
}

function Write-OctalField($Header, $Offset, $Length, [Int64]$Value) {
  $Text = [Convert]::ToString($Value, 8).PadLeft($Length - 1, "0") + "`0"
  Write-AsciiField $Header $Offset $Length $Text
}

function Add-TarEntry($Stream, $EntryName, [byte[]]$Content, [int]$Mode, [char]$TypeFlag) {
  $Header = New-Object byte[] 512
  $Name = $EntryName.Replace("\", "/")
  if ($Name.Length -gt 100) {
    throw "tar entry path is too long: $Name"
  }

  Write-AsciiField $Header 0 100 $Name
  Write-OctalField $Header 100 8 $Mode
  Write-OctalField $Header 108 8 0
  Write-OctalField $Header 116 8 0
  Write-OctalField $Header 124 12 $Content.Length
  $MTime = [int][double]::Parse((Get-Date -UFormat %s), [Globalization.CultureInfo]::InvariantCulture)
  Write-OctalField $Header 136 12 $MTime
  for ($Index = 148; $Index -lt 156; $Index++) {
    $Header[$Index] = 32
  }
  $Header[156] = [byte][char]$TypeFlag
  Write-AsciiField $Header 257 6 "ustar"
  Write-AsciiField $Header 263 2 "00"

  $Checksum = 0
  foreach ($Byte in $Header) {
    $Checksum += $Byte
  }
  $ChecksumText = [Convert]::ToString($Checksum, 8).PadLeft(6, "0") + "`0 "
  Write-AsciiField $Header 148 8 $ChecksumText

  $Stream.Write($Header, 0, $Header.Length)
  if ($Content.Length -gt 0) {
    $Stream.Write($Content, 0, $Content.Length)
    $Padding = (512 - ($Content.Length % 512)) % 512
    if ($Padding -gt 0) {
      $Stream.Write((New-Object byte[] $Padding), 0, $Padding)
    }
  }
}

function New-TarGz($SourceDir, $TargetPath) {
  if (Test-Path $TargetPath) {
    Remove-Item -LiteralPath $TargetPath -Force
  }

  $FileStream = [System.IO.File]::Create($TargetPath)
  try {
    $GzipStream = [System.IO.Compression.GzipStream]::new($FileStream, [System.IO.Compression.CompressionLevel]::Optimal)
    try {
      $RootPath = (Resolve-Path $SourceDir).Path
      $Items = Get-ChildItem -LiteralPath $RootPath -Recurse -Force | Sort-Object FullName
      foreach ($Item in $Items) {
        $Relative = $Item.FullName.Substring($RootPath.Length).TrimStart("\", "/").Replace("\", "/")
        if ($Item.PSIsContainer) {
          Add-TarEntry $GzipStream ($Relative + "/") ([byte[]]::new(0)) 493 "5"
          continue
        }

        $Mode = 420
        if ($Relative -eq "$AppName.app/Contents/MacOS/classpoints") {
          $Mode = 493
        }
        Add-TarEntry $GzipStream $Relative ([System.IO.File]::ReadAllBytes($Item.FullName)) $Mode "0"
      }
      $GzipStream.Write(([byte[]]::new(1024)), 0, 1024)
    } finally {
      $GzipStream.Dispose()
    }
  } finally {
    $FileStream.Dispose()
  }
}

if (Test-Path $MacRelease) {
  Remove-Item -LiteralPath $MacRelease -Recurse -Force
}
New-Item -ItemType Directory -Path $MacRelease | Out-Null

Write-Host "安装前端依赖..."
Set-Location (Join-Path $Root "frontend")
npm install

Write-Host "构建 Vue 前端..."
npm run build

Write-Host "整理 Go 依赖..."
Set-Location $Root
go mod tidy

$Targets = @(
  @{ Arch = "arm64"; Label = "Apple Silicon" },
  @{ Arch = "amd64"; Label = "Intel" }
)

foreach ($Target in $Targets) {
  $Arch = $Target.Arch
  $PackageDir = Join-Path $MacRelease "classpoints-macos-$Arch"
  $AppDir = Join-Path $PackageDir "$AppName.app"
  $ContentsDir = Join-Path $AppDir "Contents"
  $MacOSDir = Join-Path $ContentsDir "MacOS"
  $ResourcesDir = Join-Path $ContentsDir "Resources"
  $BinaryPath = Join-Path $MacOSDir "classpoints"
  $ArchivePath = Join-Path $Release "classpoints-macos-$Arch.app.tar.gz"

  New-Item -ItemType Directory -Path $MacOSDir | Out-Null
  New-Item -ItemType Directory -Path $ResourcesDir | Out-Null

  Write-Host "编译 macOS $($Target.Label) 程序..."
  $env:GOOS = "darwin"
  $env:GOARCH = $Arch
  $env:CGO_ENABLED = "0"
  go build -ldflags "-s -w" -o $BinaryPath .\cmd\classpoints

  @"
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleDevelopmentRegion</key>
  <string>zh_CN</string>
  <key>CFBundleDisplayName</key>
  <string>$DisplayName</string>
  <key>CFBundleExecutable</key>
  <string>classpoints</string>
  <key>CFBundleIdentifier</key>
  <string>local.classpoints.app</string>
  <key>CFBundleInfoDictionaryVersion</key>
  <string>6.0</string>
  <key>CFBundleName</key>
  <string>$AppName</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleShortVersionString</key>
  <string>$Version</string>
  <key>CFBundleVersion</key>
  <string>$Version</string>
  <key>LSMinimumSystemVersion</key>
  <string>11.0</string>
  <key>NSHighResolutionCapable</key>
  <true/>
</dict>
</plist>
"@ | Set-Content -Path (Join-Path $ContentsDir "Info.plist") -Encoding UTF8

  @"
班级积分系统 macOS 版

1. 解压 classpoints-macos-$Arch.app.tar.gz
2. 将 ClassPoints.app 拖到“应用程序”，或直接双击运行
3. 程序会启动本地服务并自动打开浏览器
4. 默认访问地址：http://127.0.0.1:8787
5. 数据保存在：~/Library/Application Support/ClassPoints/data/classpoints.db
6. 自动备份保存在：~/Library/Application Support/ClassPoints/data/backups
7. 运行日志保存在：~/Library/Application Support/ClassPoints/logs/classpoints.log

如果 macOS 提示无法验证开发者：
右键 ClassPoints.app，选择“打开”，再确认打开。

如端口 8787 被占用，可在终端中运行：
CLASSPOINTS_PORT=8788 /Applications/ClassPoints.app/Contents/MacOS/classpoints
"@ | Set-Content -Path (Join-Path $PackageDir "README-macOS.txt") -Encoding UTF8

  Write-Host "打包 macOS $($Target.Label) 应用..."
  New-TarGz $PackageDir $ArchivePath
}

Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue
Remove-Item Env:\CGO_ENABLED -ErrorAction SilentlyContinue

Write-Host "macOS 安装包已生成："
Get-ChildItem -Path $Release -Filter "classpoints-macos-*.app.tar.gz" | Select-Object FullName,Length
