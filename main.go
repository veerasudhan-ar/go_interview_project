package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

type Trait struct {
	Value string `json:"value"`
	Type  string `json:"type"`
}

// EventData represents the structure of your JSON data
type EventData struct {
	Ev    string           `json:"ev"`
	Et    string           `json:"et"`
	ID    string           `json:"id"`
	UID   string           `json:"uid"`
	MID   string           `json:"mid"`
	T     string           `json:"t"`
	P     string           `json:"p"`
	L     string           `json:"l"`
	Sc    string           `json:"sc"`
	Attr  map[string]Trait `json:"-"` // Ignore this field during (un)marshalling
	UAttr map[string]Trait `json:"-"` // Ignore this field during (un)marshalling
}

// WebhookData represents the structure of your JSON data
type WebhookData struct {
	Ev    string           `json:"event"`
	Et    string           `json:"event_type"`
	ID    string           `json:"app_id"`
	UID   string           `json:"user_id"`
	MID   string           `json:"message_id"`
	T     string           `json:"page_title"`
	P     string           `json:"page_url"`
	L     string           `json:"browser_language"`
	Sc    string           `json:"screen_size"`
	Attr  map[string]Trait `json:"attributes"` // Ignore this field during (un)marshalling
	UAttr map[string]Trait `json:"traits"`     // Ignore this field during (un)marshalling
}

// UnmarshalJSON is a custom unmarshaller for EventData
func (e *EventData) UnmarshalJSON(data []byte) error {
	// Use a map to dynamically capture all keys
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	// Manually assign known fields
	e.Ev = m["ev"].(string)
	e.Et = m["et"].(string)
	e.ID = m["id"].(string)
	e.UID = m["uid"].(string)
	e.MID = m["mid"].(string)
	e.T = m["t"].(string)
	e.P = m["p"].(string)
	e.L = m["l"].(string)
	e.Sc = m["sc"].(string)

	// Initialize the attribute maps
	e.Attr = make(map[string]Trait)
	e.UAttr = make(map[string]Trait)

	// Loop through the map to capture dynamic attributes
	for key, value := range m {
		// check slice bounds to avoid panic
		if len(key) < 4 {
			continue
		}

		switch {
		case key[:4] == "atrk":
			index := key[4:]
			traitKey := value.(string)
			e.Attr[traitKey] = Trait{
				Value: m["atrv"+index].(string),
				Type:  m["atrt"+index].(string),
			}
		case key[:5] == "uatrk":
			index := key[5:]
			traitKey := value.(string)
			e.UAttr[traitKey] = Trait{
				Value: m["uatrv"+index].(string),
				Type:  m["uatrt"+index].(string),
			}
		}
	}

	return nil
}

func main() {
	// create a waitgroup to wait for the goroutines to finish
	wg := sync.WaitGroup{}

	workerChannel := make(chan interface{})
	app := fiber.New()

	// POST /receive-json
	app.Post("/receive-json", func(c *fiber.Ctx) error {
		var jsonData interface{}

		if err := c.BodyParser(&jsonData); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Cannot parse JSON",
			})
		}

		// pass jsonData to a channel
		workerChannel <- jsonData

		return c.SendStatus(fiber.StatusOK)
	})

	// worker goroutine
	wg.Add(1)
	go func() {

		for data := range workerChannel {
			var eventData EventData

			// Unmarshal the JSON into the struct
			jsonBytes, _ := json.Marshal(data)
			json.Unmarshal(jsonBytes, &eventData)
			fmt.Printf("Processed Data: %+v\n", eventData)
			// change it to WebhookData
			webhookData := WebhookData{
				Ev:    eventData.Ev,
				Et:    eventData.Et,
				ID:    eventData.ID,
				UID:   eventData.UID,
				MID:   eventData.MID,
				T:     eventData.T,
				P:     eventData.P,
				L:     eventData.L,
				Sc:    eventData.Sc,
				Attr:  eventData.Attr,
				UAttr: eventData.UAttr,
			}

			// Marshal the struct back into JSON
			jsonBytes, _ = json.Marshal(webhookData)

			// send it to https://webhook.site/
			req, err := http.NewRequest("POST", "https://webhook.site/b8b1ee79-d00a-4248-808e-92ebc4226148", bytes.NewBuffer(jsonBytes))
			if err != nil {
				log.Fatal("Error reading request. ", err)
			}

			req.Header.Set("Content-Type", "application/json")

			client := &http.Client{Timeout: time.Second * 10}

			resp, err := client.Do(req)
			if err != nil {
				log.Fatal("Error reading response. ", err)
			}
			defer resp.Body.Close()

			wg.Done()
		}
	}()

	// Start the server on a specific port
	app.Listen(":3000")

	// wait for the goroutines to finish
	wg.Wait()
}
