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

    features["db"] = Feature{version: 1, category: "db"}
    categories["db"] = []string{"db_init", "db_query", "db_close"} // ,"db_prepared_query"}

    // open a db connection
    slhelp["db_init"] = LibHelp{in: "string", out: "handle",
        action: "Returns a database connection [#i1]handle[#i0], with a default schema of [#i1]string[#i0] based on\n"+
            "inbound environmental variables. (ZA_DB_HOST, ZA_DB_ENGINE, ZA_DB_PORT, ZA_DB_USER, ZA_DB_PASS.)\n"+
            "Only 'mysql' is currently supported as an engine type."}
    stdlib["db_init"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("db_init",args,1,"1","string"); !ok { return nil,err }

        schema := args[0].(string)

        dbhost, ex_host := os.LookupEnv("ZA_DB_HOST")
        dbeng , ex_eng  := os.LookupEnv("ZA_DB_ENGINE")
        dbport, ex_port := os.LookupEnv("ZA_DB_PORT")
        dbuser, ex_user := os.LookupEnv("ZA_DB_USER")
        dbpass, ex_pass := os.LookupEnv("ZA_DB_PASS")

        if !(ex_host || ex_eng || ex_port || ex_user || ex_pass) {
            return nil, errors.New("Error: Missing DB details at startup.")
        }

        // instantiate the db connection:
        dbh, err := sql.Open(dbeng, dbuser+":"+dbpass+"@tcp("+dbhost+":"+dbport+")/"+schema)
        if err != nil {
            return nil, err
        }

        return dbh, err

    }

    // close a db connection
    slhelp["db_close"] = LibHelp{in: "handle", out: "", action: "Closes the database connection."}
    stdlib["db_close"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("db_close",args,1,"1","*sql.DB"); !ok { return nil,err }
        args[0].(*sql.DB).Close()
        return nil, nil
    }


    slhelp["db_query"] = LibHelp{in: "handle,query,field_sep", out: "string", action: `Simple database query. Optional: field separator, default: '|'`}
    stdlib["db_query"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("db_query",args,2,
            "3","*sql.DB","string","string",
            "2","*sql.DB","string"); !ok { return nil,err }

        var q string
        var dbh *sql.DB

        var fsep string = "|"
        if len(args)==3 { fsep = args[2].(string) }

        dbh = args[0].(*sql.DB)
        q = args[1].(string)

        if err := dbh.Ping(); err != nil {
            log.Fatal(err)
        }

        rows, err := dbh.Query(q)
        if err != nil {
            log.Fatal(err)
        }
        defer rows.Close()

        l := make([]string, 50, 200)
        rc := 0

        cols, err := rows.ColumnTypes()
        if err != nil {
            return "", err
        }

        vals := make([]any, len(cols))
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

        return l[:rc], err
    }

}

