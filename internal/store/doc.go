// Package store is the SQLite implementation of the entity store interfaces.
// It owns schema bootstrap and migrations, row scanning, and transaction
// handling; database/sql types never cross its boundary. Stores assume the
// services validated their inputs and only check what they need to form a
// valid statement.
package store
