package main

import (
	"context"
	"fmt"
	"os"
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
	fmt.Println("Sending voice message to self...")

	targetJID := client.Store.ID.ToNonAD()

	audioData, err := os.ReadFile("output.ogg")
	if err != nil {
		panic(err)
	}
    
    // Upload the audio file to WhatsApp servers
	resp, err := client.Upload(context.Background(), audioData, whatsmeow.MediaAudio)
	if err != nil {
		panic(err)
	}

	audMsg := &waE2E.AudioMessage{
		URL:           proto.String(resp.URL),
		DirectPath:    proto.String(resp.DirectPath),
		MediaKey:      resp.MediaKey,
		Mimetype:      proto.String("audio/ogg; codecs=opus"),
		FileEncSHA256: resp.FileEncSHA256,
		FileSHA256:    resp.FileSHA256,
		FileLength:    proto.Uint64(uint64(len(audioData))),
		PTT:           proto.Bool(true), // PTT means push-to-talk (Voice Message)
	}

	msg := &waE2E.Message{
		AudioMessage: audMsg,
	}

	sendResp, err := client.SendMessage(context.Background(), targetJID, msg)
	if err != nil {
		fmt.Println("Error sending message:", err)
	} else {
		fmt.Println("Voice message sent! ID:", sendResp.ID)
	}
}
