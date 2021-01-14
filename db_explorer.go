package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

// тут вы пишете код
// обращаю ваше внимание - в этом задании запрещены глобальные переменные

type Handler struct {
	DB *sql.DB
}

func NewDbExplorer(db *sql.DB) (http.Handler, error) {
	handler := Handler{DB: db}

	mux := &http.ServeMux{}
	mux.HandleFunc("/", method(http.MethodGet, handler.getTables))
	return mux, nil
}

func (h *Handler) getTables(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query("SHOW TABLES FROM golang")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w,err)
		return
	}

	tables := []string{}

	for rows.Next() {
		var table string
		err := rows.Scan(&table)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			writeErr(w,err)
			return
		}
		
		tables = append(tables, table)
	}

	writeJSON(w, tables)
}

// utils
func writeErr(w http.ResponseWriter, err error) {
	w.Write([]byte(err.Error()))
}

func writeJSON(w http.ResponseWriter, d interface{}) {
	b, err := json.Marshal(d)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}

	w.Write(b)
}

func method(m string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// TODO method
		next(w, r)
	}
}