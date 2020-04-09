package webcsv

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// SchemaColumn - schema column
type SchemaColumn struct {
	Name      string
	Type      string
	Length    int
	Precision int
	Scale     int
}

// Schema - schema
type Schema struct {
	Version    string
	WithHeader bool
	Delimiter  string
	Columns    []SchemaColumn
	isloaded   bool
}

// ParseSchema - parse WebCSV schema. This is a basic function. We can improve it later.
func ParseSchema(raw string) (schema *Schema, Error error) {
	schema = &Schema{
		isloaded: false,
	}

	// split the header
	parts := strings.Split(raw, `;`)

	// First part: schema properties. These are separated by comma
	prop := strings.Split(parts[0], `,`)
	for _, v := range prop {
		kv := strings.Split(v, `:`)
		switch strings.TrimSpace(kv[0]) {
		case "ver":
			schema.Version = kv[1]
		case "hdr":
			schema.WithHeader, _ = strconv.ParseBool(kv[1])
		case "del":
			schema.Delimiter = kv[1]
			if schema.Delimiter == "" {
				schema.Delimiter = ","
			}
		}
	}

	// Second part: schema columns.
	schp := parts[1]
	schpart := make([]rune, len(schp))
	// check for commas inside parenthesis
	openp := -1
	closep := -1
	commapos := -1
	for i, c := range schp {

		schpart[i] = c

		if c == '(' {
			openp = i
		}

		if c == ',' {
			commapos = i
		}

		if c == ')' {
			closep = i
		}

		if openp != -1 && closep != 1 && commapos != -1 && openp < closep && commapos < closep && commapos > openp {
			schpart[commapos] = ';' // since semicolon was already parsed, we can set the delimiter temporarily to ;
			commapos = -1
			openp = -1
			closep = -1
		}

	}

	sch := strings.Split(string(schpart), `,`)
	schema.Columns = make([]SchemaColumn, len(sch))

	if len(sch) == 0 {
		Error = errors.New(`No schema defined`)
		return
	}

	loadedcols := false

	for i, v := range sch {

		// get name and value
		nv := strings.Split(v, `:`)

		name := strings.TrimSpace(nv[0])

		// A column with one element will be treated as:
		// - Column name
		// - string as default type
		// - maximum length of 4000
		if len(nv) == 1 {
			schema.Columns[i].Name = name
			schema.Columns[i].Type = "string"
			schema.Columns[i].Length = 4000

			loadedcols = true
			continue
		}

		// A column with two elements
		if len(nv) == 2 {
			schema.Columns[i].Length = 0
			schema.Columns[i].Precision = 0
			schema.Columns[i].Scale = 0
			schema.Columns[i].Name = name

			// extract length if there is any
			col := strings.ToLower(strings.TrimSpace(nv[1]))
			schema.Columns[i].Type = col

			// remove parenthesis
			col = strings.ReplaceAll(col, "(", " ")
			col = strings.ReplaceAll(col, ")", "")

			// first instance of space could be the space left by the opening parenthesis '('
			pos := strings.Index(col, ` `)
			lps := ""
			if pos != -1 {
				schema.Columns[i].Type = col[0:pos] // type name

				lps = col[pos+1:] // get length or precision and scale

				// check if the type has comma. A comma represents the precision and scale.
				// If there is no comma (semicolon replaced it earlier), it is just the length
				pos = strings.Index(lps, `;`)
				if pos != -1 {
					schema.Columns[i].Precision, _ = strconv.Atoi(lps[0:pos])
					schema.Columns[i].Scale, _ = strconv.Atoi(lps[pos+1:])
				} else {
					schema.Columns[i].Length, _ = strconv.Atoi(lps)
				}
			}

			loadedcols = true

			continue
		}
	}

	// It makes no sense of the schema does not contain columns
	schema.isloaded = loadedcols

	return
}

// ValidateReturn - data by the schema. This is just a basic validation function.
func (sch *Schema) ValidateReturn(data []byte) (Records [][]string, Error error) {

	// Data will be parsed as CSV
	r := csv.NewReader(bytes.NewReader(data))
	Records, Error = r.ReadAll()

	if Error != nil {
		return
	}

	var (
		err      error
		errorstr string
		sc       *SchemaColumn
	)

	// Validate each line and column
	for i, rec := range Records {

		for cn, cv := range rec {

			sc = &sch.Columns[cn]

			switch sc.Type {
			case "string":
				// Check if the value exceeds the length
				if len(cv) > sc.Length {
					errorstr += fmt.Sprintf("Column %d of line %d exceeds specified column length of %d\n", cn, i+1, sc.Length)
					continue
				}
			case "int":
				// Check if the value can be converted to int
				_, err = strconv.ParseInt(cv, 10, 64)
				if err != nil {
					errorstr += fmt.Sprintf("Column %d of line %d could not be converted to integer. Error: %s\n", cn, i+1, err.Error())
					continue
				}

			case "bool":
				// Check if the value can be converted to boolean
				_, err = strconv.ParseBool(cv)
				if err != nil {
					errorstr += fmt.Sprintf("Column %d of line %d could not be converted to boolean. Error: %s\n", cn, i+1, err.Error())
					continue
				}

			case "date":
				// Check if the value can be converted to date
				_, err = time.Parse("2006-01-02", cv)
				if err != nil {
					errorstr += fmt.Sprintf("Column %d of line %d could not be converted to date. Error: %s\n", cn, i+1, err.Error())
					continue
				}
			case "datetime":
				// Check if the value can be converted to datetime
				_, err = time.Parse(time.RFC3339, cv)
				if err != nil {
					errorstr += fmt.Sprintf("Column %d of line %d could not be converted to datetime. Error: %s\n", cn, i+1, err.Error())
					continue
				}
			case "decimal":

				whl := ""
				dec := ""
				// get decimal point and whole number. This is just the . being parsed.
				pos := strings.Index(cv, `.`)

				// a whole number?
				if pos == -1 {
					whl = cv
					dec = strings.Repeat(`0`, sc.Scale)

					// check if the whole number length plus the schema scale  sums up
					if len(cv) > sc.Precision-sc.Scale {
						errorstr += fmt.Sprintf("Column %d of line %d is not a valid decimal scale as specified by the schema. \n", cn, i+1)
						continue
					}

					cv = whl + `.` + dec
				}

				// decimal?
				if pos != -1 {
					whl = cv[0:pos]
					dec = cv[pos+1:]

					// Trim to scale
					if len(dec) > sc.Scale {
						dec = dec[0:sc.Scale]
					} else {
						dec = dec + strings.Repeat(`0`, sc.Scale-len(dec)) // pad the remaining with zero
					}

					// check decimal if this can be converted to a number
					_, err = strconv.ParseInt(dec, 10, 64)
					if err != nil {
						errorstr += fmt.Sprintf("Column %d of line %d contains an invalid decimal scale as specified by the schema. \n", cn, i+1)
						continue
					}

					// check if the length of the  whole number is valid
					if len(whl) > sc.Precision {
						errorstr += fmt.Sprintf("Column %d of line %d exceeds the whole number length as specified by the schema. \n", cn, i+1)
						continue
					}

					cv = whl + `.` + dec // fix
				}

				// Check if the value can be converted to decimal
				_, err = strconv.ParseFloat(cv, 64)
				if err != nil {
					errorstr += fmt.Sprintf("Column %d of line %d could not be converted to decimal. Error: %s\n", cn, i+1, err.Error())
					continue
				}
			}
		}

		if errorstr != "" {
			break
		}

	}

	if errorstr != "" {
		Error = errors.New(errorstr)
	} else {
		Error = nil
	}

	return
}

// PrintSchema - print schema to string. This is a basic function. We can improve it later.
func (sch *Schema) PrintSchema() string {
	schs := fmt.Sprintf("ver:%s,hdr:%t,del:%s", sch.Version, sch.WithHeader, sch.Delimiter) + "; "

	cma := ""
	for _, c := range sch.Columns {

		if c.Name != "" {
			schs += cma + c.Name + ":"
		}

		schs += c.Type

		if c.Type == "string" {
			schs += fmt.Sprintf("(%d)", c.Length)
		}

		if c.Type == "decimal" {
			schs += fmt.Sprintf("(%d,%d)", c.Precision, c.Scale)
		}

		cma = ","
	}

	return schs
}

// IsValid - checks if the supplied schema is the same
func (sch *Schema) IsValid(ext *Schema) bool {

	if strings.ToLower(sch.Version) != strings.ToLower(ext.Version) {
		return false
	}

	if sch.WithHeader != ext.WithHeader {
		return false
	}

	if sch.Delimiter != ext.Delimiter {
		return false
	}

	cnt := len(sch.Columns)
	if cnt != len(ext.Columns) {
		return false
	}

	for i := 0; i < cnt; i++ {
		if strings.ToLower(sch.Columns[i].Name) != strings.ToLower(ext.Columns[i].Name) {
			return false
		}
		if sch.Columns[i].Length != ext.Columns[i].Length {
			return false
		}
		if sch.Columns[i].Type != ext.Columns[i].Type {
			return false
		}
		if sch.Columns[i].Length != ext.Columns[i].Length {
			return false
		}
		if sch.Columns[i].Precision != ext.Columns[i].Precision {
			return false
		}
		if sch.Columns[i].Scale != ext.Columns[i].Scale {
			return false
		}
	}

	return true
}
