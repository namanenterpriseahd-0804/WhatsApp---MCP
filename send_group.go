package main

import (
	"context"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
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

	fmt.Println("Connected. Sending message to Bro Code...")

	// Extract the group identifier structure correctly
	// The full JID string we found was: 919898044899-1399830966@g.us
	parts := strings.Split("919898044899-1399830966@g.us", "@")
	targetJID := types.NewJID(parts[0], parts[1])

	msg := &waE2E.Message{
		Conversation: proto.String(" This is AI (Test message)"),
	}

	resp, err := client.SendMessage(context.Background(), targetJID, msg)
	if err != nil {
		fmt.Println("Error sending message:", err)
	} else {
		fmt.Println("Message sent to Bro Code! ID:", resp.ID)
	}
}
