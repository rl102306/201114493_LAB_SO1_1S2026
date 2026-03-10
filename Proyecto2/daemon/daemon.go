package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
)

type Memoria struct {
	TotalMB uint64 `json:"total_mb"`
	LibreMB uint64 `json:"libre_mb"`
	UsadaMB uint64 `json:"usada_mb"`
}

type Proceso struct {
	PID        int    `json:"pid"`
	Nombre     string `json:"nombre"`
	VSZKB      uint64 `json:"vsz_kb"`
	RSSKB      uint64 `json:"rss_kb"`
	MemPercent uint64 `json:"mem_percent"`
	CPUPercent uint64 `json:"cpu_percent"`
}

type InfoSistema struct {
	Memoria  Memoria   `json:"memoria"`
	Procesos []Proceso `json:"procesos"`
}

const (
	PROC_FILE = "/proc/continfo_pr2_so1_201114493"
	KERNEL_DIR = "/home/ronaldolara/201114493_LAB_SO1_1S2026/Proyecto2/kernel"
	BASH_DIR   = "/home/ronaldolara/201114493_LAB_SO1_1S2026/Proyecto2/bash"
	INTERVALO  = 20 * time.Second
	MAX_ALTO   = 2
	MAX_BAJO   = 3
)

var ctx = context.Background()

func conectarValkey() *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	_, err := client.Ping(ctx).Result()
	if err != nil {
		fmt.Printf("[ERROR] No se pudo conectar a Valkey: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("[OK] Conectado a Valkey correctamente")
	return client
}

func leerProc() (*InfoSistema, error) {
	data, err := os.ReadFile(PROC_FILE)
	if err != nil {
		return nil, fmt.Errorf("error leyendo %s: %v", PROC_FILE, err)
	}
	var info InfoSistema
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("error parseando JSON: %v", err)
	}
	return &info, nil
}

func ejecutarScript(ruta string) error {
	cmd := exec.Command("bash", ruta)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func cargarModulo() {
	fmt.Println("[INFO] Cargando módulo de kernel...")
	cmd := exec.Command("sudo", "insmod",
		KERNEL_DIR+"/continfo_pr2_so1_201114493.ko")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("[WARN] Módulo ya cargado o error: %v\n", err)
	} else {
		fmt.Println("[OK] Módulo de kernel cargado correctamente")
	}
}

func obtenerContenedores() []string {
	cmd := exec.Command("docker", "ps", "--format", "{{.Names}}")
	out, err := cmd.Output()
	if err != nil {
		return []string{}
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var contenedores []string
	for _, l := range lines {
		if l != "" && l != "grafana-sopes" && l != "valkey-sopes" {
			contenedores = append(contenedores, l)
		}
	}
	return contenedores
}

func eliminarContenedor(nombre string, rdb *redis.Client) {
	fmt.Printf("[INFO] Eliminando contenedor: %s\n", nombre)
	exec.Command("docker", "stop", nombre).Run()
	exec.Command("docker", "rm", nombre).Run()
	timestamp := time.Now().Unix()
	key := fmt.Sprintf("eliminado:%d:%s", timestamp, nombre)
	rdb.Set(ctx, key, timestamp, 24*time.Hour)
	rdb.Incr(ctx, "total_eliminados")
	fmt.Printf("[OK] Contenedor %s eliminado\n", nombre)
}

func gestionarContenedores(info *InfoSistema, rdb *redis.Client) {
	contenedores := obtenerContenedores()
	fmt.Printf("[INFO] Contenedores activos: %d\n", len(contenedores))

	var altoConsumo, bajoConsumo []string
	for _, c := range contenedores {
		if strings.Contains(c, "ram") || strings.Contains(c, "cpu") {
			altoConsumo = append(altoConsumo, c)
		} else {
			bajoConsumo = append(bajoConsumo, c)
		}
	}

	for len(altoConsumo) > MAX_ALTO {
		eliminarContenedor(altoConsumo[0], rdb)
		altoConsumo = altoConsumo[1:]
	}

	for len(bajoConsumo) > MAX_BAJO {
		eliminarContenedor(bajoConsumo[0], rdb)
		bajoConsumo = bajoConsumo[1:]
	}
}

func guardarMetricas(info *InfoSistema, rdb *redis.Client) {
	timestamp := time.Now().Unix()

	// Guardar memoria como valores simples
	rdb.Set(ctx, "ram:total", info.Memoria.TotalMB, 0)
	rdb.Set(ctx, "ram:libre", info.Memoria.LibreMB, 0)
	rdb.Set(ctx, "ram:usada", info.Memoria.UsadaMB, 0)

	// Guardar serie temporal de RAM usada
	key := fmt.Sprintf("ram:history:%d", timestamp)
	rdb.Set(ctx, key, info.Memoria.UsadaMB, 24*time.Hour)

	// Top 5 por RAM
	procesos := make([]Proceso, len(info.Procesos))
	copy(procesos, info.Procesos)
	sort.Slice(procesos, func(i, j int) bool {
		return procesos[i].RSSKB > procesos[j].RSSKB
	})
	for i, p := range procesos {
		if i >= 5 {
			break
		}
		key := fmt.Sprintf("top_ram:%d", i+1)
		val := fmt.Sprintf(`{"pid":%d,"nombre":"%s","rss_kb":%d}`, p.PID, p.Nombre, p.RSSKB)
		rdb.Set(ctx, key, val, 24*time.Hour)
	}

	// Top 5 por CPU
	sort.Slice(procesos, func(i, j int) bool {
		return procesos[i].CPUPercent > procesos[j].CPUPercent
	})
	for i, p := range procesos {
		if i >= 5 {
			break
		}
		key := fmt.Sprintf("top_cpu:%d", i+1)
		val := fmt.Sprintf(`{"pid":%d,"nombre":"%s","cpu_percent":%d}`, p.PID, p.Nombre, p.CPUPercent)
		rdb.Set(ctx, key, val, 24*time.Hour)
	}

	fmt.Printf("[OK] Métricas guardadas. RAM: %dMB usada / %dMB total\n",
		info.Memoria.UsadaMB, info.Memoria.TotalMB)
}

func main() {
	fmt.Println("==============================================")
	fmt.Println(" DAEMON GESTIONADOR - PROYECTO 2 SO1")
	fmt.Println(" VGOMEZ - 201114493")
	fmt.Println("==============================================")

	rdb := conectarValkey()
	defer rdb.Close()

	cargarModulo()

	fmt.Println("[INFO] Configurando cronjob...")
	ejecutarScript(BASH_DIR + "/setup_cronjob.sh")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n[INFO] Deteniendo daemon...")
		ejecutarScript(BASH_DIR + "/delete_cronjob.sh")
		exec.Command("sudo", "rmmod", "continfo_pr2_so1_201114493").Run()
		fmt.Println("[OK] Daemon detenido correctamente.")
		os.Exit(0)
	}()

	fmt.Println("[INFO] Iniciando loop principal cada 20 segundos...")
	for {
		fmt.Printf("\n[%s] Ejecutando ciclo de monitoreo...\n",
			time.Now().Format("15:04:05"))
		info, err := leerProc()
		if err != nil {
			fmt.Printf("[ERROR] %v\n", err)
		} else {
			guardarMetricas(info, rdb)
			gestionarContenedores(info, rdb)
		}
		time.Sleep(INTERVALO)
	}
}
