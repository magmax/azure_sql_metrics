/*
Copyright (c) 2015 Mediatech Solutions
    
Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/

package main

import _ "github.com/denisenkom/go-mssqldb"
import (
    "os"
    "time"
    "encoding/json"
    "log"
    "fmt"
    "flag"
    "database/sql"
    "strings"
)

var hostname, _ = os.Hostname()

var config_file = flag.String("config", "azure_sql_metrics.json", "File with configuration parameters")
var scheme      = flag.String("scheme", fmt.Sprintf("%s.database", hostname), "Graphite path")

const QUERY_ERRORS = "select count(1) from sys.messages"
const QUERY_DEADLOCKS = "select count(1) from sys.event_log where event_type = 'deadlock'"
const QUERY_THROTTLING = "select count(1) from sys.event_log where event_type like 'throttling%'"
const QUERY_RESOURCES = "select count(1) from sys.resource_stats"
const QUERY_FIREWALL = "select count(*) from sys.firewall_rules where end_ip_address != '0.0.0.0'"
const QUERY_DTU = `
select top 10 database_name, storage_in_megabytes, avg_cpu_percent, avg_data_io_percent, avg_log_write_percent, 
 (select max(v) from (values (avg_cpu_percent), (avg_log_write_percent), (avg_data_io_percent)) as value(v)) as avg_dtu_percent
from (
  select row_number() over (partition by database_name order by end_time desc) as rownumber,  *
  from sys.resource_stats
) as f where f.rownumber = 1
`

var Config struct {
    Server string
    Port string  
    User string 
    Password string
    Schema string
    IgnoreIps []interface{}
}

func read_configuration(filename *string) (string, []interface{}) {
    f, err := os.Open(*filename); if err != nil { log.Fatal("File error:", err.Error()) }
    parser := json.NewDecoder(f)
    err = parser.Decode(&Config); if err != nil { log.Fatal("Decode failed:", err.Error()) }
    
    server     := Config.Server;     if server == ""   { server = "localhost" }
    port       := Config.Port;       if port == ""     { port = "1433" }
    user       := Config.User;       if user == ""     { user = "sa" }      
    password   := Config.Password;   if password == "" { password = "" }
    schema     := Config.Schema;     if schema == ""   { schema = "foo" }
    ignore_ips := Config.IgnoreIps;  if schema == ""   { ignore_ips = nil}
    
    return fmt.Sprintf("server=%s;user id=%s;password=%s;port=%s;databaseName=%s", server, user, password, port, schema), ignore_ips
}

// Using QueryRow
func ask(conn *sql.DB, query string) string {
    stmt, err := conn.Prepare(query)
	if err != nil {
		log.Fatal("Prepare failed:", err.Error())
	}
	defer stmt.Close()
    
    row := stmt.QueryRow()
        
    var value string
	err = row.Scan(&value)
    if err != nil {
		log.Fatal("Scan failed:", err.Error())
	}
    
    return value
}

// Using Query
func ask_many(conn *sql.DB, query string) string {
    stmt, err := conn.Prepare(query)
	if err != nil {
		log.Fatal("Prepare failed:", err.Error())
	}
	defer stmt.Close()
    
    rows, err := stmt.Query()
    if err != nil {
		log.Fatal("Query failed:", err.Error())
	}
        
    var value string
    for rows.Next(){
        err = rows.Scan(&value)
        if err != nil {
            log.Fatal("Scan failed:", err.Error())
        }
        return value
    }
    return ""
}

func firewall_rules(conn *sql.DB, ignored_ips []interface{}) string {
    query := "select count(*) from sys.firewall_rules "
    if ignored_ips != nil {
        query += "where end_ip_address not in (?" + strings.Repeat(",?", len(ignored_ips)-1) + ")"
    }
    stmt, err := conn.Prepare(query)
	if err != nil {
		log.Fatal("Prepare failed:", err.Error())
	}
	defer stmt.Close()
    
    rows, err := stmt.Query(ignored_ips...)
    if err != nil {
		log.Fatal("Query failed:", err.Error())
	}
        
    var value string
    for rows.Next(){
        err = rows.Scan(&value)
        if err != nil {
            log.Fatal("Scan failed:", err.Error())
        }
        return value
    }
    return "0"
}

func dtu(conn *sql.DB, scheme *string) {
    stmt, err := conn.Prepare(QUERY_DTU)
	if err != nil {
		log.Fatal("Prepare failed:", err.Error())
	}
	defer stmt.Close()
    
    rows, err := stmt.Query()
    if err != nil {
		log.Fatal("Query failed:", err.Error())
	}
        
    var database string
    var storage_in_megabytes string
    var avg_cpu_percent string
    var avg_data_io_percent string
    var avg_log_write_percent string
    var avg_dtu_percent string
    for rows.Next(){
        err = rows.Scan(&database, &storage_in_megabytes, &avg_cpu_percent, &avg_data_io_percent, &avg_log_write_percent, &avg_dtu_percent)
        if err != nil {
            log.Fatal("Scan failed:", err.Error())
        }
        fmt.Printf("%s.sql.%s.size %s %d\n", *scheme, database, storage_in_megabytes, time.Now().Unix())
        fmt.Printf("%s.sql.%s.cpu.percent %s %d\n", *scheme, database, avg_cpu_percent, time.Now().Unix())
        fmt.Printf("%s.sql.%s.io.percent %s %d\n", *scheme, database, avg_data_io_percent, time.Now().Unix())
        fmt.Printf("%s.sql.%s.log_write.percent %s %d\n", *scheme, database, avg_log_write_percent, time.Now().Unix())
        fmt.Printf("%s.sql.%s.dtu.percent %s %d\n", *scheme, database, avg_dtu_percent, time.Now().Unix())
    }
}

func main() {
    flag.Parse()

    connString, ignored_ips := read_configuration(config_file)
    fmt.Println(connString)

    conn, err := sql.Open("mssql", connString)
    if err != nil {
		log.Fatal("Open connection failed:", err.Error())
	}
	defer conn.Close()

    fmt.Printf("%s.sql.firewall.rules %s %d\n", *scheme, firewall_rules(conn, ignored_ips), time.Now().Unix())
    fmt.Printf("%s.sql.errors %s %d\n", *scheme, ask(conn, QUERY_ERRORS), time.Now().Unix())
    /* These queries are too slow!!
    fmt.Printf("%s.sql.deadlocks %s %d\n", *scheme, ask(conn, QUERY_DEADLOCKS), time.Now().Unix())
    fmt.Printf("%s.sql.throttling %s %d\n", *scheme, ask(conn, QUERY_THROTTLING), time.Now().Unix())
    */
    
    dtu(conn, scheme)    
}