package main

import (
	"bufio"
	"log"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/rogpeppe/go-internal/lockedfile"
)

func main() {
	filename := filepath.Join(os.TempDir(), "LiveProcesses.txt")
	port := ":" + strconv.Itoa(rand.Intn(10_000))
	listener, err := net.Listen("tcp", port)
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	err = subscribeToNetwork(filename, port)
	if err != nil {
		panic(err)
	}

	sigChannel := make(chan os.Signal, 1)
	signal.Notify(sigChannel, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChannel
		log.Printf("Shutting down process on port %v...", port)
		err := unsubscribeFromNetwork(filename, port)
		if err != nil {
			log.Printf("Error shutting down process with error: %v", err)
		}
		listener.Close()
		os.Exit(0)
	}()

	log.Printf("Process is running. Use Ctrl+C to quit.")
	for {

	}
}

func subscribeToNetwork(filename string, port string) error {
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644) //O_APPEND is atomic <33
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(port + "\n")
	return err
}

func unsubscribeFromNetwork(filename string, port string) error {
	file, err := lockedfile.Edit(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var newLines []string
	for scanner.Scan() {
		if scanner.Text() == port {
			continue
		}
		newLines = append(newLines, scanner.Text())
	}

	err = file.Truncate(0)
	if err != nil {
		return err
	}

	_, err = file.Seek(0, 0)
	if err != nil {
		return err
	}

	for _, line := range newLines {
		_, err = file.WriteString(line + "\n")
		if err != nil {
			return err
		}
	}
	return nil
}
