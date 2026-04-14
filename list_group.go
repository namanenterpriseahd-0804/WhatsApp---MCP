package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
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

	// Give it a tiny bit of time to sync node states if needed just in case
	time.Sleep(2 * time.Second)

	fmt.Println("Fetching joined groups...")
	groups, err := client.GetJoinedGroups(context.Background())
	if err != nil {
		fmt.Println("Error fetching groups:", err)
		return
	}

	found := false
	for _, group := range groups {
		if strings.TrimSpace(group.Name) == "SIHL HO Staff" {
			found = true
			fmt.Printf("Found Group: '%s' (JID: %s)\n", group.Name, group.JID)
			fmt.Printf("Total Participants: %d\n", len(group.Participants))
			fmt.Println("--------------------------------------------------")
			
			for i, participant := range group.Participants {
				// participant.JID is the user's handle. .User is their phone number.
				fmt.Printf("%d. %s\n", i+1, participant.JID.User)
			}
			fmt.Println("--------------------------------------------------")
		}
	}

	if !found {
		fmt.Println("Could not find a group named 'SIHL HO Staff'.")
		fmt.Println("Available groups:")
		for _, group := range groups {
			fmt.Printf("- '%s'\n", group.Name)
		}
	}
}
