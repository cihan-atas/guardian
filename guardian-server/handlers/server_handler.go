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
	"guardian.com/server/models"
	"guardian.com/server/services"
)

func CreateServer(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var server models.Server
		if err := json.NewDecoder(r.Body).Decode(&server); err != nil {
			services.Record(db, r, services.AuditLog{
				Action:       services.ActionCreateServer,
				TargetType:   "server",
				Status:       "FAILURE",
				ErrorMessage: "Invalid request body: " + err.Error(),
			})
			http.Error(w, "Geçersiz istek gövdesi", http.StatusBadRequest)
			return
		}
		sqlStatement := `INSERT INTO servers (hostname, ip_address, description) VALUES ($1, $2, $3) RETURNING id, created_at`
		err := db.QueryRow(sqlStatement, server.Hostname, server.IPAddress, server.Description).Scan(&server.ID, &server.CreatedAt)
		if err != nil {
			log.Printf("Veritabanına kayıt eklenemedi: %v", err)
			services.Record(db, r, services.AuditLog{
				Action:       services.ActionCreateServer,
				TargetType:   "server",
				Status:       "FAILURE",
				ErrorMessage: err.Error(),
			})
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		services.Record(db, r, services.AuditLog{
			Action:     services.ActionCreateServer,
			TargetType: "server",
			TargetID:   server.ID,
			Status:     "SUCCESS",
		})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(server)
	}
}
func ListServers(db *sql.DB) http.HandlerFunc {
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

		// Opsiyonel arama: hostname veya IP üzerinde ILIKE.
		search := strings.TrimSpace(r.URL.Query().Get("search"))
		where := ""
		args := []interface{}{}
		if search != "" {
			where = " WHERE (hostname ILIKE $1 OR ip_address ILIKE $1)"
			args = append(args, "%"+search+"%")
		}
		query := fmt.Sprintf(
			"SELECT id, hostname, ip_address, description, created_at FROM servers%s ORDER BY id ASC LIMIT $%d OFFSET $%d",
			where, len(args)+1, len(args)+2)
		countArgs := append([]interface{}{}, args...)
		args = append(args, limit, offset)
		rows, err := db.Query(query, args...)
		if err != nil {
			log.Printf("Veritabanı sunucu listeleme hatası: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		var servers []models.Server
		for rows.Next() {
			var s models.Server
			if err := rows.Scan(&s.ID, &s.Hostname, &s.IPAddress, &s.Description, &s.CreatedAt); err != nil {
				log.Printf("Sunucu verisi okunurken hata: %v", err)
				http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
				return
			}
			servers = append(servers, s)
		}
		if err = rows.Err(); err != nil {
			log.Printf("Satır okuma hatası: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		var totalRecords int
		countQuery := "SELECT COUNT(*) FROM servers" + where
		if err := db.QueryRow(countQuery, countArgs...).Scan(&totalRecords); err != nil {
			log.Printf("Toplam sunucu sayısı alınamadı: %v", err)
		}
		response := struct {
			TotalRecords int             `json:"total_records"`
			Page         int             `json:"page"`
			Limit        int             `json:"limit"`
			Data         []models.Server `json:"data"`
		}{
			TotalRecords: totalRecords,
			Page:         page,
			Limit:        limit,
			Data:         servers,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

func GetServer(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serverIDStr := chi.URLParam(r, "serverID")
		id, err := strconv.Atoi(serverIDStr)
		if err != nil {
			http.Error(w, "Geçersiz sunucu ID'si", http.StatusBadRequest)
			return
		}
		var s models.Server
		query := "SELECT id, hostname, ip_address, description, created_at FROM servers WHERE id = $1"
		err = db.QueryRow(query, id).Scan(&s.ID, &s.Hostname, &s.IPAddress, &s.Description, &s.CreatedAt)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Sunucu bulunamadı", http.StatusNotFound)
				return
			}
			log.Printf("Tek sunucu sorgu hatası: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(s)
	}
}

func UpdateServer(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		serverIDStr := chi.URLParam(r, "serverID")
		id, err := strconv.Atoi(serverIDStr)
		if err != nil {
			http.Error(w, "Geçersiz sunucu ID'si", http.StatusBadRequest)
			return
		}
		var server models.Server
		if err := json.NewDecoder(r.Body).Decode(&server); err != nil {
			http.Error(w, "Geçersiz istek gövdesi", http.StatusBadRequest)
			return
		}
		sqlStatement := `UPDATE servers SET hostname = $1, ip_address = $2, description = $3 WHERE id = $4`
		result, err := db.Exec(sqlStatement, server.Hostname, server.IPAddress, server.Description, id)
		if err != nil {
			log.Printf("Veritabanı güncelleme hatası: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			http.Error(w, "Güncellenecek sunucu bulunamadı", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Sunucu %d başarıyla güncellendi.", id)
	}
}

func PatchServer(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serverIDStr := chi.URLParam(r, "serverID")
		id, err := strconv.Atoi(serverIDStr)
		if err != nil {
			http.Error(w, "Geçersiz sunucu ID'si", http.StatusBadRequest)
			return
		}

		var updates map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
			services.Record(db, r, services.AuditLog{
				Action:       services.ActionPatchServer,
				TargetType:   "server",
				TargetID:     id,
				Status:       "FAILURE",
				ErrorMessage: "Invalid request body: " + err.Error(),
			})
			http.Error(w, "Geçersiz istek gövdesi", http.StatusBadRequest)
			return
		}

		query := "UPDATE servers SET "
		args := make([]interface{}, 0, len(updates)+1)
		i := 1
		for key, value := range updates {
			if key == "hostname" || key == "ip_address" || key == "description" {
				query += fmt.Sprintf("%s = $%d, ", key, i)
				args = append(args, value)
				i++
			} else {
				http.Error(w, fmt.Sprintf("Bilinmeyen veya güncellenemeyen alan: %s", key), http.StatusBadRequest)
				return
			}
		}
		query = strings.TrimSuffix(query, ", ")
		query += fmt.Sprintf(" WHERE id = $%d RETURNING id, hostname, ip_address, description, created_at", i)
		args = append(args, id)

		var updatedServer models.Server
		err = db.QueryRow(query, args...).Scan(
			&updatedServer.ID,
			&updatedServer.Hostname,
			&updatedServer.IPAddress,
			&updatedServer.Description,
			&updatedServer.CreatedAt,
		)
		if err != nil {
			services.Record(db, r, services.AuditLog{
				Action:       services.ActionPatchServer,
				TargetType:   "server",
				TargetID:     id,
				Status:       "FAILURE",
				ErrorMessage: err.Error(),
			})
			if err == sql.ErrNoRows {
				http.Error(w, "Güncellenecek sunucu bulunamadı", http.StatusNotFound)
			} else {
				log.Printf("Veritabanı PATCH hatası: %v", err)
				http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			}
			return
		}

		services.Record(db, r, services.AuditLog{
			Action:     services.ActionPatchServer,
			TargetType: "server",
			TargetID:   id,
			Status:     "SUCCESS",
		})

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(updatedServer)
	}
}

func DeleteServer(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serverIDStr := chi.URLParam(r, "serverID")
		id, err := strconv.Atoi(serverIDStr)
		if err != nil {
			http.Error(w, "Geçersiz sunucu ID'si", http.StatusBadRequest)
			return
		}

		var usageCount int
		checkQuery := "SELECT COUNT(*) FROM access_rules WHERE server_id = $1 AND status != 'expired'"
		err = db.QueryRow(checkQuery, id).Scan(&usageCount)
		if err != nil {
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		if usageCount > 0 {
			errMsg := fmt.Sprintf("Sunucu silinemiyor. Bu sunucu %d adet aktif/bekleyen kural tarafından kullanılıyor.", usageCount)
			services.Record(db, r, services.AuditLog{
				Action:       services.ActionDeleteServer,
				TargetType:   "server",
				TargetID:     id,
				Status:       "FAILURE",
				ErrorMessage: errMsg,
			})
			http.Error(w, errMsg, http.StatusConflict)
			return
		}

		result, err := db.Exec("DELETE FROM servers WHERE id = $1", id)
		if err != nil {
			services.Record(db, r, services.AuditLog{
				Action:       services.ActionDeleteServer,
				TargetType:   "server",
				TargetID:     id,
				Status:       "FAILURE",
				ErrorMessage: err.Error(),
			})
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		if rowsAffected, _ := result.RowsAffected(); rowsAffected == 0 {
			http.Error(w, "Sunucu bulunamadı", http.StatusNotFound)
			return
		}

		services.Record(db, r, services.AuditLog{
			Action:     services.ActionDeleteServer,
			TargetType: "server",
			TargetID:   id,
			Status:     "SUCCESS",
		})
		w.WriteHeader(http.StatusNoContent)
	}
}
