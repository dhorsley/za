//+build !test

package main

import (
	"database/sql"
	"errors"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"os"
)

func buildDbLib() {

	// os level

	features["db"] = Feature{version: 1, category: "net"}
	categories["db"] = []string{"db_init", "db_query", "db_close"} // ,"db_prepared_query"}

	// @todo: fix this: engine not found when in subdir of za build!

	// open a db connection
	slhelp["db_init"] = LibHelp{in: "schema", out: "handle", action: "Returns a database connection [#i1]handle[#i0], based on inbound environmental variables."}
	stdlib["db_init"] = func(args ...interface{}) (ret interface{}, err error) {

		if len(args) != 1 {
			return nil, errors.New("Bad args (count) in db_init()")
		}

		var schema string

		switch args[0].(type) {
		case string:
			schema = args[0].(string)
		default:
			return nil, errors.New("Supplied argument was not a schema name.")
		}
		// pf("schema->%s\n", schema)

		dbhost, ex_host := os.LookupEnv("ZA_DB_HOST")
		dbengine, ex_eng := os.LookupEnv("ZA_DB_ENGINE")
		dbport, ex_port := os.LookupEnv("ZA_DB_PORT")
		dbuser, ex_user := os.LookupEnv("ZA_DB_USER")
		dbpass, ex_pass := os.LookupEnv("ZA_DB_PASS")

		if !(ex_host || ex_eng || ex_port || ex_user || ex_pass) {
			return nil, errors.New("Error: Missing DB details at startup.")
		}

		// instantiate the db connection:
		dbh, err := sql.Open(dbengine, dbuser+":"+dbpass+"@tcp("+dbhost+":"+dbport+")/"+schema)
		if err != nil {
			return nil, err
		}

		return dbh, err

	}

	// close a db connection
	slhelp["db_close"] = LibHelp{in: "handle", out: "", action: "Closes the database connection."}
	stdlib["db_close"] = func(args ...interface{}) (ret interface{}, err error) {

		if len(args) != 1 {
			return nil, errors.New("Invalid argument count to db_close().")
		}

		switch args[0].(type) {
		case *sql.DB:
			args[0].(*sql.DB).Close()
		default:
			return nil, errors.New("Supplied argument was not a database handle.")
		}

		return nil, nil
	}

	slhelp["db_query"] = LibHelp{in: "handle,query,field_sep", out: "string", action: `Simple database query. Optional: field separator, default: "|"`}
	stdlib["db_query"] = func(args ...interface{}) (ret interface{}, err error) {

		var q string
		var dbh *sql.DB
		var fsep string = "|"

		switch len(args) {
		case 2:
			break
		case 3:
			if sf("%T", args[2]) == "string" {
				fsep = args[2].(string)
			}
		default:
			return nil, errors.New("Invalid argument count to db_query().")
		}

		switch args[0].(type) {
		case *sql.DB:
			dbh = args[0].(*sql.DB)
		default:
			return nil, errors.New("Supplied argument was not a database handle.")
		}

		switch args[1].(type) {
		case string:
			q = args[1].(string)
		default:
			return nil, errors.New("Supplied argument was not a query string.")
		}

		if err := dbh.Ping(); err != nil {
			log.Fatal(err)
		}

		rows, err := dbh.Query(q) // @todo: add prepared statement args later
		if err != nil {
			log.Fatal(err)
		}
		defer rows.Close()

		l := make([]string, 50, 200)
		rc := 0

		for rows.Next() {

			cols, err := rows.ColumnTypes()
			if err != nil {
				return "", err
			}

			vals := make([]interface{}, len(cols))
			for i, _ := range cols {
				vals[i] = new(sql.RawBytes)
			}

			for rows.Next() {
				err = rows.Scan(vals...)
				l = append(l, "")
				for v := range vals {
					l[rc] += sf("%s"+fsep, vals[v])[1:]
				}
				l[rc] = l[rc][0 : len(l[rc])-1]
				rc++
			}
		}
		return l[:rc], err
	}

}
