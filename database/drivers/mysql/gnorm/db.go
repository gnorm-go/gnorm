package gnorm // import "gnorm.org/gnorm/database/drivers/mysql/gnorm"

// Note that this file is *NOT* generated. :)

import (
	"database/sql"
	"fmt"
	"strings"
)

// DB is the common interface for database operations.
//
// This should work with database/sql.DB and database/sql.Tx.
type DB interface {
	Exec(string, ...interface{}) (sql.Result, error)
	Query(string, ...interface{}) (*sql.Rows, error)
	QueryRow(string, ...interface{}) *sql.Row
}

// WhereClause has a String function should return a properly formatted where
// clause (not including the WHERE) for positional arguments starting at idx.
type WhereClause interface {
	String() string
	Values() []interface{}
}

type comparison string

const (
	compEqual   comparison = " = "
	compGreater comparison = " > "
	compLess    comparison = " < "
	compGTE     comparison = " >= "
	compLTE     comparison = " <= "
	compNE      comparison = " <> "
)

type inClause struct {
	field  string
	values []interface{}
}

func (in inClause) String() string {
	ret := in.field + " in ("
	for x := range in.values {
		if x != 0 {
			ret += ", "
		}
		ret += "?"
	}
	ret += ")"
	return ret
}

func (in inClause) Values() []interface{} {
	return in.values
}

type whereClause struct {
	field string
	comp  comparison
	value interface{}
}

func (w whereClause) String() string {
	ret := w.field + string(w.comp) + "?"
	return ret
}

func (w whereClause) Values() []interface{} {
	return []interface{}{w.value}
}

// AndClause returns a WhereClause that serializes to the AND
// of all the given where clauses.
func AndClause(wheres ...WhereClause) WhereClause {
	return andClause(wheres)
}

type andClause []WhereClause

func (a andClause) String() string {
	var wheres []string
	for _, clause := range a {
		wheres = append(wheres, clause.String())
	}
	return fmt.Sprintf("(%s)", strings.Join(wheres, " AND "))
}

func (a andClause) Values() []interface{} {
	var vals []interface{}
	for x := 0; x < len(a); x++ {
		vals = append(vals, a[x].Values()...)
	}
	return vals
}

// OrClause returns a WhereClause that serializes to the OR
// of all the given where clauses.
func OrClause(wheres ...WhereClause) WhereClause {
	return orClause(wheres)
}

type orClause []WhereClause

func (o orClause) String() string {
	var wheres []string
	for _, clause := range o {
		wheres = append(wheres, clause.String())
	}
	return fmt.Sprintf("(%s)", strings.Join(wheres, " OR "))
}

func (o orClause) Values() []interface{} {
	var vals []interface{}
	for x := 0; x < len(o); x++ {
		vals = append(vals, o[x].Values()...)
	}
	return vals
}
