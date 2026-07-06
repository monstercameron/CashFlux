@echo off
REM Windows exec wrapper so `GOOS=js GOARCH=wasm go test` can run wasm test
REM binaries here. Go looks for `go_js_wasm_exec` on PATH; the stock GOROOT copy
REM is a bash script Windows can't exec, and its path (C:\Program Files\Go) has a
REM space that breaks a naive -exec. This resolves GOROOT at runtime and hands the
REM wasm binary to node, matching what lib/wasm/go_js_wasm_exec does on Unix.
REM Put this directory on PATH before running wasm tests (see tools\wasm-test.ps1).
setlocal
for /f "delims=" %%i in ('go env GOROOT') do set "GR=%%i"
node --stack-size=8192 "%GR%\lib\wasm\wasm_exec_node.js" %*
