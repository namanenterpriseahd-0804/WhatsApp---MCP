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

	targetJID := client.Store.ID.ToNonAD()

	// 1. SEND IMAGE
	fmt.Println("Sending Image...")
	imgData, err := os.ReadFile("C:\\Users\\Naman\\Downloads\\Gemini_Generated_Image_xarxtlxarxtlxarx.png")
	if err != nil {
		fmt.Println("Failed to read image:", err)
	} else {
		respImg, err := client.Upload(context.Background(), imgData, whatsmeow.MediaImage)
		if err != nil {
			fmt.Println("Failed to upload image:", err)
		} else {
			imgMsg := &waE2E.ImageMessage{
				URL:           proto.String(respImg.URL),
				DirectPath:    proto.String(respImg.DirectPath),
				MediaKey:      respImg.MediaKey,
				Mimetype:      proto.String("image/png"),
				FileEncSHA256: respImg.FileEncSHA256,
				FileSHA256:    respImg.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(imgData))),
			}
			msg := &waE2E.Message{ImageMessage: imgMsg}
			
			resp, err := client.SendMessage(context.Background(), targetJID, msg)
			if err != nil {
				fmt.Println("Error sending image:", err)
			} else {
				fmt.Println("Image sent! ID:", resp.ID)
			}
		}
	}

	// 2. SEND VOICE MESSAGE
	fmt.Println("Sending Voice Message...")
	audioData, err := os.ReadFile("output.ogg")
	if err != nil {
		fmt.Println("Failed to read audio:", err)
	} else {
		respAud, err := client.Upload(context.Background(), audioData, whatsmeow.MediaAudio)
		if err != nil {
			fmt.Println("Failed to upload audio:", err)
		} else {
			audMsg := &waE2E.AudioMessage{
				URL:           proto.String(respAud.URL),
				DirectPath:    proto.String(respAud.DirectPath),
				MediaKey:      respAud.MediaKey,
				Mimetype:      proto.String("audio/ogg; codecs=opus"),
				FileEncSHA256: respAud.FileEncSHA256,
				FileSHA256:    respAud.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(audioData))),
				PTT:           proto.Bool(true),
			}
			msg2 := &waE2E.Message{AudioMessage: audMsg}
			resp2, err := client.SendMessage(context.Background(), targetJID, msg2)
			if err != nil {
				fmt.Println("Error sending voice:", err)
			} else {
				fmt.Println("Voice message sent! ID:", resp2.ID)
			}
		}
	}

	// 3. SEND TEXT MESSAGE
	fmt.Println("Sending Text Message...")
	msg3 := &waE2E.Message{
		Conversation: proto.String("This is an Google Antigravity (Test Message)"),
	}
	resp3, err := client.SendMessage(context.Background(), targetJID, msg3)
	if err != nil {
		fmt.Println("Error sending text:", err)
	} else {
		fmt.Println("Text message sent! ID:", resp3.ID)
	}
}
