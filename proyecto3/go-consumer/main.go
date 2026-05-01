package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
)

type WarMessage struct {
	Country         string `json:"country"`
	WarplanesInAir  int32  `json:"warplanes_in_air"`
	WarshipsInWater int32  `json:"warships_in_water"`
	Timestamp       string `json:"timestamp"`
}

var (
	rdb *redis.Client
	ctx = context.Background()
)

func connectValkey(addr string) *redis.Client {
	for {
		client := redis.NewClient(&redis.Options{Addr: addr})
		if err := client.Ping(ctx).Err(); err != nil {
			log.Printf("Valkey not ready (%v), retrying in 5s...", err)
			client.Close()
			time.Sleep(5 * time.Second)
			continue
		}
		log.Printf("Connected to Valkey at %s", addr)
		return client
	}
}

func connectRabbitMQ(url, queue string) (*amqp.Connection, *amqp.Channel, <-chan amqp.Delivery, error) {
	var conn *amqp.Connection
	var err error
	for attempt := 1; attempt <= 15; attempt++ {
		conn, err = amqp.Dial(url)
		if err == nil {
			break
		}
		log.Printf("RabbitMQ attempt %d/15: %v", attempt, err)
		time.Sleep(5 * time.Second)
	}
	if err != nil {
		return nil, nil, nil, fmt.Errorf("RabbitMQ exhausted: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, nil, nil, err
	}
	if err := ch.Qos(10, 0, false); err != nil {
		ch.Close()
		conn.Close()
		return nil, nil, nil, err
	}
	if _, err = ch.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, nil, nil, err
	}
	msgs, err := ch.Consume(queue, "go-consumer", false, false, false, false, nil)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, nil, nil, err
	}
	log.Printf("Consuming queue=%s", queue)
	return conn, ch, msgs, nil
}

// updateMaxMin updates max and min keys for a given prefix.
// Not atomic but acceptable for analytics workloads.
func updateMaxMin(prefix string, value int64) {
	maxKey := prefix + ":max"
	minKey := prefix + ":min"

	cur, err := rdb.Get(ctx, maxKey).Int64()
	if err == redis.Nil || value > cur {
		rdb.Set(ctx, maxKey, value, 0)
	}
	cur, err = rdb.Get(ctx, minKey).Int64()
	if err == redis.Nil || value < cur {
		rdb.Set(ctx, minKey, value, 0)
	}
}

func processMessage(msg WarMessage) error {
	c := msg.Country
	if c == "" {
		return fmt.Errorf("empty country")
	}

	planes := int64(msg.WarplanesInAir)
	ships := int64(msg.WarshipsInWater)
	unixMs := time.Now().UnixMilli()

	// --- Unique sequence numbers for time-series members ---
	// We need unique members in the sorted set so duplicate values at different
	// timestamps are not overwritten. We use a per-key auto-increment counter.
	planesSeq, err := rdb.Incr(ctx, fmt.Sprintf("%s:warplanes:ts:seq", c)).Result()
	if err != nil {
		return fmt.Errorf("incr planes seq: %w", err)
	}
	shipsSeq, err := rdb.Incr(ctx, fmt.Sprintf("%s:warships:ts:seq", c)).Result()
	if err != nil {
		return fmt.Errorf("incr ships seq: %w", err)
	}

	// Member format: "{value}:{seq}" — score = Unix ms timestamp
	planesMember := fmt.Sprintf("%d:%d", planes, planesSeq)
	shipsMember := fmt.Sprintf("%d:%d", ships, shipsSeq)

	pipe := rdb.Pipeline()

	// --- Per-country ---
	pipe.Incr(ctx, fmt.Sprintf("%s:count", c))
	pipe.IncrBy(ctx, fmt.Sprintf("%s:warplanes:total", c), planes)
	pipe.IncrBy(ctx, fmt.Sprintf("%s:warships:total", c), ships)
	// Frequency hashes (for per-country mode)
	pipe.HIncrBy(ctx, fmt.Sprintf("%s:warplanes:freq", c), strconv.FormatInt(planes, 10), 1)
	pipe.HIncrBy(ctx, fmt.Sprintf("%s:warships:freq", c), strconv.FormatInt(ships, 10), 1)
	// Time series — score=unixMs, member="{value}:{seq}"
	pipe.ZAdd(ctx, fmt.Sprintf("%s:warplanes:timeseries", c), redis.Z{Score: float64(unixMs), Member: planesMember})
	pipe.ZAdd(ctx, fmt.Sprintf("%s:warships:timeseries", c), redis.Z{Score: float64(unixMs), Member: shipsMember})
	// Keep only the most recent 1000 data points per series
	pipe.ZRemRangeByRank(ctx, fmt.Sprintf("%s:warplanes:timeseries", c), 0, -1001)
	pipe.ZRemRangeByRank(ctx, fmt.Sprintf("%s:warships:timeseries", c), 0, -1001)

	// --- Global ---
	pipe.Incr(ctx, "global:count")
	pipe.IncrBy(ctx, "global:warplanes:total", planes)
	pipe.IncrBy(ctx, "global:warships:total", ships)
	// Global frequency hashes (for global mode)
	pipe.HIncrBy(ctx, "global:warplanes:freq", strconv.FormatInt(planes, 10), 1)
	pipe.HIncrBy(ctx, "global:warships:freq", strconv.FormatInt(ships, 10), 1)
	// Country ranking sorted sets — score accumulates total units reported per country
	pipe.ZIncrBy(ctx, "global:warplanes:ranking", float64(planes), c)
	pipe.ZIncrBy(ctx, "global:warships:ranking", float64(ships), c)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("pipeline: %w", err)
	}

	// --- Max / Min (separate calls) ---
	updateMaxMin(fmt.Sprintf("%s:warplanes", c), planes)
	updateMaxMin(fmt.Sprintf("%s:warships", c), ships)
	updateMaxMin("global:warplanes", planes)
	updateMaxMin("global:warships", ships)

	return nil
}

func main() {
	valkeyAddr := os.Getenv("VALKEY_ADDR")
	if valkeyAddr == "" {
		valkeyAddr = "valkey-vm-svc:6379"
	}
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		rabbitURL = "amqp://guest:guest@rabbitmq-svc:5672/"
	}
	queueName := os.Getenv("RABBITMQ_QUEUE")
	if queueName == "" {
		queueName = "war_reports"
	}

	rdb = connectValkey(valkeyAddr)
	defer rdb.Close()

	for {
		conn, ch, msgs, err := connectRabbitMQ(rabbitURL, queueName)
		if err != nil {
			log.Printf("Connect failed: %v — retrying in 10s", err)
			time.Sleep(10 * time.Second)
			continue
		}

		connClose := conn.NotifyClose(make(chan *amqp.Error, 1))
		log.Println("go-consumer ready")

	loop:
		for {
			select {
			case d, ok := <-msgs:
				if !ok {
					log.Println("Channel closed — reconnecting")
					break loop
				}
				var msg WarMessage
				if err := json.Unmarshal(d.Body, &msg); err != nil {
					log.Printf("Unmarshal error: %v — discarding", err)
					d.Nack(false, false)
					continue
				}
				if err := processMessage(msg); err != nil {
					log.Printf("Process error: %v — requeuing", err)
					d.Nack(false, true)
					continue
				}
				log.Printf("OK: %s planes=%d ships=%d", msg.Country, msg.WarplanesInAir, msg.WarshipsInWater)
				d.Ack(false)

			case e := <-connClose:
				log.Printf("Connection lost: %v — reconnecting", e)
				break loop
			}
		}
		ch.Close()
		conn.Close()
		time.Sleep(5 * time.Second)
	}
}
