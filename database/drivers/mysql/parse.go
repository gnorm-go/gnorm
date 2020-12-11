package mysql // import "gnorm.org/gnorm/database/drivers/mysql"

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	// mysql driver
	_ "github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"

	"gnorm.org/gnorm/database"
	"gnorm.org/gnorm/database/drivers/mysql/gnorm/columns"
	"gnorm.org/gnorm/database/drivers/mysql/gnorm/statistics"
	"gnorm.org/gnorm/database/drivers/mysql/gnorm/tables"
)

// MySQL implements drivers.Driver interface for MySQL database.
type MySQL struct{}

// Parse reads the mysql schemas for the given schemas and converts them into
// database.Info structs.
func (MySQL) Parse(log *log.Logger, conn string, schemaNames []string, filterTables func(schema, table string) bool) (*database.Info, error) {
	return parse(log, conn, schemaNames, filterTables)
}

func parse(log *log.Logger, conn string, schemaNames []string, filterTables func(schema, table string) bool) (*database.Info, error) {
	log.Println("connecting to mysql with DSN", conn)
	db, err := sql.Open("mysql", conn)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	log.Println("querying table schemas for", schemaNames)
	tables, err := tables.Query(db, tables.TableSchemaCol.In(schemaNames))
	if err != nil {
		return nil, err
	}

	schemas := make(map[string][]*database.Table, len(schemaNames))

	for _, t := range tables {
		if !filterTables(t.TableSchema, t.TableName) {
			continue
		}
		schemas[t.TableSchema] = append(schemas[t.TableSchema], &database.Table{
			Name:    t.TableName,
			Type:    t.TableType,
			Comment: t.TableComment,
			IsView:  t.TableType == "VIEW",
		})
	}

	columns, err := columns.Query(db, columns.TableSchemaCol.In(schemaNames))
	if err != nil {
		return nil, err
	}

	enums := map[string][]*database.Enum{}

	for _, c := range columns {
		if !filterTables(c.TableSchema, c.TableName) {
			continue
		}
		tables, ok := schemas[c.TableSchema]
		if !ok {
			log.Printf("Should be impossible: column %q references unknown schema %q", c.ColumnName, c.TableSchema)
			continue
		}

		var table *database.Table
		for _, t := range tables {
			if t.Name == c.TableName {
				table = t
				break
			}
		}
		if table == nil {
			log.Printf("Should be impossible: column %q references unknown table %q in schema %q", c.ColumnName, c.TableName, c.TableSchema)
			continue
		}

		col, enum, terr := toDBColumn(c, log)
		if terr != nil {
			return nil, terr
		}

		table.Columns = append(table.Columns, col)
		if enum != nil {
			enum.Table = c.TableName
			enums[c.TableSchema] = append(enums[c.TableSchema], enum)
		}
	}

	indexes := make(map[string]map[string][]*database.Index)

	statistics, err := statistics.Query(db, statistics.TableSchemaCol.In(schemaNames))
	if err != nil {
		return nil, err
	}

	for _, s := range statistics {
		if !filterTables(s.TableSchema, s.TableName) {
			continue
		}

		tables, ok := schemas[s.TableSchema]
		if !ok {
			log.Printf("Should be impossible: index %q references unknown schema %q", s.IndexName, s.TableSchema)
			continue
		}

		var table *database.Table
		for _, t := range tables {
			if t.Name == s.TableName {
				table = t
				break
			}
		}
		if table == nil {
			log.Printf("Should be impossible: index %q references unknown table %q", s.IndexName, s.TableName)
			continue
		}

		var column *database.Column
		for _, c := range table.Columns {
			if c.Name == s.ColumnName {
				column = c
				break
			}
		}
		if column == nil {
			log.Printf("Should be impossible: index %q references unknown column %q", s.IndexName, s.ColumnName)
			continue
		}

		schemaIndex, ok := indexes[s.TableSchema]
		if !ok {
			schemaIndex = make(map[string][]*database.Index)
			indexes[s.TableSchema] = schemaIndex
		}

		var index *database.Index
		for _, i := range schemaIndex[s.TableName] {
			if i.Name == s.IndexName {
				index = i
				break
			}
		}
		if index == nil {
			index = &database.Index{Name: s.IndexName, IsUnique: s.NonUnique == 0}
			schemaIndex[s.TableName] = append(schemaIndex[s.TableName], index)
		}

		index.Columns = append(index.Columns, column)
	}

	foreignKeys, err := queryForeignKeys(log, db, schemaNames)
	if err != nil {
		return nil, err
	}
	for _, fk := range foreignKeys {
		if !filterTables(fk.SchemaName, fk.TableName) {
			log.Printf("skipping constraint %q because it is for filtered-out table %v.%v", fk.Name, fk.SchemaName, fk.TableName)
			continue
		}

		tables, ok := schemas[fk.SchemaName]
		if !ok {
			log.Printf("Should be impossible: constraint %q references unknown schema %q", fk.Name, fk.SchemaName)
			continue
		}

		var table *database.Table
		for _, t := range tables {
			if t.Name == fk.TableName {
				table = t
				break
			}
		}
		if table == nil {
			log.Printf("Should be impossible: constraint %q references unknown table %q in schema %q", fk.Name, fk.TableName, fk.SchemaName)
			continue
		}

		for _, col := range table.Columns {
			if fk.ColumnName != col.Name {
				continue
			}
			col.IsForeignKey = true
			col.ForeignKey = fk
		}
	}

	res := &database.Info{Schemas: make([]*database.Schema, 0, len(schemas))}
	for _, schema := range schemaNames {
		tables := schemas[schema]
		s := &database.Schema{
			Name:   schema,
			Tables: tables,
			Enums:  enums[schema],
		}

		dbtables := make(map[string]*database.Table, len(tables))
		for _, t := range tables {
			dbtables[t.Name] = t
		}
		for tname, index := range indexes[schema] {
			dbtables[tname].Indexes = index
		}

		res.Schemas = append(res.Schemas, s)
	}

	return res, nil
}

func toDBColumn(c *columns.Row, log *log.Logger) (*database.Column, *database.Enum, error) {
	col := &database.Column{
		Name:         c.ColumnName,
		Nullable:     c.IsNullable == "YES",
		HasDefault:   c.ColumnDefault.String != "",
		Type:         c.DataType,
		Comment:      c.ColumnComment,
		Ordinal:      c.OrdinalPosition,
		Orig:         *c,
		IsPrimaryKey: strings.Contains(c.ColumnKey, "PRI"),
	}

	// MySQL always specifies length even if it's not a part of the type. We
	// only really care if it's a part of the type, so check if the size is part
	// of the column_type.
	if strings.HasSuffix(c.ColumnType, fmt.Sprintf("(%v)", c.CharacterMaximumLength.Int64)) {
		col.Length = int(c.CharacterMaximumLength.Int64)
	}

	// MySQL ColumnType exposes sign information for numeric types. We want to
	// reflect that the type.
	if strings.Contains(c.ColumnType, "unsigned") {
		col.Type += " unsigned"
	}

	if col.Type != "enum" {
		return col, nil, nil
	}
	// in mysql, enums are specific to a column in a table, so all their data is
	// contained in the column they're used by.

	// column type should be enum('foo', 'bar')
	if len(c.ColumnType) < 5 {
		return nil, nil, errors.New("unexpected column type: " + c.ColumnType)
	}

	// we'll call the enum the same as the column name.
	// the function above will set the table name etc
	enum := &database.Enum{
		Name: col.Name,
	}
	// strip off the enum and parens
	s := c.ColumnType[5 : len(c.ColumnType)-1]
	vals := strings.Split(s, ",")
	enum.Values = make([]*database.EnumValue, len(vals))
	for x := range vals {
		enum.Values[x] = &database.EnumValue{
			// strip off the quotes
			Name: vals[x][1 : len(vals[x])-1],
			// enum values start at 1 in mysql
			Value: x + 1,
		}
	}

	return col, enum, nil
}

func queryForeignKeys(log *log.Logger, db *sql.DB, schemas []string) ([]*database.ForeignKey, error) {
	// TODO: make this work with Gnorm generated types
	const q = `SELECT lkc.TABLE_SCHEMA, lkc.TABLE_NAME, lkc.COLUMN_NAME, lkc.CONSTRAINT_NAME, lkc.POSITION_IN_UNIQUE_CONSTRAINT, lkc.REFERENCED_TABLE_NAME, lkc.REFERENCED_COLUMN_NAME
	  FROM information_schema.REFERENTIAL_CONSTRAINTS as rc
  		LEFT JOIN information_schema.KEY_COLUMN_USAGE as lkc
          ON lkc.CONSTRAINT_SCHEMA = rc.CONSTRAINT_SCHEMA
            AND lkc.CONSTRAINT_NAME = rc.CONSTRAINT_NAME
	  WHERE rc.CONSTRAINT_SCHEMA IN (%s)`
	spots := make([]string, len(schemas))
	vals := make([]interface{}, len(schemas))
	for x := range schemas {
		spots[x] = "?"
		vals[x] = schemas[x]
	}
	query := fmt.Sprintf(q, strings.Join(spots, ", "))
	rows, err := db.Query(query, vals...)
	if err != nil {
		return nil, errors.WithMessage(err, "error querying foreign keys")
	}
	defer rows.Close()
	var ret []*database.ForeignKey

	for rows.Next() {
		fk := &database.ForeignKey{}
		if err := rows.Scan(&fk.SchemaName, &fk.TableName, &fk.ColumnName, &fk.Name, &fk.UniqueConstraintPosition, &fk.ForeignTableName, &fk.ForeignColumnName); err != nil {
			return nil, errors.WithMessage(err, "error scanning foreign key constraint")
		}
		ret = append(ret, fk)
	}
	if rows.Err() != nil {
		return nil, errors.WithMessage(rows.Err(), "error reading foreign keys")
	}

	return ret, nil
}
