# PostgreSQL Schema Dump Sanitiser

This go library serves to produce a more human readable schema dump by cleaning up the output from PostgreSQL's `pg_dump -s` command.

The library was last updated with PostgreSQL 10.1.

## Setup

```
go get github.com/jchiam/psql-schema-dump-sanitiser
```

## Usage

Run from your `$GOPATH`:
```
psql-schema-dump-sanitiser <input path> > <output path>
```

## Outstanding Issues

- ~~Produced output does not print tables in referential order [#1](https://github.com/jchiam/psql-schema-dump-sanitiser/issues/1)~~
- Add tests [#2](https://github.com/jchiam/psql-schema-dump-sanitiser/issues/2)

## Specifications

The processing mechanism is as follows.

1. Redunant lines such as comments, `SET` , `EXTENSIONS` and `OWNER` statements are removed
1. `CREATE TABLE` statements are parsed into table maps containing column information
1. Any multi-line statements are squashed into single line statements
1. Sequences are parsed and process through the following
   1. Modifiers with default values are removed
   1. `ALTER SEQUENCE` statements for table ownership are squashed into the respective `CREATE SEQUENCE` statements
1. Default values are added to the table columns
1. Constraint statements are mapped to tables and columns are marked as primary key or foreign key
1. Indices statements are mapped to tables
1. If there are anymore unprocessed lines, fatal error occurs
1. Print output (tables are printed in topological order to ensure referential integrity when dumping into database)
