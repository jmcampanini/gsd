// Package cli defines the Cobra command tree. Commands adapt arguments,
// flags, streams, and presentation; validation and orchestration live in the
// entity services, which commands reach through an injected factory so help,
// version, and argument parsing never open the database.
package cli
