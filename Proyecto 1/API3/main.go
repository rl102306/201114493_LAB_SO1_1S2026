package main

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

const (
	CARNET = "201114493" // Cambia con tu carnet
	VM     = "VM2"
	API    = "API3"
)

var (
	API1_HEALTH = "http://192.168.122.221:8081/health"
	API2_HEALTH = "http://192.168.122.221:8082/health"
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

	app.Get("/api3/"+CARNET+"/call-api1", func(c *fiber.Ctx) error {
		ok := checkHealth(API1_HEALTH)
		if ok {
			return c.JSON(fiber.Map{"apiname": "API1", "message": "The API1 located on the VM1 is working", "connection": true, "carnet": CARNET})
		}
		return c.JSON(fiber.Map{"apiname": "API1", "message": "ERROR: The API1 located on the VM1 is not working", "connection": false, "carnet": CARNET})
	})

	app.Get("/api3/"+CARNET+"/call-api2", func(c *fiber.Ctx) error {
		ok := checkHealth(API2_HEALTH)
		if ok {
			return c.JSON(fiber.Map{"apiname": "API2", "message": "The API2 located on the VM1 is working", "connection": true, "carnet": CARNET})
		}
		return c.JSON(fiber.Map{"apiname": "API2", "message": "ERROR: The API2 located on the VM1 is not working", "connection": false, "carnet": CARNET})
	})

	app.Listen(":8083")
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
