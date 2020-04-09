package main

import (
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	webcsv "webcsv/lib"

	"github.com/gorilla/mux"
)

// Person - person type
type Person struct {
	Alive       bool
	LastName    string
	FirstName   string
	MiddleName  string
	Age         int
	Weight      float64
	Height      float64
	DateBorn    time.Time
	LastUpdated time.Time
}

var apiSchema *webcsv.Schema

var p []Person // data of the API

func main() {

	hp := "8000"

	router := mux.NewRouter()
	router.StrictSlash(true)

	router.PathPrefix("/").Handler(basicCRUDHandler())

	srv := &http.Server{
		Addr:    ":" + hp,
		Handler: router,
	}

	// Define API Schema
	apiSchema = &webcsv.Schema{
		Version:    "1.0",
		WithHeader: false,
		Delimiter:  ",",
	}

	// The order of this SchemaColumn array must be matched
	apiSchema.Columns = []webcsv.SchemaColumn{
		{Name: "LastName", Type: "string", Length: 50},
		{Name: "FirstName", Type: "string", Length: 50},
		{Name: "MiddleName", Type: "string", Length: 50},
		{Name: "Age", Type: "int"},
		{Name: "Height", Type: "decimal", Precision: 13, Scale: 3},
		{Name: "Weight", Type: "decimal", Precision: 13, Scale: 3},
		{Name: "Alive", Type: "bool"},
		{Name: "DateBorn", Type: "date"},
		{Name: "LastUpdated", Type: "datetime"},
	}

	log.Println("Listening at " + hp + "...")
	log.Fatal(srv.ListenAndServe())
}

func basicCRUDHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Handle POST and PUT and validate data
		if r.Method == "POST" || r.Method == "PUT" {

			b := func() []byte {
				if r.Body != nil {
					b, _ := ioutil.ReadAll(r.Body)
					defer r.Body.Close()
					return b
				}
				return []byte{}
			}

			// At this point, the handler could decide whether to validate a schema
			// or directly parse the body of the data into CSV records
			raw := strings.TrimSpace(r.Header.Get("Content-Schema"))
			if strings.ToLower(raw) == "none" && raw == "" {
				w.Write([]byte("ERROR,No valid schema found"))
				return
			}

			sch, err := webcsv.ParseSchema(raw)
			if err != nil {
				w.Write([]byte(fmt.Sprintf("ERROR,Parse: %v", err)))
				return
			}

			// API schema will validate the supplied schema.
			if !apiSchema.IsValid(sch) {
				w.Write([]byte(fmt.Sprintf("ERROR,Invalid schema")))
				return
			}

			recs, err := sch.ValidateReturn(b())
			if err != nil {
				w.Write([]byte("ERROR,Data did not pass the validation against schema"))
				return
			}

			if r.Method == "POST" {
				// Process storing the parsed data.
				// This demo stores the data into a Person struct array for us to retrieve later
				// A json.Unmarshal like deserialization could also be employed here.

				if p == nil {
					p = make([]Person, 0) // an existing list could have been uploaded
				}

				for _, rec := range recs {

					// This should be aligned to the schema order
					pitem := Person{
						LastName:   rec[0],
						FirstName:  rec[1],
						MiddleName: rec[2],
					}
					pitem.Age, _ = strconv.Atoi(rec[3])
					pitem.Height, _ = strconv.ParseFloat(rec[4], 64)
					pitem.Weight, _ = strconv.ParseFloat(rec[5], 64)
					pitem.Alive, _ = strconv.ParseBool(rec[6])
					pitem.DateBorn, _ = time.Parse(time.RFC3339, rec[7])
					pitem.LastUpdated, _ = time.Parse(time.RFC3339, rec[8])

					p = append(p, pitem) // This is not optimal but this is just an example
				}

				w.Write([]byte("OK,Insert"))
			}

			if r.Method == "PUT" {

				// Update could supply a query string to update the specified record
				lname := r.URL.Query().Get("ln")
				fname := r.URL.Query().Get("fn")
				mname := r.URL.Query().Get("mn")

				rec := recs[0] // Updates usuall just have one record

				for i := range p {

					if p[i].LastName == lname && p[i].FirstName == fname && p[i].MiddleName == mname {

						p[i].Age, _ = strconv.Atoi(rec[3])
						p[i].Height, _ = strconv.ParseFloat(rec[4], 64)
						p[i].Weight, _ = strconv.ParseFloat(rec[5], 64)
						p[i].Alive, _ = strconv.ParseBool(rec[6])
						p[i].DateBorn, _ = time.Parse(time.RFC3339, rec[7])
						p[i].LastUpdated, _ = time.Parse(time.RFC3339, rec[8])

						break
					}
				}

				w.Write([]byte("OK,Update"))

			}
		}

		if r.Method == "GET" {

			// Write schema on the header. It can check for request not to send the header to skip sending the header
			w.Header().Set("Content-Schema", apiSchema.PrintSchema())

			// The order of values should be returned as the schema specifies
			cw := csv.NewWriter(w)
			for _, prec := range p {
				rec := []string{
					prec.LastName,
					prec.FirstName,
					prec.MiddleName,
				}
				rec = append(rec, strconv.Itoa(prec.Age))
				rec = append(rec, strconv.FormatFloat(prec.Height, 'f', apiSchema.Columns[4].Scale, 64))
				rec = append(rec, strconv.FormatFloat(prec.Weight, 'f', apiSchema.Columns[5].Scale, 64))
				rec = append(rec, strconv.FormatBool(prec.Alive))
				rec = append(rec, prec.DateBorn.Format(time.RFC3339))
				rec = append(rec, prec.LastUpdated.Format(time.RFC3339))

				cw.Write(rec)
			}

			cw.Flush()
		}

		if r.Method == "DELETE" {

			// Delete could supply a query string to delete the specified record
			lname := r.URL.Query().Get("ln")
			fname := r.URL.Query().Get("fn")
			mname := r.URL.Query().Get("mn")

			pcopy := p
			p = make([]Person, 0)
			for _, prec := range pcopy {
				if prec.LastName == lname && prec.FirstName == fname && prec.MiddleName == mname {
					continue
				}
				p = append(p, prec) // This is not optimal but this is just an example
			}

			w.Write([]byte("OK,Delete"))
		}
	})
}
