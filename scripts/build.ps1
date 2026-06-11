$ErrorActionPreference = "Stop"

$Root = Resolve-Path "$PSScriptRoot\.."
$Release = Join-Path $Root "release"
$PackageDir = Join-Path $Release "classpoints"
$ExePath = Join-Path $PackageDir "classpoints.exe"
$ZipPath = Join-Path $Release "classpoints-windows.zip"

if (Test-Path $PackageDir) {
  Remove-Item -LiteralPath $PackageDir -Recurse -Force
}
New-Item -ItemType Directory -Path $PackageDir | Out-Null

Write-Host "安装前端依赖..."
Set-Location (Join-Path $Root "frontend")
npm install

Write-Host "构建 Vue 前端..."
npm run build

Write-Host "编译 Go 单机程序..."
Set-Location $Root
go mod tidy
go build -ldflags "-s -w -H=windowsgui" -o $ExePath .\cmd\classpoints

Write-Host "写入启动说明..."
@"
班级积分系统

1. 双击 classpoints.exe
2. 程序会启动本地服务并自动打开浏览器
3. 数据保存在 classpoints.exe 同级目录的 data\classpoints.db
4. 自动备份保存在 classpoints.exe 同级目录的 data\backups
5. 运行日志保存在 classpoints.exe 同级目录的 logs\classpoints.log

如端口 8787 被占用，可在 PowerShell 中运行：
`$env:CLASSPOINTS_PORT="8788"; .\classpoints.exe

升级安装时，安装程序会自动关闭正在运行的 classpoints.exe。
"@ | Set-Content -Path (Join-Path $PackageDir "使用说明.txt") -Encoding UTF8

if (Test-Path $ZipPath) {
  Remove-Item -LiteralPath $ZipPath -Force
}
Compress-Archive -Path (Join-Path $PackageDir "*") -DestinationPath $ZipPath

Write-Host "完成：$ZipPath"
