[CmdletBinding()]
param(
	[string]$MysqlHost = $(if ($env:NEW_API_MYSQL_HOST) { $env:NEW_API_MYSQL_HOST } else { '127.0.0.1' }),
	[int]$MysqlPort = $(if ($env:NEW_API_MYSQL_PORT) { [int]$env:NEW_API_MYSQL_PORT } else { 3306 }),
	[string]$MysqlUser = $(if ($env:NEW_API_MYSQL_USER) { $env:NEW_API_MYSQL_USER } else { 'root' }),
	[string]$MysqlPassword = $(if ($env:NEW_API_MYSQL_PASSWORD) { $env:NEW_API_MYSQL_PASSWORD } else { '123456' }),
	[string]$MysqlDatabase = $(if ($env:NEW_API_MYSQL_DATABASE) { $env:NEW_API_MYSQL_DATABASE } else { 'new_api' }),
	[int]$Port = $(if ($env:PORT) { [int]$env:PORT } else { 3008 }),
	[switch]$SkipStart
)

$ErrorActionPreference = 'Stop'

$projectRoot = Resolve-Path (Join-Path $PSScriptRoot '..')

function Resolve-RequiredCommand([string]$name) {
	$command = Get-Command $name -ErrorAction SilentlyContinue
	if ($command) {
		return $command.Source
	}
	throw "未找到命令: $name"
}

function Resolve-MysqlExecutable() {
	$command = Get-Command mysql -ErrorAction SilentlyContinue
	if ($command) {
		return $command.Source
	}

	$fallbacks = @(
		'E:\mysql-8.0.27-winx64\bin\mysql.exe',
		'C:\Program Files\MySQL\MySQL Server 8.0\bin\mysql.exe',
		'C:\Program Files\MySQL\MySQL Server 8.4\bin\mysql.exe'
	)
	foreach ($path in $fallbacks) {
		if (Test-Path $path) {
			return $path
		}
	}

	throw '未找到 mysql 客户端，请先安装 MySQL 并确保 mysql.exe 在 PATH 中。'
}

function Ensure-Database([string]$mysqlExe) {
	$mysqlArgs = @(
		"--host=$MysqlHost",
		"--port=$MysqlPort",
		"--user=$MysqlUser",
		'--protocol=TCP'
	)
	if ($MysqlPassword -ne '') {
		$mysqlArgs += "--password=$MysqlPassword"
	}

	$sql = "CREATE DATABASE IF NOT EXISTS ``$MysqlDatabase`` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;"
	& $mysqlExe @mysqlArgs -e $sql | Out-Null
}

function Stop-NewApiProcessOnPort() {
	$listener = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
	if (-not $listener) {
		return
	}

	$process = Get-CimInstance Win32_Process -Filter "ProcessId=$($listener.OwningProcess)"
	if (-not $process) {
		return
	}

	$commandLine = if ($process.CommandLine) { $process.CommandLine } else { '' }
	$isNewApiProcess = $process.Name -in @('go.exe', 'main.exe') -or $commandLine -match 'new-api|run main.go|go-build'
	if (-not $isNewApiProcess) {
		throw "端口 $Port 已被进程 $($process.Name)($($process.ProcessId)) 占用，脚本不会自动停止非 new-api 进程。"
	}

	Write-Host "停止占用端口 $Port 的旧进程: $($process.Name)($($process.ProcessId))" -ForegroundColor Yellow
	Stop-Process -Id $process.ProcessId -Force
	Start-Sleep -Seconds 1
}

$goExe = Resolve-RequiredCommand 'go'
$mysqlExe = Resolve-MysqlExecutable

Ensure-Database -mysqlExe $mysqlExe
if (-not $SkipStart) {
	Stop-NewApiProcessOnPort
}

$env:SQL_DSN = "$($MysqlUser):$($MysqlPassword)@tcp($($MysqlHost):$($MysqlPort))/$($MysqlDatabase)?charset=utf8mb4&parseTime=true&loc=Local"
Remove-Item Env:LOG_SQL_DSN -ErrorAction SilentlyContinue
Remove-Item Env:SQLITE_PATH -ErrorAction SilentlyContinue
Remove-Item Env:PORT -ErrorAction SilentlyContinue
$env:PORT = "$Port"

Write-Host "new-api 将使用 MySQL 启动" -ForegroundColor Green
Write-Host "Project : $projectRoot"
Write-Host "MySQL   : $($MysqlUser)@$($MysqlHost):$($MysqlPort)/$($MysqlDatabase)"
Write-Host "Port    : $Port"

if ($SkipStart) {
	return
}

Push-Location $projectRoot
try {
	& $goExe run main.go
} finally {
	Pop-Location
}
