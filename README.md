# Smart Personal Finance and Expense Tracker

A beginner-friendly local college capstone project built with Go, HTML, CSS, JavaScript, and SQLite.

## Requirements
- Go 1.23 or newer
- Internet only for the first dependency download (`go mod tidy`). The application itself does not call external APIs or require internet after setup.

## Run on Windows PowerShell

```powershell
cd finance-tracker
go mod tidy
go run .
```

Open http://localhost:8080

Stop the server with `Ctrl+C`.

## Database
The app creates `finance.db` automatically in the project folder. Your data remains on this computer and persists between runs. Back up this file to back up your records.

## Features
- Monthly income, expense, and savings summary
- Add and delete income/expense transactions
- Category budgets with spending progress
- CSV transaction export
- Local SQLite storage
- Responsive browser interface

## Notes
- This starter version is designed for one local user and does not include online accounts or external API integrations.
- The server binds to `127.0.0.1`, so it is accessible only from the same computer.
- Financial values are stored as SQLite numeric values for this educational starter. For production financial software, use integer minor units or a decimal library and add more extensive tests/security controls.
