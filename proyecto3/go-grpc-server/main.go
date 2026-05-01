// Deployment 2 — gRPC Server
// Receives WarReportRequest via gRPC, forwards the payload to
// go-rabbitmq-writer (Deployment 3) via HTTP POST /publish.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"google.golang.org/grpc"

	pb "go-grpc-server/proto"
)

type WarMessage struct {
	Country         string `json:"country"`
	WarplanesInAir  int32  `json:"warplanes_in_air"`
	WarshipsInWater int32  `json:"warships_in_water"`
	Timestamp       string `json:"timestamp"`
}

type server struct {
	pb.UnimplementedWarReportServiceServer
	httpClient    *http.Client
	writerURL     string
}

func (s *server) SendReport(ctx context.Context, req *pb.WarReportRequest) (*pb.WarReportResponse, error) {
	msg := WarMessage{
		Country:         req.Country.String(),
		WarplanesInAir:  req.WarplanesInAir,
		WarshipsInWater: req.WarshipsInWater,
		Timestamp:       req.Timestamp,
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	url := s.writerURL + "/publish"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("forward to rabbitmq-writer: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rabbitmq-writer returned %d", resp.StatusCode)
	}

	log.Printf("Forwarded: country=%s warplanes=%d warships=%d",
		msg.Country, msg.WarplanesInAir, msg.WarshipsInWater)

	return &pb.WarReportResponse{Status: "ok"}, nil
}

func main() {
	writerURL := os.Getenv("RABBITMQ_WRITER_URL")
	if writerURL == "" {
		writerURL = "http://go-rabbitmq-writer-svc:8081"
	}

	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "50051"
	}

	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%s", grpcPort))
	if err != nil {
		log.Fatalf("Failed to listen on :%s — %v", grpcPort, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterWarReportServiceServer(grpcServer, &server{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		writerURL:  writerURL,
	})

	log.Printf("gRPC server (Deployment 2) listening on :%s, writer=%s", grpcPort, writerURL)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("gRPC serve: %v", err)
	}
}
