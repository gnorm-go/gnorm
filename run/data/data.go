// Package data supplies the data for gnorm templates
package data

import (
	"fmt"
)

// This is all the data passed to templates.

// DBData is all the data about a database that we know.
type DBData struct {
	Schemas       []*Schema
	SchemasByName map[string]*Schema `yaml:"-" json:"-"` // dbname to schema
}

// SchemaData is the data passed to schema templates.
type SchemaData struct {
	Schema *Schema
	DB     *DBData
	Config ConfigData
	Params map[string]interface{}
}

// TableData is the data passed to table templates.
type TableData struct {
	Table  *Table
	DB     *DBData
	Config ConfigData
	Params map[string]interface{}
}

// EnumData is the data passed to enum templates.
type EnumData struct {
	Enum   *Enum
	DB     *DBData
	Config ConfigData
	Params map[string]interface{}
}

// Schema is the data about a DB schema.
type Schema struct {
	Name         string            // the converted name of the schema
	DBName       string            // the original name of the schema in the DB
	Tables       Tables            // the list of tables in this schema
	Enums        Enums             // the list of enums in this schema
	TablesByName map[string]*Table `yaml:"-" json:"-"` // dbnames to tables
}

// Table is the data about a DB Table.
type Table struct {
	Name                               string                      // the converted name of the table
	DBName                             string                      // the original name of the table in the DB
	Schema                             *Schema                     `yaml:"-" json:"-"` // the schema this table is in
	Columns                            Columns                     // Database columns
	ColumnsByName                      map[string]*Column          `yaml:"-" json:"-"` // dbname to column
	ForeignColumnsByForeignKey         map[string][]*ForeignColumn // foreign key columns
	PrimaryKeys                        Columns                     // Primary Key Columns
	ForeignKeys                        []string                    // database names of this tables foreign keys
	ForeignKeyReferences               []string                    // database names of foreign keys referencing this table
	ForeignTablesByForeignKey          map[string]*ForeignTable    // tables referenced by foreign keys
	ForeignTablesByForeignKeyReference map[string]*ForeignTable    // all tables referencing this table
}

// Returns true if Table has one or more primary keys
func (t *Table) HasPrimaryKey() bool {
	return len(t.PrimaryKeys) > 0
}

// Returns true if Table has one or more foreign keys
func (t *Table) HasForeignKeys() bool {
	return len(t.ForeignColumnsByForeignKey) > 0
}

// Returns true if one or more foreign keys reference Table
func (t *Table) HasForeignKeyReferences() bool {
	return len(t.ForeignKeyReferences) > 0
}

// Column is the data about a DB column of a table.
type Column struct {
	Table                               *Table                    `yaml:"-" json:"-"` // the table this column is in
	Name                                string                    // the converted name of the column
	DBName                              string                    // the original name of the column in the DB
	Type                                string                    // the converted name of the type
	DBType                              string                    // the original type of the column in the DB
	IsArray                             bool                      // true if the column type is an array
	Length                              int                       // non-zero if the type has a length (e.g. varchar[16])
	UserDefined                         bool                      // true if the type is user-defined
	Nullable                            bool                      // true if the column is not NON NULL
	HasDefault                          bool                      // true if the column has a default
	IsPrimaryKey                        bool                      // true if the column is a primary key
	IsForeignKey                        bool                      // true if the column is a foreign key
	IsForeignKeyReference               bool                      // true if the column is referenced by a foreign key
	ForeignColumn                       *ForeignColumn            // foreign key database definition
	ForeignKeyReferences                []string                  // all database names of foreign keys referencing this column
	ForeignColumnsByForeignKeyReference map[string]*ForeignColumn // all columns referring to this column
	Orig                                interface{}               `yaml:"-" json:"-"` // the raw database column data
}

type ForeignTable struct {
	Name             string
	TableName        string
	ForeignTableName string
	Table            *Table `yaml:"-" json:"-"`
	ForeignTable     *Table `yaml:"-" json:"-"`
}

// Foreign Column contains the definition of a database foreign key
type ForeignColumn struct {
	Name                     string // the original name of the foreign key constraint in the db
	ColumnName               string
	ForeignColumnName        string
	UniqueConstraintPosition int     // the position of the unique constraint in the db
	Column                   *Column `yaml:"-" json:"-"` // the foreign key column
	ForeignColumn            *Column `yaml:"-" json:"-"` // the referenced column

}

// Enum represents a type that has a set of allowed values.
type Enum struct {
	Name   string       // the converted name of the enum
	DBName string       // the original name of the enum in the DB
	Schema *Schema      `yaml:"-" json:"-"` // the schema the enum is in
	Table  *Table       `yaml:"-" json:"-"` // (mysql) the table this enum is part of
	Values []*EnumValue // the list of possible values for this enum
}

// EnumValue is one of the named values for an enum.
type EnumValue struct {
	Name   string // the converted label of the enum
	DBName string // the original label of the enum in the DB
	Value  int    // the value for this enum value (order)
}

// ConfigData holds the portion of the config that will be available to
// templates.  NOte that Params are added to the data at a higher level.
type ConfigData struct {
	// ConnStr is the connection string for the database.  Environment variables
	// in $FOO form will be expanded.
	ConnStr string

	// Schemas holds the names of schemas to generate code for.
	Schemas []string

	// IncludeTables is a map of schema names to table names. It is whitelist of
	// tables to generate data for. Tables not in this list will not be included
	// in data generated by gnorm. You cannot set IncludeTables if ExcludeTables
	// is set.
	IncludeTables map[string][]string

	// ExcludeTables is a map of schema names to table names.  It is a blacklist
	// of tables to ignore while generating data. All tables in a schema that
	// are not in this list will be used for generation. You cannot set
	// ExcludeTables if IncludeTables is set.
	ExcludeTables map[string][]string

	// PostRun is a command with arguments that is run after each file is
	// generated by GNORM.  It is generally used to reformat the file, but it
	// can be for any use. Environment variables will be expanded, and the
	// special $GNORMFILE environment variable may be used, which will expand to
	// the name of the file that was just generated.
	PostRun []string

	// TypeMap is a mapping of database type names to replacement type names
	// (generally types from your language for deserialization).  Types not in
	// this list will remain in their database form.  In the data sent to your
	// template, this is the Column.Type, and the original type is in
	// Column.OrigType.  Note that because of the way tables in TOML work,
	// TypeMap and NullableTypeMap must be at the end of your configuration
	// file.
	TypeMap map[string]string

	// NullableTypeMap is a mapping of database type names to replacement type
	// names (generally types from your language for deserialization)
	// specifically for database columns that are nullable.  Types not in this
	// list will remain in their database form.  In the data sent to your
	// template, this is the Column.Type, and the original type is in
	// Column.OrigType.   Note that because of the way tables in TOML work,
	// TypeMap and NullableTypeMap must be at the end of your configuration
	// file.
	NullableTypeMap map[string]string
}

// Strings is a named type of []string to allow us to put methods on it.
type Strings []string

// Sprintf calls fmt.Sprintf(format, str) for every string in this value and
// returns the results as a new Strings.
func (s Strings) Sprintf(format string) Strings {
	ret := make(Strings, len(s))
	for x := range s {
		ret[x] = fmt.Sprintf(format, s[x])
	}
	return ret
}

// Columns represents the ordered list of columns in a table.
type Columns []*Column

// Names returns the ordered list of column Names in this table.
func (c Columns) Names() Strings {
	names := make(Strings, len(c))
	for x := range c {
		names[x] = c[x].Name
	}
	return names
}

// DBNames returns the ordered list of column DBNames in this table.
func (c Columns) DBNames() Strings {
	names := make(Strings, len(c))
	for x := range c {
		names[x] = c[x].DBName
	}
	return names
}

// Tables is a list of tables in this schema.
type Tables []*Table

// Names returns a list of table Names in this schema.
func (t Tables) Names() Strings {
	names := make([]string, len(t))
	for x := range t {
		names[x] = t[x].Name
	}
	return names
}

// DBNames returns a list of table DBNames in this schema.
func (t Tables) DBNames() Strings {
	names := make([]string, len(t))
	for x := range t {
		names[x] = t[x].DBName
	}
	return names
}

// Enums represents all the enums in a schema.
type Enums []*Enum

// Names returns the list of enum Names in this schema.
func (c Enums) Names() Strings {
	names := make(Strings, len(c))
	for x := range c {
		names[x] = c[x].Name
	}
	return names
}

// DBNames returns the list of enum DBNames in this schema.
func (c Enums) DBNames() Strings {
	names := make(Strings, len(c))
	for x := range c {
		names[x] = c[x].DBName
	}
	return names
}
