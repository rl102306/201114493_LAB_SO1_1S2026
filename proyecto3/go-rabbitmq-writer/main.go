// Deployment 3 — RabbitMQ Writer
// Recibe WarMessage JSON via HTTP POST /publish desde el gRPC Server
// (Deployment 2) y publica el mensaje en RabbitMQ.
//
// CONCURRENCIA: el canal AMQP no es thread-safe. Se protege con sync.Mutex
// para que múltiples goroutines (una por request HTTP) no colisionen al
// publicar simultáneamente.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type WarMessage struct {
	Country         string `json:"country"`
	WarplanesInAir  int32  `json:"warplanes_in_air"`
	WarshipsInWater int32  `json:"warships_in_water"`
	Timestamp       string `json:"timestamp"`
}

// broker agrupa la conexión AMQP y su mutex de protección.
type broker struct {
	mu   sync.Mutex
	conn *amqp.Connection
	ch   *amqp.Channel
	url  string
	queue string
}

func newBroker(url, queue string) (*broker, error) {
	b := &broker{url: url, queue: queue}
	if err := b.connect(); err != nil {
		return nil, err
	}
	return b, nil
}

func (b *broker) connect() error {
	var conn *amqp.Connection
	var err error
	for attempt := 1; attempt <= 15; attempt++ {
		conn, err = amqp.Dial(b.url)
		if err == nil {
			break
		}
		log.Printf("RabbitMQ intento %d/15: %v", attempt, err)
		time.Sleep(5 * time.Second)
	}
	if err != nil {
		return fmt.Errorf("conexión agotada: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("canal: %w", err)
	}
	if _, err = ch.QueueDeclare(b.queue, true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return fmt.Errorf("declarar cola: %w", err)
	}

	b.conn = conn
	b.ch = ch
	log.Printf("Conectado a RabbitMQ, cola=%s", b.queue)
	return nil
}

// reconnect se llama bajo el mutex cuando el canal detecta un error.
func (b *broker) reconnect() {
	log.Println("Reconectando a RabbitMQ...")
	if b.ch != nil {
		b.ch.Close()
	}
	if b.conn != nil {
		b.conn.Close()
	}
	for {
		if err := b.connect(); err != nil {
			log.Printf("Reconexión fallida: %v — reintentando en 5s", err)
			time.Sleep(5 * time.Second)
			continue
		}
		log.Println("Reconexión exitosa")
		return
	}
}

// publish publica un mensaje de forma thread-safe.
func (b *broker) publish(body []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	err := b.ch.Publish(
		"",      // exchange default
		b.queue, // routing key
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
		},
	)
	if err != nil {
		// Canal roto — reconectar bajo el mismo mutex
		b.reconnect()
		// Reintentar una vez tras reconexión
		err = b.ch.Publish("", b.queue, false, false,
			amqp.Publishing{
				ContentType:  "application/json",
				Body:         body,
				DeliveryMode: amqp.Persistent,
			},
		)
	}
	return err
}

// ---- HTTP handlers ----

var rb *broker

func publishHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "método no permitido", http.StatusMethodNotAllowed)
		return
	}

	var msg WarMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	body, err := json.Marshal(msg)
	if err != nil {
		http.Error(w, "marshal error", http.StatusInternalServerError)
		return
	}

	if err := rb.publish(body); err != nil {
		log.Printf("Error publicando: %v", err)
		http.Error(w, "publish failed", http.StatusServiceUnavailable)
		return
	}

	log.Printf("Publicado: country=%s warplanes=%d warships=%d",
		msg.Country, msg.WarplanesInAir, msg.WarshipsInWater)
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}

func main() {
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		rabbitURL = "amqp://guest:guest@rabbitmq-svc:5672/"
	}
	queueName := os.Getenv("RABBITMQ_QUEUE")
	if queueName == "" {
		queueName = "war_reports"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	var err error
	rb, err = newBroker(rabbitURL, queueName)
	if err != nil {
		log.Fatalf("Setup RabbitMQ: %v", err)
	}
	defer rb.conn.Close()
	defer rb.ch.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/publish", publishHandler)
	mux.HandleFunc("/health", healthHandler)

	log.Printf("go-rabbitmq-writer (Deployment 3) escuchando en :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("HTTP server: %v", err)
	}
}
