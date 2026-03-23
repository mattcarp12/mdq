package main

import (
	"log"
	"time"
)

func main() {
	log.Println("Worker starting up, connecting to Redis...")

	// TODO: Connect to Redis and Postgres

	for {
		// TODO: XREADGROUP from Redis Stream
		// TODO: If job received -> Update DB to RUNNING -> Execute -> Update DB to COMPLETED
		
		log.Println("Polling for jobs...")
		time.Sleep(5 * time.Second) // Simulated polling delay for now
	}
}