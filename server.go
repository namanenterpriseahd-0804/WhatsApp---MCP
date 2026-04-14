package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/rs/cors"
	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
	"github.com/skip2/go-qrcode"
)

var WAClient *whatsmeow.Client
var CurrentQRBase64 string

type SendPayload struct {
	JID     string `json:"jid"`
	Message string `json:"message"`
}

func initWhatsApp() {
	container, err := sqlstore.New(context.Background(), "sqlite3", "file:store.db?_foreign_keys=on", nil)
	if err != nil {
		fmt.Println("DB Init Error:", err)
		return
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		fmt.Println("Device Store Error:", err)
		return
	}

	WAClient = whatsmeow.NewClient(deviceStore, nil)
	if WAClient.Store.ID == nil {
		qrChan, _ := WAClient.GetQRChannel(context.Background())
		go func() {
			err = WAClient.Connect()
			if err != nil {
				fmt.Println("Connect Error:", err)
				return
			}
			for evt := range qrChan {
				if evt.Event == "code" {
					png, err := qrcode.Encode(evt.Code, qrcode.Medium, 256)
					if err == nil {
						CurrentQRBase64 = base64.StdEncoding.EncodeToString(png)
						fmt.Println("New QR code generated.")
					}
				} else if evt.Event == "success" {
					CurrentQRBase64 = ""
					fmt.Println("Login successful!")
				}
			}
		}()
	} else {
		err = WAClient.Connect()
		if err != nil {
			fmt.Println("Connect Error:", err)
			return
		}
		fmt.Println("Connected to WhatsApp!")
	}
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	if WAClient == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "error", "message": "Client not initialized"})
		return
	}
	
	status := "Connected"
	if WAClient.Store.ID == nil {
		status = "Awaiting Login"
	}
	
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": status,
		"jid":    WAClient.Store.ID,
	})
}

func qrHandler(w http.ResponseWriter, r *http.Request) {
	if WAClient.Store.ID != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Already logged in"})
		return
	}
	if CurrentQRBase64 == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "QR not ready"})
		return
	}
	
	json.NewEncoder(w).Encode(map[string]interface{}{
		"qr": "data:image/png;base64," + CurrentQRBase64,
	})
}

func groupsHandler(w http.ResponseWriter, r *http.Request) {
	if !WAClient.IsConnected() {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Not connected"})
		return
	}

	groups, err := WAClient.GetJoinedGroups(context.Background())
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}

	var formattedGroups []interface{}
	for _, group := range groups {
		formattedGroups = append(formattedGroups, map[string]interface{}{
			"jid":          group.JID.String(),
			"name":         group.Name,
			"participants": len(group.Participants),
		})
	}
	json.NewEncoder(w).Encode(formattedGroups)
}

func sendHandler(w http.ResponseWriter, r *http.Request) {
	var payload SendPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Invalid payload"})
		return
	}

	var targetJID types.JID
	if strings.Contains(payload.JID, "@g.us") {
		parts := strings.Split(payload.JID, "@")
		targetJID = types.NewJID(parts[0], parts[1])
	} else if strings.Contains(payload.JID, "@s.whatsapp.net") {
		parts := strings.Split(payload.JID, "@")
		targetJID = types.NewJID(parts[0], parts[1])
	} else {
		// Default format mapped if no standard appendix provided
		targetJID = types.NewJID(payload.JID, "s.whatsapp.net")
	}

	msg := &waE2E.Message{
		Conversation: proto.String(payload.Message),
	}

	resp, err := WAClient.SendMessage(context.Background(), targetJID, msg)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
	} else {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "id": resp.ID})
	}
}

func main() {
	initWhatsApp()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", statusHandler)
	mux.HandleFunc("/api/qr", qrHandler)
	mux.HandleFunc("/api/groups", groupsHandler)
	mux.HandleFunc("/api/send", sendHandler)

	handler := cors.Default().Handler(mux)

	fmt.Println("Go Server listening on http://localhost:8080")
	http.ListenAndServe(":8080", handler)
}
