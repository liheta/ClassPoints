# 班级积分系统

技术栈：

- 前端：Vue 3 + Vite
- 后端：Go + Gin
- 数据库：SQLite
- 部署：Windows / macOS 单机本地运行
- 运行模式：完全离线，浏览器访问本机服务

## 初版功能

- 登录页：本地老师名称登录，不联网。
- 我的班级页：创建班级弹窗，进入班级。
- 班级首页：学生数、班级总分、今日记录、第一名、近期记录。
- 课堂加分页：单人加减分，批量加分弹窗，写入 SQLite 后实时刷新。
- 学生管理页：新增学生弹窗，学生列表，删除学生。
- 积分规则页：新增规则弹窗，启用/停用规则，删除规则。
- 积分记录页：筛选记录，撤销有效记录。
- 排行榜页：全部排行、今日排行。
- 系统设置页：学校名称、备份目录、自动备份间隔、保留备份数量、立即备份。

## 开发运行

需要本机已安装 Go、Node.js 和 npm。

```powershell
.\scripts\dev.ps1
```

开发模式下：

- 后端 API：`http://127.0.0.1:8787`
- 前端页面：`http://127.0.0.1:5173`

## 构建 Windows 单机包

```powershell
.\scripts\build.ps1
```

构建产物：

- `release\classpoints\classpoints.exe`
- `release\classpoints-windows.zip`

双击 `classpoints.exe` 后，程序会启动本地服务并打开浏览器。默认地址：

```text
http://127.0.0.1:8787
```

## 构建 Windows 安装文件

安装文件使用 Inno Setup 6 生成。先安装 Inno Setup 6：

```text
https://jrsoftware.org/isinfo.php
```

然后运行：

```powershell
.\scripts\build-installer.ps1
```

构建产物：

```text
release\installer\ClassPointsSetup-0.1.0.exe
```

升级安装时，安装器会自动结束正在运行的 `classpoints.exe`，避免旧程序占用文件导致安装失败。

安装器会把程序安装到 Windows 应用目录，并创建开始菜单快捷方式；用户数据和 SQLite 数据库默认保存在：

```text
data\classpoints.db
data\backups\
```

运行日志默认写入：

```text
logs\classpoints.log
```

## 构建 macOS 应用包

可在 Windows 上交叉编译 macOS 应用包：

```powershell
.\scripts\build-macos.ps1
```

构建产物：

```text
release\classpoints-macos-arm64.app.tar.gz
release\classpoints-macos-amd64.app.tar.gz
```

- `arm64`：Apple Silicon Mac（M1/M2/M3/M4）
- `amd64`：Intel Mac

解压后得到 `ClassPoints.app`，可拖到“应用程序”或直接双击运行。程序会启动本地服务并自动打开浏览器，默认地址：

```text
http://127.0.0.1:8787
```

macOS 默认数据目录：

```text
~/Library/Application Support/ClassPoints/data/classpoints.db
~/Library/Application Support/ClassPoints/data/backups/
~/Library/Application Support/ClassPoints/logs/classpoints.log
```

如果 macOS 提示无法验证开发者，右键 `ClassPoints.app`，选择“打开”，再确认打开。

说明：当前脚本生成的是未签名 `.app.tar.gz`。正式的 `.dmg` 或 `.pkg` 安装器通常需要在 macOS 上完成签名、公证和打包。

## 数据位置

默认数据目录：

```text
data\classpoints.db
data\backups\
```

可通过环境变量修改：

```powershell
$env:CLASSPOINTS_DATA_DIR="D:\ClassPointsData"
$env:CLASSPOINTS_PORT="8788"
.\classpoints.exe
```

默认会在日志文件输出 SQL 日志，包含 SQL、参数、耗时和错误。需要关闭时：

```powershell
$env:CLASSPOINTS_SQL_LOG="0"
.\classpoints.exe
```
