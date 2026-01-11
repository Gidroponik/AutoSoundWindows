@echo off
echo Building AutoSound...

REM Устанавливаем зависимости
echo Installing dependencies...
go mod tidy

REM Генерируем ресурсы (требуется rsrc или go-winres)
REM Если rsrc не установлен:
REM   go install github.com/akavel/rsrc@latest
REM Если go-winres не установлен:
REM   go install github.com/tc-hib/go-winres@latest

echo Generating resources...
if exist rsrc.syso del rsrc.syso

REM Попробуем использовать go-winres если доступен
where go-winres >nul 2>&1
if %ERRORLEVEL% EQU 0 (
    go-winres make --in winres.json
) else (
    REM Попробуем использовать rsrc
    where rsrc >nul 2>&1
    if %ERRORLEVEL% EQU 0 (
        rsrc -manifest AutoSound.manifest -o rsrc.syso
    ) else (
        echo Warning: Neither go-winres nor rsrc found. Building without manifest.
        echo Install with: go install github.com/akavel/rsrc@latest
    )
)

REM Собираем приложение
echo Building application...
go build -ldflags="-H windowsgui -s -w" -o AutoSound.exe .

if %ERRORLEVEL% EQU 0 (
    echo.
    echo Build successful! Run AutoSound.exe to start the application.
) else (
    echo.
    echo Build failed!
)

pause
