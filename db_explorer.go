package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"regexp"

	// "strings"

	"strconv"
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

	handler := &Handler{DB: db, tables: tables}
	return handler, nil
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
		err := rows.Scan(&col.Name, &col.Type, &col.Null, &col.Key, &col.Default, &col.Extra)
		if err != nil {
			return columns, err
		}

		columns = append(columns, col)
	}

	return columns, nil
}

var rowsURLRe = regexp.MustCompile("/([^?/.]+)$")

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		h.GetTables(w, r)
	} else if rowsURLRe.MatchString(r.URL.Path) {
		h.GetRows(w, r)
	} else {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
	}
}

func (h *Handler) GetTables(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	writeJSON(w, h.tables)
}

func (h *Handler) GetRows(w http.ResponseWriter, r *http.Request) {
	params := rowsURLRe.FindStringSubmatch(r.URL.Path)
	tableName := params[1]

	var table *Table
	for _, t := range h.tables {
		if t.Name == tableName {
			table = &t
			break
		}
	}

	if table == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	rows, err := h.DB.Query("SELECT * FROM " + table.Name)
	defer rows.Close()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	writeJSON(w, rowsToMap(rows))
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

func rowsToMap(rows *sql.Rows) []map[string]interface{} {
	columns, err := rows.Columns()
	if err != nil {
		panic(err.Error())
	}

	values := make([]interface{}, len(columns))

	scanArgs := make([]interface{}, len(values))
	for i := range values {
		scanArgs[i] = &values[i]
	}

	results := make(map[string]interface{})
	data := []map[string]interface{}{}

	for rows.Next() {
		err = rows.Scan(scanArgs...)
		if err != nil {
			panic(err.Error())
		}

		for i, value := range values {
			switch value.(type) {
				case nil:
					results[columns[i]] = nil

				case []byte:
					s := string(value.([]byte))
					x, err := strconv.Atoi(s)

					if err != nil {
						results[columns[i]] = s
					} else {
						results[columns[i]] = x
					}


				default:
					results[columns[i]] = value
			}
		}

		data = append(data, results)
	}

	return data
}