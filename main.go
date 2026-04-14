package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	_ "github.com/mattn/go-sqlite3" // Requires CGO
	"github.com/mdp/qrterminal/v3"
	"github.com/skip2/go-qrcode"
)

func main() {
	// Initialize SQLite store with CGO
	container, err := sqlstore.New(context.Background(), "sqlite3", "file:store.db?_foreign_keys=on", nil)
	if err != nil {
		panic(err)
	}
	
	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		panic(err)
	}

	client := whatsmeow.NewClient(deviceStore, nil)
	
	if client.Store.ID == nil {
		// New login
		qrChan, _ := client.GetQRChannel(context.Background())
		err = client.Connect()
		if err != nil {
			panic(err)
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				fmt.Println("Scan this QR Code:")
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
				qrcode.WriteFile(evt.Code, qrcode.Medium, 256, "qr.png")
				fmt.Println("QR code image saved to qr.png")
			} else {
				fmt.Println("Login event:", evt.Event)
			}
		}
	} else {
		// Already logged in
		err = client.Connect()
		if err != nil {
			panic(err)
		}
		fmt.Println("Connected to WhatsApp!")
	}

	// Listen to Ctrl+C (you can also do something else that prevents the program from exiting)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	client.Disconnect()
}
