// guardian/guardian-server/handlers/user_handler.go

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

func CreateSystemUser(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// DİKKAT: JSON'dan gelen 'description' alanı artık doğrudan
		// models.SystemUser içindeki sql.NullString alanına map edilecek.
		// Eğer JSON'da description alanı yoksa veya null ise, user.Description.Valid false olacak.
		var user models.SystemUser
		if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
			services.Record(db, r, services.AuditLog{
				Action:       services.ActionCreateUser,
				TargetType:   "user",
				Status:       "FAILURE",
				ErrorMessage: "Invalid request body: " + err.Error(),
			})
			http.Error(w, "Geçersiz istek gövdesi: "+err.Error(), http.StatusBadRequest)
			return
		}

		sqlStatement := `INSERT INTO system_users (username, description) VALUES ($1, $2) RETURNING id, created_at`
		// user.Description zaten sql.NullString tipinde olduğu için doğrudan veritabanına gönderilebilir.
		err := db.QueryRow(sqlStatement, user.Username, user.Description).Scan(&user.ID, &user.CreatedAt)
		if err != nil {
			services.Record(db, r, services.AuditLog{
				Action:       services.ActionCreateUser,
				TargetType:   "user",
				Status:       "FAILURE",
				ErrorMessage: err.Error(),
			})
			http.Error(w, "Veritabanı hatası, muhtemelen kullanıcı adı zaten mevcut.", http.StatusInternalServerError)
			return
		}

		services.Record(db, r, services.AuditLog{
			Action:     services.ActionCreateUser,
			TargetType: "user",
			TargetID:   user.ID,
			Status:     "SUCCESS",
		})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(user)
	}
}

func PatchSystemUser(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := strconv.Atoi(chi.URLParam(r, "userID"))
		if err != nil {
			http.Error(w, "Geçersiz kullanıcı ID'si", http.StatusBadRequest)
			return
		}

		var updates map[string]*string // DİKKAT: Artık string pointer'ı bekliyoruz ki 'null' gönderebilelim.
		if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
			http.Error(w, "Geçersiz istek gövdesi", http.StatusBadRequest)
			return
		}

		newDesc, ok := updates["description"]
		if !ok || len(updates) > 1 {
			http.Error(w, "Sadece 'description' alanı güncellenebilir.", http.StatusBadRequest)
			return
		}

		// Gelen değeri sql.NullString'e çevirelim.
		var descValue sql.NullString
		if newDesc != nil {
			descValue.String = *newDesc
			descValue.Valid = true
		} else {
			// Eğer JSON'da "description": null gelirse, Valid false olur.
			descValue.Valid = false
		}

		query := "UPDATE system_users SET description = $1 WHERE id = $2 RETURNING id, username, description, created_at"
		var updatedUser models.SystemUser
		err = db.QueryRow(query, descValue, userID).Scan(&updatedUser.ID, &updatedUser.Username, &updatedUser.Description, &updatedUser.CreatedAt)
		if err != nil {
			services.Record(db, r, services.AuditLog{
				Action:       services.ActionPatchUser,
				TargetType:   "user",
				TargetID:     userID,
				Status:       "FAILURE",
				ErrorMessage: err.Error(),
			})
			if err == sql.ErrNoRows {
				http.Error(w, "Güncellenecek kullanıcı bulunamadı", http.StatusNotFound)
			} else {
				http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			}
			return
		}

		services.Record(db, r, services.AuditLog{
			Action:     services.ActionPatchUser,
			TargetType: "user",
			TargetID:   userID,
			Status:     "SUCCESS",
		})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(updatedUser)
	}
}

func DeleteSystemUser(db *sql.DB) http.HandlerFunc {
	// Bu fonksiyonda değişiklik yok, çünkü SystemUser modelini doğrudan kullanmıyor.
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := strconv.Atoi(chi.URLParam(r, "userID"))
		if err != nil {
			http.Error(w, "Geçersiz kullanıcı ID'si", http.StatusBadRequest)
			return
		}

		var usageCount int
		checkQuery := "SELECT COUNT(*) FROM access_rules WHERE system_user_id = $1 AND status != 'expired'"
		err = db.QueryRow(checkQuery, userID).Scan(&usageCount)
		if err != nil {
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}

		if usageCount > 0 {
			errMsg := fmt.Sprintf("Kullanıcı silinemiyor. Bu kullanıcı %d adet aktif/bekleyen kural tarafından kullanılıyor.", usageCount)
			services.Record(db, r, services.AuditLog{
				Action:       services.ActionDeleteUser,
				TargetType:   "user",
				TargetID:     userID,
				Status:       "FAILURE",
				ErrorMessage: errMsg,
			})
			http.Error(w, errMsg, http.StatusConflict)
			return
		}

		result, err := db.Exec("DELETE FROM system_users WHERE id = $1", userID)
		if err != nil {
			services.Record(db, r, services.AuditLog{
				Action:       services.ActionDeleteUser,
				TargetType:   "user",
				TargetID:     userID,
				Status:       "FAILURE",
				ErrorMessage: err.Error(),
			})
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		if rowsAffected, _ := result.RowsAffected(); rowsAffected == 0 {
			http.Error(w, "Silinecek kullanıcı bulunamadı", http.StatusNotFound)
			return
		}

		services.Record(db, r, services.AuditLog{
			Action:     services.ActionDeleteUser,
			TargetType: "user",
			TargetID:   userID,
			Status:     "SUCCESS",
		})
		w.WriteHeader(http.StatusNoContent)
	}
}

func ListSystemUsers(db *sql.DB) http.HandlerFunc {
	// Bu fonksiyon, SystemUser modelini kullandığı için sql.NullString değişikliğinden etkilenir,
	// ancak Scan metodu sql.NullString'i doğal olarak desteklediği için kodda bir değişiklik gerekmez.
	// Hata bu fonksiyonda olduğu için, model değişikliğinin bu hatayı çözmesi gerekir.
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

		query := "SELECT id, username, description, created_at FROM system_users ORDER BY id ASC LIMIT $1 OFFSET $2"
		rows, err := db.Query(query, limit, offset)
		if err != nil {
			log.Printf("Veritabanı system_users sorgu hatası: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var users []models.SystemUser
		for rows.Next() {
			var u models.SystemUser
			if err := rows.Scan(&u.ID, &u.Username, &u.Description, &u.CreatedAt); err != nil {
				log.Printf("System user verisi okunurken hata: %v", err)
				http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
				return
			}
			users = append(users, u)
		}
		if err = rows.Err(); err != nil {
			log.Printf("Satır okuma hatası (system_users): %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}

		var totalRecords int
		countQuery := "SELECT COUNT(*) FROM system_users"
		db.QueryRow(countQuery).Scan(&totalRecords)

		response := struct {
			TotalRecords int                 `json:"total_records"`
			Page         int                 `json:"page"`
			Limit        int                 `json:"limit"`
			Data         []models.SystemUser `json:"data"`
		}{
			TotalRecords: totalRecords,
			Page:         page,
			Limit:        limit,
			Data:         users,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

func GetSystemUser(db *sql.DB) http.HandlerFunc {
	// Bu fonksiyon da sql.NullString değişikliğinden etkilenir,
	// ancak Scan metodu sayesinde kodda değişiklik gerekmez.
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(chi.URLParam(r, "userID"))
		if err != nil {
			http.Error(w, "Geçersiz kullanıcı ID'si", http.StatusBadRequest)
			return
		}

		var user models.SystemUser
		query := "SELECT id, username, description, created_at FROM system_users WHERE id = $1"
		err = db.QueryRow(query, id).Scan(&user.ID, &user.Username, &user.Description, &user.CreatedAt)

		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Kullanıcı bulunamadı", http.StatusNotFound)
				return
			}
			log.Printf("Tek kullanıcı sorgu hatası: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(user)
	}
}
