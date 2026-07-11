package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"guardian.com/server/services"
)

// searchSessionLimit, global komut aramasının taradığı en yeni oturum sayısı;
// çok sayıda kayıt biriktiğinde tarama maliyetini sınırlar.
const searchSessionLimit = 1000

// SearchCommands, tüm (son N) oturumlarda komut metni arar
// (GET /api/commands/search?q=<terim>&limit=<n>).
func SearchCommands(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		if len([]rune(q)) < 2 {
			http.Error(w, "Arama terimi en az 2 karakter olmalıdır.", http.StatusBadRequest)
			return
		}
		maxResults, err := strconv.Atoi(r.URL.Query().Get("limit"))
		if err != nil || maxResults < 1 || maxResults > 500 {
			maxResults = 100
		}

		matches, err := services.SearchCommands(db, q, searchSessionLimit, maxResults)
		if err != nil {
			log.Printf("Komut arama hatası: %v", err)
			http.Error(w, "Sunucu hatası.", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, matches)
	}
}

func GetSessionCommands(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, err := strconv.Atoi(chi.URLParam(r, "sessionID"))
		if err != nil {
			http.Error(w, "Geçersiz oturum ID'si", http.StatusBadRequest)
			return
		}

		log.Printf("Oturum komut listesi isteği alındı: Session ID %d", sessionID)

		details, err := services.ParseSessionEvents(db, sessionID)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, fmt.Sprintf("Oturum ID %d bulunamadı", sessionID), http.StatusNotFound)
			} else {
				log.Printf("Oturum ayrıştırılırken hata: %v", err)
				http.Error(w, "Sunucu hatası: Oturum ayrıştırılamadı", http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(details)
	}
}
