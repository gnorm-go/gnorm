package postgres // import "gnorm.org/gnorm/database/drivers/postgres"

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	// register postgres driver
	_ "github.com/lib/pq"
	"github.com/pkg/errors"

	"gnorm.org/gnorm/database"
	"gnorm.org/gnorm/database/drivers/postgres/gnorm/columns"
	"gnorm.org/gnorm/database/drivers/postgres/gnorm/tables"
)

// PG implements drivers.Driver interface for interacting with postgresql
// database.
type PG struct{}

// Parse reads the postgres schemas for the given schemas and converts them into
// database.Info structs.
func (PG) Parse(log *log.Logger, conn string, schemaNames []string, filterTables func(schema, table string) bool) (*database.Info, error) {
	return parse(log, conn, schemaNames, filterTables)
}

func parse(log *log.Logger, conn string, schemaNames []string, filterTables func(schema, table string) bool) (*database.Info, error) {
	log.Println("connecting to postgres with DSN", conn)
	db, err := sql.Open("postgres", conn)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	sch := make([]sql.NullString, len(schemaNames))
	for x := range schemaNames {
		sch[x] = sql.NullString{String: schemaNames[x], Valid: true}
	}

	log.Println("querying table schemas for", schemaNames)
	tables, err := tables.Query(db, tables.TableSchemaCol.In(sch))
	if err != nil {
		return nil, err
	}

	log.Printf("found %v tables", len(tables))
	schemas := make(map[string][]*database.Table, len(schemaNames))
	for _, t := range tables {
		if !filterTables(t.TableSchema.String, t.TableName.String) {
			log.Printf("skipping filtered-out table %v.%v", t.TableSchema.String, t.TableName.String)
			continue
		}

		schemas[t.TableSchema.String] = append(schemas[t.TableSchema.String], &database.Table{
			Name:         t.TableName.String,
			Type:         t.TableType.String,
			IsView:       t.TableType.String == "VIEW",
			IsInsertable: t.IsInsertableInto.String == "YES",
		})
	}

	columns, err := columns.Query(db, columns.TableSchemaCol.In(sch))
	if err != nil {
		return nil, err
	}
	log.Printf("found %v columns for all tables in all specified schemas", len(columns))
	for _, c := range columns {
		if !filterTables(c.TableSchema.String, c.TableName.String) {
			log.Printf("skipping column %q because it is for filtered-out table %v.%v", c.ColumnName.String, c.TableSchema.String, c.TableName.String)
			continue
		}

		tables, ok := schemas[c.TableSchema.String]
		if !ok {
			log.Printf("Should be impossible: column %q references unknown schema %q", c.ColumnName.String, c.TableSchema.String)
			continue
		}
		var table *database.Table
		for _, t := range tables {
			if t.Name == c.TableName.String {
				table = t
				break
			}
		}
		if table == nil {
			log.Printf("Should be impossible: column %q references unknown table %q in schema %q", c.ColumnName.String, c.TableName.String, c.TableSchema.String)
			continue
		}

		col := toDBColumn(c, log)
		table.Columns = append(table.Columns, col)
	}

	primaryKeys, err := queryPrimaryKeys(log, db, schemaNames)
	if err != nil {
		return nil, err
	}
	log.Printf("found %v primary keys", len(primaryKeys))
	for _, pk := range primaryKeys {
		if !filterTables(pk.SchemaName, pk.TableName) {
			log.Printf("skipping constraint %q because it is for filtered-out table %v.%v", pk.Name, pk.SchemaName, pk.TableName)
			continue
		}

		tables, ok := schemas[pk.SchemaName]
		if !ok {
			log.Printf("Should be impossible: constraint %q references unknown schema %q", pk.Name, pk.SchemaName)
			continue
		}
		var table *database.Table
		for _, t := range tables {
			if t.Name == pk.TableName {
				table = t
				break
			}
		}
		if table == nil {
			log.Printf("Should be impossible: constraint %q references unknown table %q in schema %q", pk.Name, pk.TableName, pk.SchemaName)
			continue
		}

		for _, col := range table.Columns {
			if pk.ColumnName != col.Name {
				continue
			}
			col.IsPrimaryKey = true
		}
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

	enums, err := queryEnums(log, db, schemaNames)
	if err != nil {
		return nil, err
	}
	log.Printf("found %v enums for all schemas", len(enums))

	indexResults, err := queryIndexes(log, db, schemaNames)
	if err != nil {
		return nil, err
	}
	log.Printf("found %d indexes for all tables in all schemas", len(indexResults))

	indexes := make(map[string]map[string][]*database.Index)
outer:
	for _, r := range indexResults {
		if !filterTables(r.SchemaName, r.TableName) {
			continue
		}

		tables, ok := schemas[r.SchemaName]
		if !ok {
			log.Printf("Should be impossible: index %q references unknown schema %q", r.IndexName, r.SchemaName)
			continue
		}

		var table *database.Table
		for _, t := range tables {
			if t.Name == r.TableName {
				table = t
				break
			}
		}
		if table == nil {
			log.Printf("Should be impossible: index %q references unknown table %q", r.IndexName, r.TableName)
			continue
		}

		columnMap := make(map[string]*database.Column, len(table.Columns))
		for _, c := range table.Columns {
			columnMap[c.Name] = c
		}

		columns := make([]*database.Column, 0)
		for _, c := range r.Columns {
			column, cok := columnMap[c]
			if !cok {
				log.Printf("Should be impossible: index %q references unknown column %q", r.IndexName, c)
				continue outer
			}
			columns = append(columns, column)
		}

		schemaIndex, ok := indexes[r.SchemaName]
		if !ok {
			schemaIndex = make(map[string][]*database.Index)
			indexes[r.SchemaName] = schemaIndex
		}

		var index *database.Index
		for _, i := range schemaIndex[r.TableName] {
			if i.Name == r.IndexName {
				index = i
				break
			}
		}
		if index == nil {
			index = &database.Index{Name: r.IndexName, IsUnique: r.IsUnique}
			schemaIndex[r.TableName] = append(schemaIndex[r.TableName], index)
		}

		index.Columns = columns
	}

	columnCommentResults, err := queryColumnComments(log, db, schemaNames)
	if err != nil {
		return nil, err
	}
	log.Printf("found %d comments for all columns in all tables in all specified schemas", len(columnCommentResults))

	for _, r := range columnCommentResults {
		if !filterTables(r.SchemaName, r.TableName) {
			continue
		}

		tables, ok := schemas[r.SchemaName]
		if !ok {
			log.Printf("Should be impossible: comment for %q.%q.%q references unknown schema %q",
				r.SchemaName, r.TableName, r.ColumnName, r.SchemaName)
			continue
		}

		var table *database.Table
		for _, t := range tables {
			if t.Name == r.TableName {
				table = t
				break
			}
		}
		if table == nil {
			log.Printf("Should be impossible: comment for %q.%q.%q references unknown table %q",
				r.SchemaName, r.TableName, r.ColumnName, r.TableName)
			continue
		}

		for _, c := range table.Columns {
			if c.Name == r.ColumnName {
				c.Comment = r.Comment
				break
			}
		}
	}

	tableCommentResults, err := queryTableComments(log, db, schemaNames)
	if err != nil {
		return nil, err
	}
	log.Printf("found %d comments for all tables in all specified schemas", len(tableCommentResults))

	for _, r := range tableCommentResults {
		if !filterTables(r.SchemaName, r.TableName) {
			continue
		}

		tables, ok := schemas[r.SchemaName]
		if !ok {
			log.Printf("Should be impossible: comment for %q.%q references unknown schema %q",
				r.SchemaName, r.TableName, r.SchemaName)
			continue
		}

		var table *database.Table
		for _, t := range tables {
			if t.Name == r.TableName {
				table = t
				break
			}
		}
		if table == nil {
			log.Printf("Should be impossible: comment for %q.%q references unknown table %s",
				r.SchemaName, r.TableName, r.TableName)
			continue
		}

		table.Comment = r.Comment
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

func toDBColumn(c *columns.Row, log *log.Logger) *database.Column {
	col := &database.Column{
		Name:       c.ColumnName.String,
		Nullable:   c.IsNullable.String == "YES",
		HasDefault: c.ColumnDefault.String != "",
		Length:     int(c.CharacterMaximumLength.Int64),
		Ordinal:    c.OrdinalPosition.Int64,
		Orig:       *c,
	}

	typ := c.DataType.String
	switch typ {
	case "ARRAY":
		col.IsArray = true
		// when it's an array, postges prepends an underscore to the standard
		// name.
		typ = c.UdtName.String[1:]

	case "USER-DEFINED":
		col.UserDefined = true
		typ = c.UdtName.String
	}

	col.Type = typ

	return col
}

func queryPrimaryKeys(log *log.Logger, db *sql.DB, schemas []string) ([]*database.PrimaryKey, error) {
	// TODO: make this work with Gnorm generated types
	const q = `
	SELECT k.table_schema, k.table_name, k.column_name, k.constraint_name
	FROM information_schema.key_column_usage k
	LEFT JOIN information_schema.table_constraints c
    	ON k.table_schema = c.table_schema
    	AND k.table_name = c.table_name
    	AND k.constraint_name = c.constraint_name
	WHERE c.constraint_type='PRIMARY KEY' AND k.table_schema IN (%s)`
	spots := make([]string, len(schemas))
	vals := make([]interface{}, len(schemas))
	for x := range schemas {
		spots[x] = fmt.Sprintf("$%v", x+1)
		vals[x] = schemas[x]
	}
	query := fmt.Sprintf(q, strings.Join(spots, ", "))
	rows, err := db.Query(query, vals...)
	if err != nil {
		return nil, errors.WithMessage(err, "error querying keys")
	}
	defer rows.Close()
	var ret []*database.PrimaryKey

	for rows.Next() {
		kc := &database.PrimaryKey{}
		if err := rows.Scan(&kc.SchemaName, &kc.TableName, &kc.ColumnName, &kc.Name); err != nil {
			return nil, errors.WithMessage(err, "error scanning key constraint")
		}
		ret = append(ret, kc)
	}
	return ret, nil
}

func queryForeignKeys(log *log.Logger, db *sql.DB, schemas []string) ([]*database.ForeignKey, error) {
	// TODO: make this work with Gnorm generated types
	const q = `SELECT rc.constraint_schema, lkc.table_name, lkc.column_name, lkc.constraint_name, lkc.position_in_unique_constraint, fkc.table_name, fkc.column_name
	  FROM information_schema.referential_constraints rc
  		LEFT JOIN information_schema.key_column_usage lkc
    	  ON lkc.table_schema = rc.constraint_schema
      		AND lkc.constraint_name = rc.constraint_name
  		LEFT JOIN information_schema.key_column_usage fkc
    	  ON fkc.table_schema = rc.constraint_schema
      	    AND fkc.ordinal_position = lkc.position_in_unique_constraint
      		AND fkc.constraint_name = rc.unique_constraint_name
	  WHERE rc.constraint_schema IN (%s)`
	spots := make([]string, len(schemas))
	vals := make([]interface{}, len(schemas))
	for x := range schemas {
		spots[x] = fmt.Sprintf("$%v", x+1)
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

type indexResult struct {
	SchemaName string
	TableName  string
	IndexName  string
	IsUnique   bool
	Columns    []string
}

func queryIndexes(log *log.Logger, db *sql.DB, schemaNames []string) ([]indexResult, error) {
	const q = `
	SELECT
		n.nspname as schema,
		trim(both '"' from i.indrelid::regclass::text) as table,
		c.relname as name,
		i.indisunique as is_unique,
		array_to_string(ARRAY(
			SELECT pg_get_indexdef(i.indexrelid, k + 1, true)
			FROM generate_subscripts(i.indkey, 1) as k
			ORDER BY k
		), ',') as column_names
	FROM pg_index as i
	JOIN pg_class as c
		ON c.oid = i.indexrelid
	JOIN pg_namespace as n
		ON n.oid = c.relnamespace
	WHERE n.nspname IN (%s)`

	spots := make([]string, len(schemaNames))
	vals := make([]interface{}, len(schemaNames))
	for i := range schemaNames {
		spots[i] = fmt.Sprintf("$%v", i+1)
		vals[i] = schemaNames[i]
	}

	query := fmt.Sprintf(q, strings.Join(spots, ", "))
	rows, err := db.Query(query, vals...)
	if err != nil {
		return nil, errors.WithMessage(err, "error querying indexes")
	}
	defer rows.Close()

	var results []indexResult
	for rows.Next() {
		var r indexResult
		var cs string
		if err := rows.Scan(&r.SchemaName, &r.TableName, &r.IndexName, &r.IsUnique, &cs); err != nil {
			return nil, errors.WithMessage(err, "error scanning index")
		}
		r.Columns = strings.Split(cs, ",") // array converted to string in query

		// postgres prepends schema onto table name if outside of public schema
		if r.SchemaName != "public" {
			r.TableName = r.TableName[len(r.SchemaName)+1:]
		}

		results = append(results, r)
	}

	return results, nil
}

type columnCommentResult struct {
	SchemaName string
	TableName  string
	ColumnName string
	Comment    string
}

func queryColumnComments(log *log.Logger, db *sql.DB, schemaNames []string) ([]columnCommentResult, error) {
	const q = `
	SELECT
		cols.table_schema,
		cols.table_name,
		cols.column_name,
			(
					SELECT pg_catalog.col_description(c.oid, cols.ordinal_position::int)
					FROM pg_catalog.pg_class c
					WHERE c.relname = cols.table_name
			) AS column_comment
	FROM information_schema.columns cols
	WHERE cols.table_schema IN (%s)`

	spots := make([]string, len(schemaNames))
	vals := make([]interface{}, len(schemaNames))
	for i := range schemaNames {
		spots[i] = fmt.Sprintf("$%v", i+1)
		vals[i] = schemaNames[i]
	}

	query := fmt.Sprintf(q, strings.Join(spots, ", "))
	rows, err := db.Query(query, vals...)
	if err != nil {
		return nil, errors.WithMessage(err, "error querying column comments")
	}
	defer rows.Close()

	var results []columnCommentResult
	for rows.Next() {
		var r columnCommentResult
		var c sql.NullString
		if err := rows.Scan(&r.SchemaName, &r.TableName, &r.ColumnName, &c); err != nil {
			return nil, errors.WithMessage(err, "error scanning column comment")
		}

		if c.Valid {
			r.Comment = c.String
			results = append(results, r)
		}
	}

	return results, nil
}

type tableCommentResult struct {
	SchemaName string
	TableName  string
	Comment    string
}

func queryTableComments(log *log.Logger, db *sql.DB, schemaNames []string) ([]tableCommentResult, error) {
	const q = `
	SELECT
		tabs.table_schema,
		tabs.table_name,
			(
					SELECT obj_description(c.oid)
					FROM pg_class c
					WHERE c.relname = tabs.table_name
			) AS column_comment
	FROM information_schema.tables tabs
	WHERE tabs.table_schema IN (%s)`

	spots := make([]string, len(schemaNames))
	vals := make([]interface{}, len(schemaNames))
	for i := range schemaNames {
		spots[i] = fmt.Sprintf("$%v", i+1)
		vals[i] = schemaNames[i]
	}

	query := fmt.Sprintf(q, strings.Join(spots, ", "))
	rows, err := db.Query(query, vals...)
	if err != nil {
		return nil, errors.WithMessage(err, "error querying table comments")
	}
	defer rows.Close()

	var results []tableCommentResult
	for rows.Next() {
		var r tableCommentResult
		var c sql.NullString
		if err := rows.Scan(&r.SchemaName, &r.TableName, &c); err != nil {
			return nil, errors.WithMessage(err, "error scanning table comment")
		}

		if c.Valid {
			r.Comment = c.String
			results = append(results, r)
		}
	}

	return results, nil
}

func queryEnums(log *log.Logger, db *sql.DB, schemas []string) (map[string][]*database.Enum, error) {
	// TODO: make this work with Gnorm generated types
	const q = `
	SELECT      n.nspname, t.typname as type
	FROM        pg_type t
	LEFT JOIN   pg_catalog.pg_namespace n ON n.oid = t.typnamespace
	WHERE       (t.typrelid = 0 OR (SELECT c.relkind = 'c' FROM pg_catalog.pg_class c WHERE c.oid = t.typrelid))
	AND     NOT EXISTS(SELECT 1 FROM pg_catalog.pg_type el WHERE el.oid = t.typelem AND el.typarray = t.oid)
	AND     n.nspname IN (%s)`
	spots := make([]string, len(schemas))
	vals := make([]interface{}, len(schemas))
	for x := range schemas {
		spots[x] = fmt.Sprintf("$%v", x+1)
		vals[x] = schemas[x]
	}
	query := fmt.Sprintf(q, strings.Join(spots, ", "))
	rows, err := db.Query(query, vals...)
	if err != nil {
		return nil, errors.WithMessage(err, "error querying enum names")
	}
	defer rows.Close()
	ret := map[string][]*database.Enum{}
	for rows.Next() {
		var name, schema string
		if err := rows.Scan(&schema, &name); err != nil {
			return nil, errors.WithMessage(err, "error scanning enum name into string")
		}
		vals, err := queryValues(log, db, schema, name)
		if err != nil {
			return nil, err
		}
		enum := &database.Enum{
			Name:   name,
			Values: vals,
		}
		ret[schema] = append(ret[schema], enum)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.WithMessage(err, "error reading enum names")
	}
	return ret, nil
}

func queryValues(log *log.Logger, db *sql.DB, schema, enum string) ([]*database.EnumValue, error) {
	// TODO: make this work with Gnorm generated types
	rows, err := db.Query(`
	SELECT
	e.enumlabel,
	e.enumsortorder
	FROM pg_type t
	JOIN ONLY pg_namespace n ON n.oid = t.typnamespace
	LEFT JOIN pg_enum e ON t.oid = e.enumtypid
	WHERE n.nspname = $1 AND t.typname = $2`, schema, enum)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to query enum values for %s.%s", schema, enum)
	}
	defer rows.Close()
	var vals []*database.EnumValue
	for rows.Next() {
		var name sql.NullString
		var val sql.NullInt64
		if err := rows.Scan(&name, &val); err != nil {
			return nil, errors.Wrapf(err, "failed reading enum values for %s.%s", schema, enum)
		}
		vals = append(vals, &database.EnumValue{Name: name.String, Value: int(val.Int64)})
	}
	log.Printf("found %d values for enum %v.%v", len(vals), schema, enum)
	return vals, nil
}
