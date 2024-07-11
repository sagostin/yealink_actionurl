package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
)

type ActionEvent struct {
	Timestamp      time.Time
	MAC            string
	IP             string
	Model          string
	Firmware       string
	EventType      string
	CustomerID     string
	ActiveURL      string
	ActiveUser     string
	ActiveHost     string
	Local          string
	Remote         string
	DisplayLocal   string
	DisplayRemote  string
	CallID         string
	CallerID       string
	CalledNumber   string
	AdditionalInfo map[string]string
}

func main() {
	app := fiber.New()

	app.Get("/action/:customerID/:eventType", handleActionEvent)

	log.Fatal(app.Listen(":3000"))
}

func handleActionEvent(c *fiber.Ctx) error {
	customerID := c.Params("customerID")
	eventType := c.Params("eventType")

	event := ActionEvent{
		Timestamp:      time.Now(),
		MAC:            c.Query("mac"),
		IP:             c.Query("ip"),
		Model:          c.Query("model"),
		Firmware:       c.Query("firmware"),
		EventType:      eventType,
		CustomerID:     customerID,
		ActiveURL:      c.Query("active_url"),
		ActiveUser:     c.Query("active_user"),
		ActiveHost:     c.Query("active_host"),
		Local:          c.Query("local"),
		Remote:         c.Query("remote"),
		DisplayLocal:   c.Query("display_local"),
		DisplayRemote:  c.Query("display_remote"),
		CallID:         c.Query("call_id"),
		CallerID:       c.Query("callerID"),
		CalledNumber:   c.Query("calledNumber"),
		AdditionalInfo: make(map[string]string),
	}

	// Collect all other query parameters as additional info
	c.Context().QueryArgs().VisitAll(func(key, value []byte) {
		k := string(key)
		if !isStandardField(k) {
			event.AdditionalInfo[k] = string(value)
		}
	})

	// Save to flat file
	if err := saveToFile(event); err != nil {
		return c.Status(500).SendString("Error saving event")
	}

	return c.SendString("Event recorded successfully")
}

func isStandardField(field string) bool {
	standardFields := []string{
		"mac", "ip", "model", "firmware", "active_url", "active_user", "active_host",
		"local", "remote", "display_local", "display_remote", "call_id", "callerID", "calledNumber",
	}
	for _, f := range standardFields {
		if field == f {
			return true
		}
	}
	return false
}

func saveToFile(event ActionEvent) error {
	filename := fmt.Sprintf("%s_events.json", event.CustomerID)
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	return encoder.Encode(event)
}
