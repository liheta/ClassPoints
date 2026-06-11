$ErrorActionPreference = "Stop"

Write-Host "启动后端 API: http://127.0.0.1:8787"
Start-Process powershell -ArgumentList "-NoExit", "-Command", "cd '$PSScriptRoot\..'; go run .\cmd\classpoints"

Write-Host "启动前端开发服务: http://127.0.0.1:5173"
Set-Location "$PSScriptRoot\..\frontend"
npm install
npm run dev
