package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type listing struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Price       string    `json:"price"`
	City        string    `json:"city"`
	CreatedAt   time.Time `json:"created_at"`
}

func List(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(
			`SELECT id, title, description, price, city, created_at
				FROM listings
				ORDER BY created_at DESC
				LIMIT 100`)
		if err != nil {
			log.Printf("query: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		listings := []listing{}

		for rows.Next() {
			var l listing
			if err := rows.Scan(&l.ID, &l.Title, &l.Description, &l.Price, &l.City, &l.CreatedAt); err != nil {
				log.Printf("rows.scan: %v", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}

			listings = append(listings, l)
		}

		if err := rows.Err(); err != nil {
			log.Printf("rows.err: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		_ = json.NewEncoder(w).Encode(listings)
	}
}

func DeleteListing(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		_, err := db.Exec(
			`DELETE FROM listings WHERE id = $1`, id)
		if err != nil {
			log.Printf("delete: %w", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
