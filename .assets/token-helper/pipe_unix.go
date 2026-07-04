// pipe_unix.go
//go:build !windows

package main

import (
    "errors"
    "os"
    "os/signal"
    "syscall"
    "time"
)

func runServer() {
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

    reqPath := "/tmp/" + pipeName + ".req"
    respPath := "/tmp/" + pipeName + ".resp"

    // Clean up any stale FIFOs
    os.Remove(reqPath)
    os.Remove(respPath)

    if err := syscall.Mkfifo(reqPath, 0666); err != nil && !errors.Is(err, syscall.EEXIST) {
        logError("mkfifo failed on '" + reqPath + "': " + err.Error())
        return
    }
    if err := syscall.Mkfifo(respPath, 0666); err != nil && !errors.Is(err, syscall.EEXIST) {
        logError("mkfifo failed on '" + respPath + "': " + err.Error())
        os.Remove(reqPath)
        return
    }

    // Signal handler — unblocks the blocking read by writing a byte
    go func() {
        <-sigCh
        gRunning.Store(false)
        f, err := os.OpenFile(reqPath, os.O_WRONLY|syscall.O_NONBLOCK, 0)
        if err == nil {
            f.Write([]byte("x"))
            f.Close()
        }
    }()

    for gRunning.Load() {
        // Open request pipe non-blocking so we can poll gRunning while idle
        rfd, err := os.OpenFile(reqPath, os.O_RDONLY|syscall.O_NONBLOCK, 0)
        if err != nil {
            if !gRunning.Load() {
                break
            }
            time.Sleep(200 * time.Millisecond)
            continue
        }

        // Switch to blocking mode — wait for an actual client to write
        syscall.SetNonblock(int(rfd.Fd()), false)

        buf := make([]byte, 256)
        n, _ := rfd.Read(buf)
        rfd.Close()

        if n <= 0 {
            continue
        }
        if !gRunning.Load() {
            break
        }

        // Compute payload only when asked
        payload := computeFinalPayload()
        response := payload
        if response == "" {
            response = "ERROR"
        }
        response += "\n"

        wfd, err := os.OpenFile(respPath, os.O_WRONLY, 0)
        if err != nil {
            logError("Failed to open response FIFO '" + respPath + "' for write: " + err.Error())
            continue
        }
        _, err = wfd.Write([]byte(response))
        if err != nil {
            logError("write() to response FIFO failed: " + err.Error())
        }
        wfd.Close()
    }

    os.Remove(reqPath)
    os.Remove(respPath)
}