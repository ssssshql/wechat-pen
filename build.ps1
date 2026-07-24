# 一键构建并嵌入前端
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

Write-Host "==> building web"
Push-Location web
npm install
npm run build
Pop-Location

Write-Host "==> embedding spa"
if (Test-Path server\spa) { Remove-Item server\spa -Recurse -Force }
New-Item -ItemType Directory -Path server\spa | Out-Null
Copy-Item -Path web\dist\* -Destination server\spa -Recurse -Force

Write-Host "==> go build"
go test ./converter
go build -o wechat-pen.exe .

Write-Host ""
Write-Host "Done. Run: .\wechat-pen.exe serve"
Write-Host "Open:  http://127.0.0.1:8080"
