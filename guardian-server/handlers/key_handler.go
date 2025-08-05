package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"guardian.com/server/models"
	"guardian.com/server/services"
)

func CreatePublicKey(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var pk models.PublicKey
		if err := json.NewDecoder(r.Body).Decode(&pk); err != nil {
			services.Record(db, r, services.AuditLog{
				Action:       services.ActionCreateKey,
				TargetType:   "key",
				Status:       "FAILURE",
				ErrorMessage: "Invalid request body: " + err.Error(),
			})
			http.Error(w, "Geçersiz istek gövdesi: "+err.Error(), http.StatusBadRequest)
			return
		}

		fingerprint, err := services.GenerateFingerprint(pk.SshPublicKey)
		if err != nil {
			services.Record(db, r, services.AuditLog{
				Action:       services.ActionCreateKey,
				TargetType:   "key",
				Status:       "FAILURE",
				ErrorMessage: "Invalid public key format: " + err.Error(),
			})
			http.Error(w, fmt.Sprintf("Geçersiz public anahtar formatı: %v", err), http.StatusBadRequest)
			return
		}
		pk.FingerprintSHA256 = fingerprint

		sqlStatement := `INSERT INTO public_keys (key_name, ssh_public_key, fingerprint_sha256) VALUES ($1, $2, $3) RETURNING id, created_at`
		err = db.QueryRow(sqlStatement, pk.KeyName, pk.SshPublicKey, pk.FingerprintSHA256).Scan(&pk.ID, &pk.CreatedAt)
		if err != nil {
			services.Record(db, r, services.AuditLog{
				Action:       services.ActionCreateKey,
				TargetType:   "key",
				Status:       "FAILURE",
				ErrorMessage: err.Error(),
			})
			http.Error(w, "Veritabanı hatası, muhtemelen anahtar veya parmak izi zaten mevcut.", http.StatusInternalServerError)
			return
		}

		services.Record(db, r, services.AuditLog{
			Action:     services.ActionCreateKey,
			TargetType: "key",
			TargetID:   pk.ID,
			Status:     "SUCCESS",
		})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(pk)
	}
}

func PatchPublicKey(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(chi.URLParam(r, "keyID"))
		if err != nil {
			http.Error(w, "Geçersiz anahtar ID'si", http.StatusBadRequest)
			return
		}

		var updates map[string]string
		if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
			http.Error(w, "Geçersiz istek gövdesi", http.StatusBadRequest)
			return
		}

		newKeyName, ok := updates["key_name"]
		if !ok || len(updates) > 1 {
			http.Error(w, "Sadece 'key_name' alanı güncellenebilir.", http.StatusBadRequest)
			return
		}

		query := `UPDATE public_keys SET key_name = $1 WHERE id = $2 RETURNING id, key_name, ssh_public_key, fingerprint_sha256, created_at`
		var updatedKey models.PublicKey
		err = db.QueryRow(query, newKeyName, id).Scan(&updatedKey.ID, &updatedKey.KeyName, &updatedKey.SshPublicKey, &updatedKey.FingerprintSHA256, &updatedKey.CreatedAt)
		if err != nil {
			services.Record(db, r, services.AuditLog{
				Action:       services.ActionPatchKey,
				TargetType:   "key",
				TargetID:     id,
				Status:       "FAILURE",
				ErrorMessage: err.Error(),
			})
			if err == sql.ErrNoRows {
				http.Error(w, "Güncellenecek anahtar bulunamadı", http.StatusNotFound)
			} else {
				http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			}
			return
		}

		services.Record(db, r, services.AuditLog{
			Action:     services.ActionPatchKey,
			TargetType: "key",
			TargetID:   id,
			Status:     "SUCCESS",
		})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(updatedKey)
	}
}

func DeletePublicKey(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		keyID, err := strconv.Atoi(chi.URLParam(r, "keyID"))
		if err != nil {
			http.Error(w, "Geçersiz anahtar ID'si", http.StatusBadRequest)
			return
		}

		var usageCount int
		checkQuery := "SELECT COUNT(*) FROM access_rules WHERE public_key_id = $1 AND status != 'expired'"
		err = db.QueryRow(checkQuery, keyID).Scan(&usageCount)
		if err != nil {
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}

		if usageCount > 0 {
			errMsg := fmt.Sprintf("Anahtar silinemiyor. Bu anahtar %d adet aktif/bekleyen kural tarafından kullanılıyor.", usageCount)
			services.Record(db, r, services.AuditLog{
				Action:       services.ActionDeleteKey,
				TargetType:   "key",
				TargetID:     keyID,
				Status:       "FAILURE",
				ErrorMessage: errMsg,
			})
			http.Error(w, errMsg, http.StatusConflict)
			return
		}

		result, err := db.Exec("DELETE FROM public_keys WHERE id = $1", keyID)
		if err != nil {
			services.Record(db, r, services.AuditLog{
				Action:       services.ActionDeleteKey,
				TargetType:   "key",
				TargetID:     keyID,
				Status:       "FAILURE",
				ErrorMessage: err.Error(),
			})
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		if rowsAffected, _ := result.RowsAffected(); rowsAffected == 0 {
			http.Error(w, "Silinecek anahtar bulunamadı", http.StatusNotFound)
			return
		}

		services.Record(db, r, services.AuditLog{
			Action:     services.ActionDeleteKey,
			TargetType: "key",
			TargetID:   keyID,
			Status:     "SUCCESS",
		})
		w.WriteHeader(http.StatusNoContent)
	}
}

func ListPublicKeys(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := strconv.Atoi(r.URL.Query().Get("page"))
		if err != nil || page < 1 {
			page = 1
		}
		limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
		if err != nil || limit < 1 {
			limit = 8
		}
		offset := (page - 1) * limit

		query := "SELECT id, key_name, ssh_public_key, fingerprint_sha256, created_at FROM public_keys ORDER BY id ASC LIMIT $1 OFFSET $2"
		rows, err := db.Query(query, limit, offset)
		if err != nil {
			log.Printf("Veritabanı public_keys sorgu hatası: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var keys []models.PublicKey
		for rows.Next() {
			var k models.PublicKey
			if err := rows.Scan(&k.ID, &k.KeyName, &k.SshPublicKey, &k.FingerprintSHA256, &k.CreatedAt); err != nil {
				log.Printf("Public key verisi okunurken hata: %v", err)
				http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
				return
			}
			keys = append(keys, k)
		}
		if err = rows.Err(); err != nil {
			log.Printf("Satır okuma hatası (public_keys): %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}

		var totalRecords int
		countQuery := "SELECT COUNT(*) FROM public_keys"
		db.QueryRow(countQuery).Scan(&totalRecords)

		response := struct {
			TotalRecords int                `json:"total_records"`
			Page         int                `json:"page"`
			Limit        int                `json:"limit"`
			Data         []models.PublicKey `json:"data"`
		}{
			TotalRecords: totalRecords,
			Page:         page,
			Limit:        limit,
			Data:         keys,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

func GetPublicKey(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(chi.URLParam(r, "keyID"))
		if err != nil {
			http.Error(w, "Geçersiz anahtar ID'si", http.StatusBadRequest)
			return
		}

		var pk models.PublicKey
		query := "SELECT id, key_name, ssh_public_key, fingerprint_sha256, created_at FROM public_keys WHERE id = $1"
		err = db.QueryRow(query, id).Scan(&pk.ID, &pk.KeyName, &pk.SshPublicKey, &pk.FingerprintSHA256, &pk.CreatedAt)

		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Anahtar bulunamadı", http.StatusNotFound)
				return
			}
			log.Printf("Tek anahtar sorgu hatası: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pk)
	}
}
