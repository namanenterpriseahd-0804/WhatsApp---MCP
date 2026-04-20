package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "store.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if len(os.Args) < 2 {
		log.Fatal("Please provide a search term")
	}
	searchTerm := "%" + os.Args[1] + "%"

	rows, err := db.Query("SELECT their_jid, full_name, push_name FROM whatsmeow_contacts WHERE full_name LIKE ? OR push_name LIKE ?", searchTerm, searchTerm)
	if err != nil {
		log.Fatal("Error querying contacts:", err)
	}
	defer rows.Close()

	found := false
	for rows.Next() {
		var jid, fullName, pushName sql.NullString
		err = rows.Scan(&jid, &fullName, &pushName)
		if err != nil {
			log.Fatal(err)
		}
		found = true
		fmt.Printf("JID: %s | Full Name: %s | Push Name: %s\n", jid.String, fullName.String, pushName.String)
	}

	if !found {
		fmt.Println("No contact found.")
	}
}
