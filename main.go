package main

import (
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	log "github.com/sirupsen/logrus"
	"os"
	"strconv"
	"time"
)

// Global LogManager so handlers can use it
var (
	lm                *LogManager
	saveToFileEnabled bool
	dataDir           string
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
	// Read env-based options
	saveToFileEnabled = getEnvBool("SAVE_TO_FILE", true)
	dataDir = os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	// Init Loki client + LogManager
	lokiClient := NewLokiClient()
	lm = NewLogManager(lokiClient)
	lm.LoadTemplates()
	defer lm.CloseLogManager()

	log.WithFields(log.Fields{
		"loki_enabled":  lokiClient.Enabled,
		"loki_push_url": lokiClient.PushURL,
		"loki_job":      lokiClient.Job,
		"save_to_file":  saveToFileEnabled,
		"data_dir":      dataDir,
	}).Info("Initialized action event logger")

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

	// Save to flat file (local audit trail), *if enabled*
	if saveToFileEnabled {
		if err := saveToFile(event); err != nil {
			log.WithError(err).Error("failed to save action event to file")

			// Also send error to Loki (if configured)
			if lm != nil {
				fields := buildLokiFieldsFromEvent(&event)
				fields["error"] = err.Error()
				l := lm.BuildLog(
					"PHONE_ACTION",
					"Failed to save action event for customer %s (%s)",
					log.ErrorLevel,
					fields,
					event.CustomerID,
					event.EventType,
				)
				lm.SendLog(l)
			}

			return c.Status(500).SendString("Error saving event")
		}
	} else {
		log.WithFields(log.Fields{
			"customer_id": event.CustomerID,
			"event_type":  event.EventType,
		}).Debug("SAVE_TO_FILE is disabled; event not written to disk")
	}

	// Send successful event to Loki
	if lm != nil {
		fields := buildLokiFieldsFromEvent(&event)
		l := lm.BuildLog(
			"PHONE_ACTION",
			"Action event (%s) recorded for customer %s",
			log.InfoLevel,
			fields,
			event.EventType,
			event.CustomerID,
		)
		lm.SendLog(l)
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
	// Ensure the data directory exists
	if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
		return err
	}

	filename := fmt.Sprintf("%s/%s_events.json", dataDir, event.CustomerID)
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		if cerr := file.Close(); cerr != nil {
			log.Error(cerr)
		}
	}(file)

	encoder := json.NewEncoder(file)
	return encoder.Encode(event)
}

// buildLokiFieldsFromEvent flattens ActionEvent into a Loki-friendly fields map.
func buildLokiFieldsFromEvent(event *ActionEvent) map[string]interface{} {
	fields := map[string]interface{}{
		"timestamp":      event.Timestamp.Format(time.RFC3339Nano),
		"customer_id":    event.CustomerID,
		"event_type":     event.EventType,
		"mac":            event.MAC,
		"ip":             event.IP,
		"model":          event.Model,
		"firmware":       event.Firmware,
		"active_url":     event.ActiveURL,
		"active_user":    event.ActiveUser,
		"active_host":    event.ActiveHost,
		"local":          event.Local,
		"remote":         event.Remote,
		"display_local":  event.DisplayLocal,
		"display_remote": event.DisplayRemote,
		"call_id":        event.CallID,
		"caller_id":      event.CallerID,
		"called_number":  event.CalledNumber,
	}

	for k, v := range event.AdditionalInfo {
		fields["extra_"+k] = v
	}

	return fields
}

// getEnvBool reads a bool env var with a default.
func getEnvBool(name string, def bool) bool {
	v, ok := os.LookupEnv(name)
	if !ok || v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
