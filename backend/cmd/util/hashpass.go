package main

import (
	"fmt"
	"log"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run hashpass.go <password>")
	}

	password := os.Args[1]
	
	// SOTA: Cost of 12 is the current standard balance of security vs performance
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	fmt.Printf("Password: %s\n", password)
	fmt.Printf("Bcrypt Hash: %s\n", string(hash))
}