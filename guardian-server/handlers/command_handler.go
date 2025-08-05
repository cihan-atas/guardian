package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"guardian.com/server/services"
)

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
