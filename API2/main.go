package main

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

const (
	CARNET = "201114493"
	VM     = "VM1"
	API    = "API2"
)

var (
	API1_HEALTH = "http://localhost:8081/health"
	API3_HEALTH = "http://192.168.122.29:8083/health"
)

func main() {
	app := fiber.New()
	app.Use(logger.New())

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":    "UP",
			"message":   API + " is Ready",
			"timestamp": time.Now().Format(time.RFC3339),
			"VM":        VM,
			"carnet":    CARNET,
		})
	})

	app.Get("/api2/"+CARNET+"/call-api1", func(c *fiber.Ctx) error {
		ok := checkHealth(API1_HEALTH)
		if ok {
			return c.JSON(fiber.Map{"apiname": "API1", "message": "The API1 located on the VM1 is working", "connection": true, "carnet": CARNET})
		}
		return c.JSON(fiber.Map{"apiname": "API1", "message": "ERROR: The API1 located on the VM1 is not working", "connection": false, "carnet": CARNET})
	})

	app.Get("/api2/"+CARNET+"/call-api3", func(c *fiber.Ctx) error {
		ok := checkHealth(API3_HEALTH)
		if ok {
			return c.JSON(fiber.Map{"apiname": "API3", "message": "The API3 located on the VM2 is working", "connection": true, "carnet": CARNET})
		}
		return c.JSON(fiber.Map{"apiname": "API3", "message": "ERROR: The API3 located on the VM2 is not working", "connection": false, "carnet": CARNET})
	})

	app.Listen(":8082")
}

func checkHealth(url string) bool {
	client := fiber.Get(url)
	client.Timeout(3 * time.Second)
	statusCode, _, errs := client.Bytes()
	if errs != nil {
		return false
	}
	return statusCode == 200
}

