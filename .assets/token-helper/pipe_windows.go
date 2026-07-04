// pipe_windows.go
//go:build windows

package main

import (
    "os"
    "os/signal"
    "syscall"
    "time"

    "golang.org/x/sys/windows"
)

func runServer() {
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

    pipePath := `\\.\pipe\` + pipeName
    utf16Path, err := windows.UTF16PtrFromString(pipePath)
    if err != nil {
        logError("UTF16PtrFromString failed: " + err.Error())
        return
    }

    // Signal handler — unblocks ConnectNamedPipe by connecting as a client
    go func() {
        <-sigCh
        gRunning.Store(false)
        h, err := windows.CreateFile(
            utf16Path,
            windows.GENERIC_WRITE,
            0, nil,
            windows.OPEN_EXISTING,
            0, 0)
        if err == nil {
            windows.CloseHandle(h)
        }
    }()

    for gRunning.Load() {
        pipe, err := windows.CreateNamedPipe(
            utf16Path,
            windows.PIPE_ACCESS_OUTBOUND,
            windows.PIPE_TYPE_BYTE|windows.PIPE_WAIT,
            windows.PIPE_UNLIMITED_INSTANCES,
            4096, 4096,
            0, nil)
        if err != nil {
            logError("CreateNamedPipe failed: " + err.Error())
            if !gRunning.Load() {
                break
            }
            time.Sleep(time.Second)
            continue
        }

        // Blocks until a client connects
        err = windows.ConnectNamedPipe(pipe, nil)
        if err != nil && err != windows.ERROR_PIPE_CONNECTED {
            if !gRunning.Load() {
                windows.CloseHandle(pipe)
                break
            }
            logError("ConnectNamedPipe failed: " + err.Error())
            windows.CloseHandle(pipe)
            continue
        }

        if !gRunning.Load() {
            windows.CloseHandle(pipe)
            break
        }

        // Compute payload only when asked
        payload := computeFinalPayload()
        response := payload
        if response == "" {
            response = "ERROR"
        }
        response += "\n"

        var written uint32
        err = windows.WriteFile(pipe, []byte(response), &written, nil)
        if err != nil {
            logError("WriteFile to named pipe failed: " + err.Error())
        }

        windows.FlushFileBuffers(pipe)
        windows.DisconnectNamedPipe(pipe)
        windows.CloseHandle(pipe)
    }
}