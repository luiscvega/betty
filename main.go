package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/text/message"
)

func main() {
	notifyFlag := flag.Bool("notify", false, "notify on slack")
	flag.Parse()

	db, err := sql.Open("sqlite3", flag.Args()[0])
	if err != nil {
		panic(err)
	}
	defer db.Close()

	accounts, err := getAccounts(db)
	if err != nil {
		panic(err)
	}

	proxyUrl, err := url.Parse(os.Getenv("PROXY_URL"))
	if err != nil {
		panic(err)
	}

	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyUrl)}}

	for _, account := range accounts {
		// Step 1: Get access token
		data := url.Values{
			"grant_type": []string{"password"},
			"client_id":  []string{account.clientId},
			"username":   []string{account.username},
			"password":   []string{account.password},
			"scope":      []string{"account_inquiry"},
		}

		req, err := http.NewRequest("POST", "https://api.unionbankph.com/ubp/external/partners/v1/oauth2/token", strings.NewReader(data.Encode()))
		if err != nil {
			panic(err)
		}

		req.Header.Add("Accept", "application/json")
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		resp, err := client.Do(req)
		if err != nil {
			panic(err)
		}

		inload := struct {
			AccessToken string `json:"access_token"`
		}{}

		if err = json.NewDecoder(resp.Body).Decode(&inload); err != nil {
			panic(err)
		}

		// Step 2: Get transactions
		req, err = http.NewRequest("GET", "https://api.unionbankph.com/ubp/external/portal/accounts/v1/transactions/paginate?limit=500", nil)
		if err != nil {
			panic(err)
		}

		req.Header.Add("Accept", "application/json")
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("x-ibm-client-id", account.clientId)
		req.Header.Add("x-ibm-client-secret", account.clientSecret)
		req.Header.Add("x-partner-id", account.partnerId)
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", inload.AccessToken))

		resp, err = client.Do(req)
		if err != nil {
			panic(err)
		}

		if resp.StatusCode != http.StatusOK {
			bs, _ := ioutil.ReadAll(resp.Body)
			fmt.Println(string(bs))
			panic("not ok!")
		}

		transactionsInload := struct {
			Records []record
		}{}

		if err = json.NewDecoder(resp.Body).Decode(&transactionsInload); err != nil {
			panic(err)
		}

		// Step 3: Save transactions
		for _, record := range transactionsInload.Records {
			var exists bool

			if err := db.QueryRow("SELECT COUNT(*) > 0 FROM records WHERE tran_id = ? AND tran_type = ?", record.TranId, record.TranType).Scan(&exists); err != nil {
				panic(err)
			}

			if exists {
				continue
			}

			if _, err := db.Exec("INSERT INTO records (account_id, tran_id, tran_type, amount, currency, tran_date, remarks2, remarks, balance_currency, posted_date, tran_description) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", account.id, record.TranId, record.TranType, record.Amount, record.Currency, record.TranDate, record.Remarks2, record.Remarks, record.BalanceCurrency, record.PostedDate, record.TranDescription); err != nil {
				panic(err)
			}

			// Step 4: Notify each new transaction
			if notifyFlag != nil && !*notifyFlag {
				continue
			}

			var tranType string
			var heart string

			if record.TranType == "C" {
				heart = ":green_heart:"
				tranType = "Credit"
			} else {
				heart = ":heart:"
				tranType = "Debit"
			}

			postedDate, err := time.Parse("2006-01-02T15:04:05.000", record.PostedDate)
			if err != nil {
				panic(err)
			}

			amount, err := strconv.ParseFloat(record.Amount, 64)
			if err != nil {
				panic(err)
			}

			var b bytes.Buffer

			outload := map[string]string{"text": strings.Join(strings.Fields(fmt.Sprintf("%s New Unionbank %s %s %s: %s %s %s %s %s on %s", heart, account.name, tranType, record.TranId,
				message.NewPrinter(message.MatchLanguage("en")).Sprintf("%.2f", amount),
				record.Currency, record.TranDescription, record.Remarks, record.Remarks2, postedDate.Format("1/2 15:04"))), " ")}

			if err := json.NewEncoder(&b).Encode(outload); err != nil {
				panic(err)
			}

			if _, err := http.Post(os.Getenv("SLACK_HOOK_URL"), "application/json", &b); err != nil {
				panic(err)
			}
		}
	}
}

func getAccounts(db *sql.DB) ([]account, error) {
	accounts := []account{}

	rows, err := db.Query("SELECT id, number, name, client_id, client_secret, username, password, partner_id FROM accounts")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		account := account{}

		if err := rows.Scan(
			&account.id,
			&account.number,
			&account.name,
			&account.clientId,
			&account.clientSecret,
			&account.username,
			&account.password,
			&account.partnerId,
		); err != nil {
			return nil, err
		}

		accounts = append(accounts, account)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return accounts, nil
}

type account struct {
	id           string
	number       string
	name         string
	clientId     string
	clientSecret string
	username     string
	password     string
	partnerId    string
}

type record struct {
	RecordNumber    string `json:"recordNumber"`
	TranId          string `json:"tranId"`
	TranType        string `json:"tranType"`
	Amount          string `json:"amount"`
	Currency        string `json:"currency"`
	TranDate        string `json:"tranDate"`
	Remarks2        string `json:"remarks2"`
	Remarks         string `json:"remarks"`
	BalanceCurrency string `json:"balanceCurrency"`
	PostedDate      string `json:"postedDate"`
	TranDescription string `json:"tranDescription"`
}
