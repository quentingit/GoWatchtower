package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// Structure pour stocker la config et les résultats
type Config struct {
	Interval int      `json:"interval"`
	URLs     []string `json:"urls"`
}

type URLStatus struct {
	URL        string
	Status     string
	StatusCode int
	CheckedAt  time.Time
}

var (
	statusMap = make(map[string]URLStatus)
	mu        sync.Mutex
)

// Charge la config depuis le fichier JSON
func loadConfig(filename string) (*Config, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// Initialise le fichier de logs
func initLogFile() *os.File {
	logFile, err := os.OpenFile("monitor.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Erreur lors de l'ouverture du fichier de logs: %v", err)
	}
	log.SetOutput(logFile)
	return logFile
}

// Vérifie l'état d'une URL
func checkURL(url string) {
	start := time.Now()
	resp, err := http.Get(url)
	duration := time.Since(start)

	timestamp := time.Now().Format("2006-01-02 15:04:05")

	mu.Lock()
	defer mu.Unlock()

	if err != nil {
		statusMap[url] = URLStatus{URL: url, Status: "❌ DOWN", StatusCode: 0, CheckedAt: time.Now()}
		logMessage := fmt.Sprintf("[%s] [❌] %s est inaccessible (%v)\n", timestamp, url, err)
		fmt.Print(logMessage)
		log.Println(logMessage)
		return
	}
	defer resp.Body.Close()

	status := "✅ UP"
	if resp.StatusCode >= 400 {
		status = "⚠️ PROBLÈME"
	}

	statusMap[url] = URLStatus{URL: url, Status: status, StatusCode: resp.StatusCode, CheckedAt: time.Now()}

	logMessage := fmt.Sprintf("[%s] [%s] %s (%d) - %v\n", timestamp, status, url, resp.StatusCode, duration)
	fmt.Print(logMessage)
	log.Println(logMessage)
}

// Boucle de monitoring en arrière-plan
func startMonitoring(config *Config) {
	ticker := time.NewTicker(time.Duration(config.Interval) * time.Second)
	defer ticker.Stop()

	fmt.Println("\n🔍 Vérification initiale...")
	for _, url := range config.URLs {
		go checkURL(url)
	}

	for {
		<-ticker.C
		fmt.Println("\n🔍 Vérification périodique...")
		for _, url := range config.URLs {
			go checkURL(url)
		}
	}
}

// Route unique qui sert le HTML et met à jour le tableau
func homeHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("templates/index.html")
	if err != nil {
		http.Error(w, "Erreur de template", http.StatusInternalServerError)
		return
	}
	mu.Lock()
	defer mu.Unlock()
	tmpl.Execute(w, statusMap)
}

func main() {
	config, err := loadConfig("urls.json")
	if err != nil {
		fmt.Println("Erreur lors du chargement de la config :", err)
		return
	}

	logFile := initLogFile()
	defer logFile.Close()

	go startMonitoring(config) // Lancement du monitoring en arrière-plan

	// Une seule route pour tout
	http.HandleFunc("/", homeHandler)

	fmt.Println("📡 Serveur lancé sur http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}