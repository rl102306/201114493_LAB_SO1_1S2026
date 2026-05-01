package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "go-grpc-client/proto"
)

type WarReport struct {
	Country         string `json:"country"`
	WarplanesInAir  int32  `json:"warplanes_in_air"`
	WarshipsInWater int32  `json:"warships_in_water"`
	Timestamp       string `json:"timestamp"`
}

var grpcClient pb.WarReportServiceClient

func initGRPCClient() {
	serverAddr := os.Getenv("GRPC_SERVER_ADDR")
	if serverAddr == "" {
		serverAddr = "go-grpc-server-svc:50051"
	}

	conn, err := grpc.NewClient(
		serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("Failed to connect to gRPC server at %s: %v", serverAddr, err)
	}

	grpcClient = pb.NewWarReportServiceClient(conn)
	log.Printf("gRPC client initialized, target: %s", serverAddr)
}

func countryToEnum(country string) pb.Countries {
	switch country {
	case "USA":
		return pb.Countries_usa
	case "RUS":
		return pb.Countries_rus
	case "CHN":
		return pb.Countries_chn
	case "ESP":
		return pb.Countries_esp
	case "GTM":
		return pb.Countries_gtm
	default:
		return pb.Countries_countries_unknown
	}
}

func handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var report WarReport
	if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
		http.Error(w, fmt.Sprintf("Bad request: %v", err), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := grpcClient.SendReport(ctx, &pb.WarReportRequest{
		Country:         countryToEnum(report.Country),
		WarplanesInAir:  report.WarplanesInAir,
		WarshipsInWater: report.WarshipsInWater,
		Timestamp:       report.Timestamp,
	})
	if err != nil {
		log.Printf("gRPC SendReport error: %v", err)
		http.Error(w, "gRPC call failed", http.StatusInternalServerError)
		return
	}

	log.Printf("Report sent: country=%s warplanes=%d warships=%d status=%s",
		report.Country, report.WarplanesInAir, report.WarshipsInWater, resp.Status)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": resp.Status})
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}

func main() {
	initGRPCClient()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/report", handleReport)
	mux.HandleFunc("/health", healthHandler)

	log.Printf("go-grpc-client listening on :%s", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), mux); err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}
}
