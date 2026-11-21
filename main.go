package main

import (
	"database/sql"
	"log"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Transaction struct {
	ID          int
	Type        string
	Category    string
	Amount      float64
	Description string
	Date        string
	Method      string
}

type Budget struct {
	ID       int
	Category string
	Amount   float64
	Spent    float64
}

type PageData struct {
	Transactions []Transaction
	Budgets      []Budget
	Income       float64
	Expenses     float64
	Savings      float64
	Month        string
	Message      string
}

var db *sql.DB

func main() {
	var err error
	db, err = sql.Open("sqlite", "finance.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err = initDB(); err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/", dashboard)
	http.HandleFunc("/transaction/add", addTransaction)
	http.HandleFunc("/transaction/delete", deleteTransaction)
	http.HandleFunc("/budget/add", addBudget)
	http.HandleFunc("/budget/delete", deleteBudget)
	http.HandleFunc("/export", exportCSV)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	log.Println("Finance Tracker running at http://localhost:8080")
	log.Fatal(http.ListenAndServe("127.0.0.1:8080", nil))
}

func initDB() error {
	schema := `
	CREATE TABLE IF NOT EXISTS transactions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		type TEXT NOT NULL CHECK(type IN ('income','expense')),
		category TEXT NOT NULL,
		amount REAL NOT NULL CHECK(amount > 0),
		description TEXT,
		date TEXT NOT NULL,
		method TEXT DEFAULT 'Cash'
	);
	CREATE TABLE IF NOT EXISTS budgets (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		category TEXT NOT NULL,
		amount REAL NOT NULL CHECK(amount > 0),
		month TEXT NOT NULL,
		UNIQUE(category, month)
	);`
	_, err := db.Exec(schema)
	return err
}

func dashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	now := time.Now()
	month := now.Format("2006-01")
	data := PageData{Month: month, Message: r.URL.Query().Get("msg")}

	rows, err := db.Query(`SELECT id,type,category,amount,description,date,method
		FROM transactions ORDER BY date DESC, id DESC LIMIT 100`)
	if err != nil {
		http.Error(w, "Could not load transactions", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var t Transaction
		if err := rows.Scan(&t.ID, &t.Type, &t.Category, &t.Amount, &t.Description, &t.Date, &t.Method); err != nil {
			http.Error(w, "Could not read transactions", http.StatusInternalServerError)
			return
		}
		data.Transactions = append(data.Transactions, t)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "Could not read transactions", http.StatusInternalServerError)
		return
	}

	_ = db.QueryRow(`SELECT COALESCE(SUM(amount),0) FROM transactions WHERE type='income' AND substr(date,1,7)=?`, month).Scan(&data.Income)
	_ = db.QueryRow(`SELECT COALESCE(SUM(amount),0) FROM transactions WHERE type='expense' AND substr(date,1,7)=?`, month).Scan(&data.Expenses)
	data.Savings = data.Income - data.Expenses

	budgetRows, err := db.Query(`SELECT b.id,b.category,b.amount,
		COALESCE((SELECT SUM(t.amount) FROM transactions t
		WHERE t.type='expense' AND t.category=b.category AND substr(t.date,1,7)=b.month),0)
		FROM budgets b WHERE b.month=? ORDER BY b.category`, month)
	if err == nil {
		defer budgetRows.Close()
		for budgetRows.Next() {
			var b Budget
			if budgetRows.Scan(&b.ID, &b.Category, &b.Amount, &b.Spent) == nil {
				data.Budgets = append(data.Budgets, b)
			}
		}
	}

	if err := render(w, data); err != nil {
		log.Println("template:", err)
	}
}

func barWidth(value, other float64) int {
	max := value
	if other > max { max = other }
	if max <= 0 { return 0 }
	width := int(value / max * 100)
	if width < 3 && value > 0 { width = 3 }
	return width
}

func budgetWidth(spent, amount float64) int {
	if amount <= 0 { return 0 }
	width := int(spent / amount * 100)
	if width > 100 { width = 100 }
	if width < 0 { width = 0 }
	return width
}

func budgetPercent(spent, amount float64) float64 {
	if amount <= 0 { return 0 }
	return spent / amount * 100
}

func render(w http.ResponseWriter, data PageData) error {
	// Templates are parsed on each request so editing the HTML needs no rebuild.
	t, err := templateParse()
	if err != nil {
		return err
	}
	return t.Execute(w, data)
}

func templateParse() (*template.Template, error) {
	return template.New("index.html").Funcs(template.FuncMap{
		"barWidth": barWidth,
		"budgetWidth": budgetWidth,
		"budgetPercent": budgetPercent,
		"today": func() string { return time.Now().Format("2006-01-02") },
	}).ParseFiles("templates/index.html")
}

func addTransaction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	typ := r.FormValue("type")
	category := strings.TrimSpace(r.FormValue("category"))
	amount, err := strconv.ParseFloat(r.FormValue("amount"), 64)
	date := r.FormValue("date")
	if (typ != "income" && typ != "expense") || category == "" || err != nil || amount <= 0 || len(date) != 10 {
		http.Redirect(w, r, "/?msg=Please+enter+valid+transaction+details", http.StatusSeeOther)
		return
	}
	_, err = db.Exec(`INSERT INTO transactions(type,category,amount,description,date,method)
		VALUES(?,?,?,?,?,?)`, typ, category, amount, strings.TrimSpace(r.FormValue("description")), date, r.FormValue("method"))
	if err != nil {
		http.Redirect(w, r, "/?msg=Could+not+save+transaction", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/?msg=Transaction+saved", http.StatusSeeOther)
}

func deleteTransaction(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		id, err := strconv.Atoi(r.FormValue("id"))
		if err == nil && id > 0 {
			_, _ = db.Exec("DELETE FROM transactions WHERE id=?", id)
		}
	}
	http.Redirect(w, r, "/?msg=Transaction+deleted", http.StatusSeeOther)
}

func addBudget(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	category := strings.TrimSpace(r.FormValue("category"))
	amount, err := strconv.ParseFloat(r.FormValue("amount"), 64)
	month := r.FormValue("month")
	if category == "" || err != nil || amount <= 0 || len(month) != 7 {
		http.Redirect(w, r, "/?msg=Please+enter+valid+budget+details", http.StatusSeeOther)
		return
	}
	_, err = db.Exec(`INSERT INTO budgets(category,amount,month) VALUES(?,?,?)
		ON CONFLICT(category,month) DO UPDATE SET amount=excluded.amount`, category, amount, month)
	if err != nil {
		http.Redirect(w, r, "/?msg=Could+not+save+budget", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/?msg=Budget+saved", http.StatusSeeOther)
}

func deleteBudget(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		id, err := strconv.Atoi(r.FormValue("id"))
		if err == nil && id > 0 {
			_, _ = db.Exec("DELETE FROM budgets WHERE id=?", id)
		}
	}
	http.Redirect(w, r, "/?msg=Budget+deleted", http.StatusSeeOther)
}

func exportCSV(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="transactions.csv"`)
	w.Write([]byte("Type,Category,Amount,Description,Date,Payment Method\n"))
	rows, err := db.Query("SELECT type,category,amount,description,date,method FROM transactions ORDER BY date DESC")
	if err != nil {
		http.Error(w, "Export failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var typ, category, desc, date, method string
		var amount float64
		if rows.Scan(&typ, &category, &amount, &desc, &date, &method) == nil {
			// CSV quoting: double embedded quotes and wrap each field.
			w.Write([]byte(csvRow(typ, category, strconv.FormatFloat(amount, 'f', 2, 64), desc, date, method)))
		}
	}
}

func csvRow(values ...string) string {
	var out []string
	for _, v := range values {
		out = append(out, `"`+strings.ReplaceAll(v, `"`, `""`)+`"`)
	}
	return strings.Join(out, ",") + "\n"
}
