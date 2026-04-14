package main

import (
	"context"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

func main() {
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
		fmt.Println("No login session found")
		return
	}

	err = client.Connect()
	if err != nil {
		panic(err)
	}
	defer client.Disconnect()

	fmt.Println("Connected. Waiting for session sync...")
	time.Sleep(3 * time.Second)
	fmt.Println("Sending message to self...")

	// Send message to own number (Note: ToNonAD returns the standard JID structure)
	targetJID := client.Store.ID.ToNonAD()

	msg := &waE2E.Message{
		Conversation: proto.String("This is an Google Antigravity (Test Message)"),
	}

	resp, err := client.SendMessage(context.Background(), targetJID, msg)
	if err != nil {
		fmt.Println("Error sending message:", err)
	} else {
		fmt.Println("Message sent! ID:", resp.ID)
	}
}
