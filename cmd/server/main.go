package main

import (
	"fmt"
	"log"
	"net/http"

	"order-matching-engine/internal/api"
	"order-matching-engine/internal/engine"
)

func main() {
	fmt.Println("order-matching-engine starting...")

	eng := engine.NewMatchingEngine()
	apiLayer := api.NewAPI(eng)

	router := apiLayer.Router()

	fmt.Println("Server listening on :8080")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatal(err)
	}
}
