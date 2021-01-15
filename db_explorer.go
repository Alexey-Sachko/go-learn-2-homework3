package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

// тут вы пишете код
// обращаю ваше внимание - в этом задании запрещены глобальные переменные

type Handler struct {
	DB     *sql.DB
	tables []Table
}

type TableColumn struct {
	Name    string
	Type    string
	Null    string
	Key     string
	Default interface{}
	Extra   interface{}
}

type Table struct {
	Name    string
	Columns []TableColumn
}

func NewDbExplorer(db *sql.DB) (http.Handler, error) {
	tables, err := getTables(db)
	if err != nil {
		return nil, err
	}

	handler := Handler{DB: db, tables: tables}

	mux := &http.ServeMux{}
	mux.HandleFunc("/", method(http.MethodGet, handler.GetTables))
	return mux, nil
}

func getTables(db *sql.DB) ([]Table, error) {
	tables := []Table{}

	rows, err := db.Query("SHOW TABLES FROM golang")
	if err != nil {
		return tables, err
	}

	for rows.Next() {
		table := Table{}
		err := rows.Scan(&table.Name)
		if err != nil {
			return tables, err
		}

		table.Columns, err = getColumns(db, table.Name)
		if err != nil {
			return tables, err
		}

		tables = append(tables, table)
	}

	return tables, nil
}

func getColumns(db *sql.DB, tableName string) ([]TableColumn, error) {
	columns := []TableColumn{}

	rows, err := db.Query("SHOW COLUMNS FROM " + tableName)
	if err != nil {
		return columns, err
	}

	for rows.Next() {
		col := TableColumn{}
		err := rows.Scan(&col.Name, &col.Type, &col.Null, &col.Key, &col.Default,&col.Extra)
		if err != nil {
			return columns, err
		}

		columns = append(columns, col)
	}

	return columns, nil
}

func (h *Handler) GetTables(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	writeJSON(w, h.tables)
}

// utils
func writeErr(w http.ResponseWriter, err error) {
	w.Write([]byte(err.Error()))
}

func writeJSON(w http.ResponseWriter, d interface{}) {
	b, err := json.MarshalIndent(d, "", "  ")
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
