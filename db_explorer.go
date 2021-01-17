package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

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

type Scanable interface {
	Scan()
}

func NewDbExplorer(db *sql.DB) (http.Handler, error) {
	tables, err := getTables(db)
	if err != nil {
		return nil, err
	}

	handler := &Handler{DB: db, tables: tables}
	return handler, nil
}

func (t *Table) GetColNames() []string {
	res := make([]string, len(t.Columns), len(t.Columns))
	for i, col := range t.Columns {
		res[i] = col.Name
	}
	return res
}

func (t *Table) GetPrimaryCol() *TableColumn {
	var column *TableColumn
	for _, col := range t.Columns {
		if col.Key == "PRI" {
			column = &col
			break
		}
	}
	return column
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

var _URLSection = "([^?/.]+)"
var rowsURLRe = regexp.MustCompile("^/" + _URLSection + "$")
var rowURLRe = regexp.MustCompile("^/" + _URLSection + "/" + _URLSection + "$")

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		h.GetTables(w, r)
	} else if rowsURLRe.MatchString(r.URL.Path) {
		if r.Method == http.MethodGet {
			h.GetRows(w, r)
		} else if r.Method == http.MethodPost {
			h.InsertRow(w, r)
		} else {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte("StatusMethodNotAllowed"))
		}
	} else if rowURLRe.MatchString(r.URL.Path) {
		if r.Method == http.MethodGet {
			h.GetRow(w, r)
		} else if r.Method == http.MethodPut {
			h.UpdateRow(w, r)
		} else if r.Method == http.MethodDelete {
			h.DeleteRow(w, r)
		} else {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte("StatusMethodNotAllowed"))
		}

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

	limit, err := strconv.Atoi(r.FormValue("limit"))
	if err != nil || limit < 0 {
		limit = 5
	}

	offset, err := strconv.Atoi(r.FormValue("offset"))
	if err != nil || offset < 0 {
		offset = 0
	}

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

	rows, err := h.DB.Query("SELECT * FROM "+table.Name+" LIMIT ? OFFSET ?", limit, offset)
	defer rows.Close()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}

	data, err := rowsToMap(rows)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	writeJSON(w, data)
}

func (h *Handler) GetRow(w http.ResponseWriter, r *http.Request) {
	params := rowURLRe.FindStringSubmatch(r.URL.Path)
	fmt.Println("params: ", params)

	tableName := params[1]
	rowIDStr := params[2]

	rowID, err := strconv.Atoi(rowIDStr)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

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

	prCol := table.GetPrimaryCol()
	if prCol == nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, errors.New("table have not primary column"))
		return
	}

	row := h.DB.QueryRow("SELECT * FROM "+table.Name+" WHERE "+table.Name+"."+prCol.Name+" = ?", rowID)
	data, err := rowToMap(row, table.GetColNames())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	writeJSON(w, data)
}

func (h *Handler) InsertRow(w http.ResponseWriter, r *http.Request) {
	params := rowsURLRe.FindStringSubmatch(r.URL.Path)
	fmt.Println("params: ", params)

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

	var bData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&bData); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeErr(w, err)
		return
	}

	colNames := make([]string, 0, len(table.Columns))
	values := make([]interface{}, 0, len(colNames))
	for _, col := range table.Columns {
		if col.Key != "PRI" {
			colNames = append(colNames, col.Name)
			val, exists := bData[col.Name]
			if (!exists || val == nil) && col.Null == "NO" {
				w.WriteHeader(http.StatusBadRequest)
				writeErr(w, errors.New("Field: '"+col.Name+"' is required"))
				return
			}
			values = append(values, val)
		}
	}

	tmpls := make([]string, len(colNames))
	for i := range tmpls {
		tmpls[i] = "?"
	}

	sqlSt := `INSERT INTO ` + table.Name + ` (` + strings.Join(colNames, ",") + `)
	VALUES (` + strings.Join(tmpls, ",") + `)`

	fmt.Println("sqlSt:", sqlSt)
	fmt.Println("values:", values)

	_, err := h.DB.Exec(sqlSt, values...)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("successfully created"))
}

func (h *Handler) UpdateRow(w http.ResponseWriter, r *http.Request) {
	params := rowURLRe.FindStringSubmatch(r.URL.Path)
	fmt.Println("params: ", params)

	tableName := params[1]
	rowIDStr := params[2]

	rowID, err := strconv.Atoi(rowIDStr)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

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

	var bData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&bData); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeErr(w, err)
		return
	}

	colNames := make([]string, 0, len(table.Columns))
	values := make([]interface{}, 0, len(colNames))
	for _, col := range table.Columns {
		if col.Key != "PRI" {
			colNames = append(colNames, col.Name)

			val, exists := bData[col.Name]
			if (!exists || val == nil) && col.Null == "NO" {
				w.WriteHeader(http.StatusBadRequest)
				writeErr(w, errors.New("Field: '"+col.Name+"' is required"))
				return
			}
			values = append(values, val)
		}
	}

	values = append(values, rowID)

	var sqlCols string
	for i, col := range colNames {
		sqlCols += col + " = ?"
		if len(colNames)-1 != i {
			sqlCols += ","
		}
	}

	prCol := table.GetPrimaryCol()
	if prCol == nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, errors.New("table have not primary column"))
		return
	}

	sqlSt := `UPDATE ` + table.Name + ` SET ` + sqlCols + ` WHERE ` + prCol.Name + ` = ?`

	fmt.Println("sqlSt:", sqlSt)
	fmt.Println("values:", values)

	_, err = h.DB.Exec(sqlSt, values...)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte("successfully created"))
}

func (h *Handler) DeleteRow(w http.ResponseWriter, r *http.Request) {
	params := rowURLRe.FindStringSubmatch(r.URL.Path)
	fmt.Println("params: ", params)

	tableName := params[1]
	rowIDStr := params[2]

	rowID, err := strconv.Atoi(rowIDStr)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

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

	prCol := table.GetPrimaryCol()
	if prCol == nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, errors.New("table have not primary column"))
		return
	}

	sqlSt := `DELETE FROM ` + table.Name + ` WHERE ` + prCol.Name + ` = ?`

	fmt.Println("sqlSt:", sqlSt)

	res, err := h.DB.Exec(sqlSt, rowID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}

	if aff,_ := res.RowsAffected(); aff == 0 {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found row"))	
		return
	} 

	w.WriteHeader(http.StatusNoContent)
	w.Write([]byte("successfully deleted"))
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

func rowsToMap(rows *sql.Rows) ([]map[string]interface{}, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	values := make([]interface{}, len(columns))

	scanArgs := make([]interface{}, len(values))
	for i := range values {
		scanArgs[i] = &values[i]
	}

	data := []map[string]interface{}{}

	for rows.Next() {
		err = rows.Scan(scanArgs...)
		if err != nil {
			return nil, err
		}
		results := parseRowData(columns, values)
		data = append(data, results)
	}

	return data, nil
}

func rowToMap(row *sql.Row, columns []string) (map[string]interface{}, error) {
	values := make([]interface{}, len(columns))
	scanArgs := make([]interface{}, len(values))
	for i := range values {
		scanArgs[i] = &values[i]
	}
	err := row.Scan(scanArgs...)
	if err != nil {
		return nil, err
	}
	result := parseRowData(columns, values)
	return result, nil
}

func parseRowData(columns []string, values []interface{}) map[string]interface{} {
	results := make(map[string]interface{})
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
	return results
}
